package server

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"gorm.io/gorm"

	"easysub/internal/config"
	"easysub/internal/secure"
	"easysub/internal/store"
)

type userDTO struct {
	ID             int            `json:"id"`
	Username       string         `json:"username"`
	Email          string         `json:"email"`
	IsAdmin        bool           `json:"is_admin"`
	IsActive       bool           `json:"is_active"`
	EmailVerified  bool           `json:"email_verified"`
	IsApproved     bool           `json:"is_approved"`
	Locale         string         `json:"locale"`
	Theme          string         `json:"theme"`
	BaseCurrency   string         `json:"base_currency"`
	MonthlyBudget  *float64       `json:"monthly_budget"`
	NotifySettings map[string]any `json:"notify_settings"`
	TotpEnabled    bool           `json:"totp_enabled"`
	CreatedAt      time.Time      `json:"created_at"`
}

func publicUser(u *store.User) userDTO {
	settings := map[string]any{}
	if u.NotifySettings.Valid && u.NotifySettings.V != nil {
		settings = u.NotifySettings.V
	}
	return userDTO{
		ID: u.ID, Username: u.Username, Email: u.Email, IsAdmin: u.IsAdmin, IsActive: u.IsActive,
		EmailVerified: u.EmailVerified, IsApproved: u.IsApproved, Locale: u.Locale, Theme: u.Theme,
		BaseCurrency: u.BaseCurrency, MonthlyBudget: u.MonthlyBudget, NotifySettings: settings,
		TotpEnabled: u.TotpEnabled, CreatedAt: u.CreatedAt,
	}
}

func cleanUsername(v string) string { return strings.TrimSpace(v) }
func cleanEmail(v string) string    { return strings.ToLower(strings.TrimSpace(v)) }

func validateAccount(username, email, password string, requirePassword bool) error {
	if len([]rune(username)) < 2 || len([]rune(username)) > 64 {
		return Err(422, "username must contain 2 to 64 characters")
	}
	if _, err := mail.ParseAddress(email); err != nil || len(email) > 255 {
		return Err(422, "a valid email address is required")
	}
	if requirePassword && len(password) < 6 {
		return Err(422, "password must contain at least 6 characters")
	}
	return nil
}

func (s *Server) health(*Ctx) (any, error) {
	return map[string]any{"status": "ok", "version": Version}, nil
}

func (s *Server) ready(*Ctx) (any, error) {
	return map[string]any{"status": "ready", "database": "sqlite"}, nil
}

func (s *Server) setupStatus(*Ctx) (any, error) {
	return map[string]any{"configured": store.IsConfigured(), "driver": "sqlite", "path": config.C.SQLitePath}, nil
}

func (s *Server) setupTest(c *Ctx) (any, error) {
	var in store.DBConfig
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	message, err := store.TestConnection(in)
	if err != nil {
		return nil, Err(422, "SQLite test failed: %v", err)
	}
	return map[string]any{"message": message}, nil
}

func (s *Server) setupSave(c *Ctx) (any, error) {
	var in store.DBConfig
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	if err := store.SaveConfig(in); err != nil {
		return nil, Err(500, "could not save SQLite configuration: %v", err)
	}
	if !store.ConnectSaved(true) {
		return nil, Err(500, "could not initialize SQLite database")
	}
	return map[string]any{"status": "ok"}, nil
}

func tokenResponse(u *store.User) map[string]any {
	return map[string]any{"access_token": secure.AccessToken(u.ID), "refresh_token": secure.RefreshToken(u.ID), "token_type": "bearer"}
}

