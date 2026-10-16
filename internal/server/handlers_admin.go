package server

import (
	"strings"

	"easysub/internal/store"
)

func (s *Server) listUsers(c *Ctx) (any, error) {
	var rows []store.User
	if err := c.DB.Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]userDTO, 0, len(rows))
	for i := range rows {
		out = append(out, publicUser(&rows[i]))
	}
	return out, nil
}
func (s *Server) createUser(c *Ctx) (any, error) {
	var in struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		IsAdmin  bool   `json:"is_admin"`
		IsActive *bool  `json:"is_active"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	in.Username = cleanUsername(in.Username)
	in.Email = cleanEmail(in.Email)
	if err := validateAccount(in.Username, in.Email, in.Password, true); err != nil {
		return nil, err
	}
	var n int64
	c.DB.Model(&store.User{}).Where("username=? OR email=?", in.Username, in.Email).Count(&n)
	if n > 0 {
		return nil, Err(409, "username or email is already in use")
	}
	hash, err := store.HashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	u := store.User{Username: in.Username, Email: in.Email, PasswordHash: hash, IsAdmin: in.IsAdmin, IsActive: boolValue(in.IsActive, true), EmailVerified: true, IsApproved: true, Locale: "zh", Theme: "light", BaseCurrency: "CNY"}
	if err := c.DB.Create(&u).Error; err != nil {
		return nil, err
	}
	store.EnsureUserDefaults(c.DB, &u)
	return publicUser(&u), nil
}
func (s *Server) updateUser(c *Ctx) (any, error) {
	id, err := c.PathInt("id")
	if err != nil {
		return nil, err
	}
	var u store.User
	if err := c.DB.First(&u, id).Error; err != nil {
		return nil, Err(404, "user not found")
	}
	raw, err := c.BindFields()
	if err != nil {
		return nil, err
	}
	var active, admin *bool
	var locale, theme, base string
	if err := Apply(raw, map[string]any{"is_active": &active, "is_admin": &admin, "locale": &locale, "theme": &theme, "base_currency": &base}); err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if _, ok := raw["is_active"]; ok {
		updates["is_active"] = active
		u.IsActive = boolValue(active, u.IsActive)
	}
	if _, ok := raw["is_admin"]; ok {
		updates["is_admin"] = admin
		u.IsAdmin = boolValue(admin, u.IsAdmin)
	}
	if _, ok := raw["locale"]; ok {
		updates["locale"] = strings.TrimSpace(locale)
		u.Locale = locale
	}
	if _, ok := raw["theme"]; ok {
		updates["theme"] = strings.TrimSpace(theme)
		u.Theme = theme
	}
	if _, ok := raw["base_currency"]; ok {
		updates["base_currency"] = strings.ToUpper(strings.TrimSpace(base))
		u.BaseCurrency = updates["base_currency"].(string)
	}
	if len(updates) > 0 {
		if err := c.DB.Model(&u).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return publicUser(&u), nil
}
func (s *Server) deleteUser(c *Ctx) (any, error) {
	id, err := c.PathInt("id")
	if err != nil {
		return nil, err
	}
	if id == c.User.ID {
		return nil, Err(422, "cannot delete your own account")
	}
	res := c.DB.Delete(&store.User{}, id)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, Err(404, "user not found")
	}
	return map[string]any{"status": "ok"}, nil
}
