package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"easysub/internal/backup"
	"easysub/internal/config"
	"easysub/internal/icons"
	"easysub/internal/notify"
	"easysub/internal/scheduler"
	"easysub/internal/store"
)

func (s *Server) iconManifest(*Ctx) (any, error) { return icons.Manifest(), nil }
func (s *Server) iconFromURL(c *Ctx) (any, error) {
	var in struct {
		URL string `json:"url"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	path, err := icons.FromURL(c.Context(), c.User.ID, in.URL)
	if err != nil {
		return nil, Err(422, "could not download icon: %v", err)
	}
	return map[string]any{"path": path}, nil
}
func (s *Server) iconUpload(c *Ctx) (any, error) {
	if err := c.R.ParseMultipartForm(2 << 20); err != nil {
		return nil, Err(422, "invalid icon upload")
	}
	file, header, err := c.R.FormFile("file")
	if err != nil {
		return nil, Err(422, "an icon file is required")
	}
	defer file.Close()
	path, err := icons.SaveUpload(c.User.ID, header.Filename, file)
	if err != nil {
		return nil, Err(422, "could not save icon: %v", err)
	}
	return map[string]any{"path": path}, nil
}

func (s *Server) exportBackup(c *Ctx) (any, error) { return backup.ExportUser(c.DB, c.User), nil }
func backupData(c *Ctx) (map[string]any, bool, error) {
	var in struct {
		Data    json.RawMessage `json:"data"`
		Replace bool            `json:"replace"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, false, err
	}
	var data map[string]any
	if len(in.Data) == 0 || json.Unmarshal(in.Data, &data) != nil {
		return nil, false, Err(422, "backup data must be a JSON object")
	}
	return data, in.Replace, nil
}
func (s *Server) importBackup(c *Ctx) (any, error) {
	data, replace, err := backupData(c)
	if err != nil {
		return nil, err
	}
	n, err := backup.ImportUser(c.DB, c.User, data, replace)
	if err != nil {
		return nil, Err(422, "could not import backup: %v", err)
	}
	return map[string]any{"imported": n}, nil
}
func (s *Server) exportAllBackup(c *Ctx) (any, error) { return backup.ExportAll(c.DB), nil }
func (s *Server) importAllBackup(c *Ctx) (any, error) {
	data, replace, err := backupData(c)
	if err != nil {
		return nil, err
	}
	result, err := backup.ImportAll(c.DB, data, replace)
	if err != nil {
		return nil, Err(422, "could not import backup: %v", err)
	}
	return result, nil
}
func (s *Server) listLocalBackups(*Ctx) (any, error) { return backup.ListLocal(), nil }
func (s *Server) runLocalBackup(c *Ctx) (any, error) {
	out, err := backup.RunLocalBackup(c.DB)
	if err != nil {
		return nil, Err(500, "could not create local backup: %v", err)
	}
	return out, nil
}
func (s *Server) webdavConfig(c *Ctx) (any, error) {
	cfg, status := backup.LoadWebDAV(c.User)
	return map[string]any{"config": cfg, "status": status}, nil
}
func webdavRequest(c *Ctx) (backup.WebDAVConfig, error) {
	var in backup.WebDAVConfig
	if err := c.Bind(&in); err != nil {
		return in, err
	}
	return in, nil
}
func (s *Server) saveWebDAVConfig(c *Ctx) (any, error) {
	in, err := webdavRequest(c)
	if err != nil {
		return nil, err
	}
	out, err := backup.SaveWebDAV(c.DB, c.User, in)
	if err != nil {
		return nil, Err(422, "could not save WebDAV configuration: %v", err)
	}
	return out, nil
}
func (s *Server) testWebDAV(c *Ctx) (any, error) {
	in, err := webdavRequest(c)
	if err != nil {
		return nil, err
	}
	if in.Password == "" {
		old, _ := backup.LoadWebDAV(c.User)
		in.Password = old.Password
	}
	message, err := backup.TestWebDAV(in)
	if err != nil {
		return nil, Err(422, "WebDAV test failed: %v", err)
	}
	return map[string]any{"message": message}, nil
}
func (s *Server) listWebDAVFiles(c *Ctx) (any, error) {
	cfg, _ := backup.LoadWebDAV(c.User)
	rows, err := backup.ListRemote(cfg)
	if err != nil {
		return nil, Err(422, "could not list WebDAV files: %v", err)
	}
	return map[string]any{"files": rows}, nil
}
func (s *Server) runWebDAVBackup(c *Ctx) (any, error) {
	row, err := backup.RunWebDAVBackup(c.DB, c.User)
	if err != nil {
		return nil, Err(422, "WebDAV backup failed: %v", err)
	}
	return row, nil
}
func (s *Server) restoreWebDAVBackup(c *Ctx) (any, error) {
	var in struct {
		Name    string `json:"name"`
		Replace bool   `json:"replace"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	cfg, _ := backup.LoadWebDAV(c.User)
	data, err := backup.ReadRemote(cfg, in.Name)
	if err != nil {
		return nil, Err(422, "could not read WebDAV backup: %v", err)
	}
	n, err := backup.ImportUser(c.DB, c.User, data, in.Replace)
	if err != nil {
		return nil, Err(422, "could not restore backup: %v", err)
	}
	return map[string]any{"imported": n}, nil
}
func (s *Server) deleteWebDAVFile(c *Ctx) (any, error) {
	cfg, _ := backup.LoadWebDAV(c.User)
	if err := backup.DeleteRemote(cfg, c.R.PathValue("name")); err != nil {
		return nil, Err(422, "could not delete WebDAV backup: %v", err)
	}
	return map[string]any{"status": "ok"}, nil
}

func (s *Server) notificationConfig(c *Ctx) (any, error) { return notify.LoadConfig(c.User), nil }
func (s *Server) saveNotificationConfig(c *Ctx) (any, error) {
	var in struct {
		Config map[string]any `json:"config"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	out := notify.ApplyConfig(c.User, in.Config)
	if err := c.DB.Model(c.User).Updates(map[string]any{"notify_config": c.User.NotifyConfig, "telegram_enabled": c.User.TelegramEnabled, "telegram_bot_token": c.User.TelegramBotToken, "telegram_chat_id": c.User.TelegramChatID, "telegram_admin_id": c.User.TelegramAdminID, "telegram_api_base": c.User.TelegramAPIBase, "telegram_proxy": c.User.TelegramProxy}).Error; err != nil {
		return nil, err
	}
	return out, nil
}
func (s *Server) testNotification(c *Ctx) (any, error) {
	var in struct {
		Channel string         `json:"channel"`
		Config  map[string]any `json:"config"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	if in.Config == nil {
		in.Config = notify.LoadConfig(c.User)[in.Channel]
	}
	if in.Config == nil {
		return nil, Err(422, "unknown notification channel")
	}
	if err := notify.SendOne(c.Context(), in.Channel, in.Config, "EasySub test", "This is a test notification from EasySub."); err != nil {
		return nil, Err(422, "notification failed: %v", err)
	}
	return map[string]any{"status": "ok"}, nil
}
func (s *Server) checkWeCom(c *Ctx) (any, error) {
	var in struct {
		Config map[string]any `json:"config"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	out, err := notify.WecomAgentInfo(c.Context(), in.Config)
	if err != nil {
		return nil, Err(422, "WeCom check failed: %v", err)
	}
	return out, nil
}
func (s *Server) telegramMe(c *Ctx) (any, error) {
	cfg := notify.LoadConfig(c.User)["telegram"]
	out, err := notify.TelegramGetMe(c.Context(), stringValue(cfg["bot_token"]), stringValue(cfg["api_base"]), stringValue(cfg["proxy"]))
	if err != nil {
		return nil, Err(422, "Telegram request failed: %v", err)
	}
	return out, nil
}
func (s *Server) telegramUpdates(c *Ctx) (any, error) {
	cfg := notify.LoadConfig(c.User)["telegram"]
	out, err := notify.TelegramGetUpdates(c.Context(), stringValue(cfg["bot_token"]), stringValue(cfg["api_base"]), stringValue(cfg["proxy"]))
	if err != nil {
		return nil, Err(422, "Telegram request failed: %v", err)
	}
	return out, nil
}
func (s *Server) notificationLogs(c *Ctx) (any, error) {
	var rows []store.NotificationLog
	q := c.DB.Where("user_id=?", c.User.ID)
	if sub := c.QueryInt("subscription_id", 0); sub > 0 {
		q = q.Where("subscription_id=?", sub)
	}
	if err := q.Order("sent_at desc").Limit(200).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
func (s *Server) runNotificationScan(c *Ctx) (any, error) {
	return scheduler.RunReminderScan(c.Context()), nil
}
func stringValue(v any) string {
	if x, ok := v.(string); ok {
		return x
	}
	return ""
}

func (s *Server) calendarToken(c *Ctx) (any, error) {
	reset, _ := c.QueryBool("reset")
	if c.User.CalendarToken == nil || reset {
		buf := make([]byte, 24)
		_, _ = rand.Read(buf)
		token := hex.EncodeToString(buf)
		if err := c.DB.Model(c.User).Update("calendar_token", token).Error; err != nil {
			return nil, err
		}
		c.User.CalendarToken = &token
	}
	return map[string]any{"token": *c.User.CalendarToken, "url": "/api/calendar/" + *c.User.CalendarToken + "/easysub.ics"}, nil
}
func (s *Server) calendarICS(c *Ctx) (any, error) {
	token := c.R.PathValue("token")
	var u store.User
	if err := c.DB.Where("calendar_token=?", token).First(&u).Error; err != nil {
		return nil, Err(404, "calendar not found")
	}
	var rows []store.Subscription
	c.DB.Where("user_id=? AND is_active=? AND show_in_calendar=? AND next_renewal_date IS NOT NULL", u.ID, true, true).Find(&rows)
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//EasySub//EN\r\nCALSCALE:GREGORIAN\r\n")
	for _, x := range rows {
		d := x.NextRenewalDate.String()
		fmt.Fprintf(&b, "BEGIN:VEVENT\r\nUID:easysub-%d-%d\r\nDTSTART;VALUE=DATE:%s\r\nSUMMARY:%s renewal\r\nDESCRIPTION:%.2f %s\r\nEND:VEVENT\r\n", u.ID, x.ID, strings.ReplaceAll(d, "-", ""), icsEscape(x.Name), x.Amount, x.Currency)
	}
	b.WriteString("END:VCALENDAR\r\n")
	c.W.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	_, _ = c.W.Write([]byte(b.String()))
	return Raw{}, nil
}
func icsEscape(v string) string {
	return strings.NewReplacer("\\", "\\\\", ",", "\\,", ";", "\\;", "\n", "\\n").Replace(v)
}
func (s *Server) logs(c *Ctx) (any, error) {
	limit := c.QueryInt("limit", 100)
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	q := c.DB.Where("user_id=?", c.User.ID)
	if after := c.QueryInt("after", 0); after > 0 {
		q = q.Where("id>?", after)
	}
	var rows []store.ActivityLog
	if err := q.Order("id desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
func (s *Server) systemInfo(c *Ctx) (any, error) {
	return map[string]any{"version": Version, "database": "SQLite", "mcp_enabled": config.C.MCPEnabled, "api_docs": "/api/docs", "go_version": runtime.Version(), "time": time.Now()}, nil
}
func (s *Server) versionCheck(*Ctx) (any, error) {
	return map[string]any{"current": Version, "latest": Version, "update_available": false}, nil
}
func (s *Server) apiDocs(c *Ctx) (any, error) {
	c.W.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(c.W, "<!doctype html><title>EasySub API</title><h1>EasySub API</h1><p>Authenticate requests with <code>Authorization: Bearer &lt;API Token&gt;</code>.</p><p>Core resources: /api/subscriptions, /api/currencies, /api/payment-methods, /api/backup and /mcp.</p>")
	return Raw{}, nil
}

var _ = context.Background
var _ = netHTTPPlaceholder
var netHTTPPlaceholder = http.MethodGet