func (s *Server) login(c *Ctx) (any, error) {
	var username, password string
	if strings.Contains(strings.ToLower(c.R.Header.Get("Content-Type")), "application/x-www-form-urlencoded") {
		if err := c.R.ParseForm(); err != nil {
			return nil, Err(422, "invalid login form")
		}
		username, password = c.R.Form.Get("username"), c.R.Form.Get("password")
	} else {
		var in struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.Bind(&in); err != nil {
			return nil, err
		}
		username, password = in.Username, in.Password
	}
	var u store.User
	q := cleanUsername(username)
	if err := c.DB.Where("username = ? OR email = ?", q, cleanEmail(q)).First(&u).Error; err != nil || !store.VerifyPassword(password, u.PasswordHash) {
		return nil, Err(401, "invalid username or password")
	}
	if !u.IsActive {
		return nil, Err(403, "account is disabled")
	}
	if !u.IsApproved {
		return nil, Err(403, "account is awaiting approval")
	}
	store.EnsureUserDefaults(c.DB, &u)
	store.Log("auth.login", "signed in", &u, "info")
	return tokenResponse(&u), nil
}

func (s *Server) register(c *Ctx) (any, error) {
	var in struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	in.Username, in.Email = cleanUsername(in.Username), cleanEmail(in.Email)
	if err := validateAccount(in.Username, in.Email, in.Password, true); err != nil {
		return nil, err
	}
	var count int64
	c.DB.Model(&store.User{}).Where("username = ? OR email = ?", in.Username, in.Email).Count(&count)
	if count > 0 {
		return nil, Err(409, "username or email is already in use")
	}
	hash, err := store.HashPassword(in.Password)
	if err != nil {
		return nil, Err(500, "could not protect password")
	}
	u := store.User{Username: in.Username, Email: in.Email, PasswordHash: hash, IsActive: true, IsApproved: !config.C.RequireAdminApproval, EmailVerified: true, Locale: "zh", Theme: "light", BaseCurrency: "CNY"}
	if err := c.DB.Create(&u).Error; err != nil {
		return nil, Err(500, "could not create account: %v", err)
	}
	store.EnsureUserDefaults(c.DB, &u)
	status := "ok"
	if !u.IsApproved {
		status = "pending"
	}
	return map[string]any{"status": status, "message": "account created"}, nil
}

func (s *Server) refresh(c *Ctx) (any, error) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	uid := secure.DecodeToken(in.RefreshToken, "refresh")
	if uid == 0 {
		return nil, Err(401, "invalid or expired refresh token")
	}
	var u store.User
	if err := c.DB.First(&u, uid).Error; err != nil || !u.IsActive {
		return nil, Err(401, "invalid or expired refresh token")
	}
	return tokenResponse(&u), nil
}

func (s *Server) me(c *Ctx) (any, error) { return publicUser(c.User), nil }

func (s *Server) updateMe(c *Ctx) (any, error) {
	raw, err := c.BindFields()
	if err != nil {
		return nil, err
	}
	var locale, theme, base string
	var budget *float64
	var settings store.JSON[map[string]any]
	if err := Apply(raw, map[string]any{"locale": &locale, "theme": &theme, "base_currency": &base, "monthly_budget": &budget, "notify_settings": &settings}); err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if _, ok := raw["locale"]; ok {
		if locale != "zh" && locale != "en" && locale != "ru" {
			return nil, Err(422, "unsupported locale")
		}
		updates["locale"] = locale
		c.User.Locale = locale
	}
	if _, ok := raw["theme"]; ok {
		if theme != "light" && theme != "dark" && theme != "system" {
			return nil, Err(422, "unsupported theme")
		}
		updates["theme"] = theme
		c.User.Theme = theme
	}
	if _, ok := raw["base_currency"]; ok {
		base = strings.ToUpper(strings.TrimSpace(base))
		var n int64
		c.DB.Model(&store.UserCurrency{}).Where("user_id = ? AND code = ?", c.User.ID, base).Count(&n)
		if n == 0 {
			return nil, Err(422, "base currency must be in your currency list")
		}
		updates["base_currency"] = base
		c.User.BaseCurrency = base
	}
	if _, ok := raw["monthly_budget"]; ok {
		updates["monthly_budget"] = budget
		c.User.MonthlyBudget = budget
	}
	if _, ok := raw["notify_settings"]; ok {
		updates["notify_settings"] = settings
		c.User.NotifySettings = settings
	}
	if len(updates) > 0 {
		if err := c.DB.Model(c.User).Updates(updates).Error; err != nil {
			return nil, Err(500, "could not update preferences: %v", err)
		}
	}
	return publicUser(c.User), nil
}

func (s *Server) updateAccount(c *Ctx) (any, error) {
	var in struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	in.Username, in.Email = cleanUsername(in.Username), cleanEmail(in.Email)
	if err := validateAccount(in.Username, in.Email, "", false); err != nil {
		return nil, err
	}
	var n int64
	c.DB.Model(&store.User{}).Where("id <> ? AND (username = ? OR email = ?)", c.User.ID, in.Username, in.Email).Count(&n)
	if n > 0 {
		return nil, Err(409, "username or email is already in use")
	}
	if err := c.DB.Model(c.User).Updates(map[string]any{"username": in.Username, "email": in.Email}).Error; err != nil {
		return nil, Err(500, "could not update account: %v", err)
	}
	c.User.Username, c.User.Email = in.Username, in.Email
	return publicUser(c.User), nil
}

func (s *Server) changePassword(c *Ctx) (any, error) {
	var in struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	if !store.VerifyPassword(in.OldPassword, c.User.PasswordHash) {
		return nil, Err(422, "current password is incorrect")
	}
	if len(in.NewPassword) < 6 {
		return nil, Err(422, "new password must contain at least 6 characters")
	}
	hash, err := store.HashPassword(in.NewPassword)
	if err != nil {
		return nil, Err(500, "could not protect password")
	}
	if err := c.DB.Model(c.User).Update("password_hash", hash).Error; err != nil {
		return nil, Err(500, "could not update password")
	}
	c.User.PasswordHash = hash
	return map[string]any{"status": "ok"}, nil
}

func (s *Server) verifyEmail(*Ctx) (any, error)    { return map[string]any{"status": "ok"}, nil }
func (s *Server) forgotPassword(*Ctx) (any, error) { return map[string]any{"status": "ok"}, nil }
func (s *Server) resetPassword(*Ctx) (any, error) {
	return nil, Err(501, "password reset email is not configured")
}

func (s *Server) listTokens(c *Ctx) (any, error) {
	var rows []store.ApiToken
	if err := c.DB.Where("user_id = ?", c.User.ID).Order("id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		masked := "…"
		if len(row.Token) >= 8 {
			masked = row.Token[:4] + "…" + row.Token[len(row.Token)-4:]
		}
		out = append(out, map[string]any{"id": row.ID, "name": row.Name, "masked": masked, "created_at": row.CreatedAt, "last_used_at": row.LastUsedAt})
	}
	return out, nil
}

func (s *Server) createToken(c *Ctx) (any, error) {
	var in struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 64 {
		return nil, Err(422, "a token name of up to 64 characters is required")
	}
	row := store.ApiToken{UserID: c.User.ID, Name: in.Name, Token: secure.RandomToken(40)}
	if err := c.DB.Create(&row).Error; err != nil {
		return nil, Err(500, "could not create token: %v", err)
	}
	return map[string]any{"id": row.ID, "name": row.Name, "token": row.Token, "created_at": row.CreatedAt}, nil
}

func (s *Server) deleteToken(c *Ctx) (any, error) {
	id, err := c.PathInt("id")
	if err != nil {
		return nil, err
	}
	tx := c.DB.Where("id = ? AND user_id = ?", id, c.User.ID).Delete(&store.ApiToken{})
	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return nil, Err(404, "token not found")
	}
	return map[string]any{"status": "ok"}, nil
}

// checkPassword uses a constant-time comparison after bcrypt verification has already normalized timing.
func checkPassword(password, hash string) bool {
	return subtle.ConstantTimeEq(int32(boolToInt(store.VerifyPassword(password, hash))), 1) == 1
}
func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

var _ = fmt.Sprintf
var _ = json.Marshal
var _ = gorm.ErrRecordNotFound
