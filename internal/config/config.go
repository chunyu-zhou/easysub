// Package config 读取环境变量（以及可选的 .env 文件）形成运行时配置。
// The embedded SQLite database is stored under DATA_DIR by default; SQLITE_PATH may override its filename.
package config

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Listen  string
	DataDir string

	JWTSecret            string
	AccessTokenTTL       time.Duration
	RefreshTokenTTL      time.Duration
	TZ                   string
	Location             *time.Location
	RequireAdminApproval bool

	TelegramBotToken string

	ExchangeAPIBase string
	ExchangeAPIURL  string
	ExchangeAPIKey  string

	ReminderScanTime string

	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	SMTPTLS      bool

	AdminUsername string
	AdminPassword string
	AdminEmail    string

	// 图标：商家 favicon 来源（{domain} 占位），抓取超时与并发
	IconProvider     string
	IconFetchTimeout time.Duration
	IconWorkers      int

	// SQLitePath optionally overrides the embedded database filename. Relative paths
	// are resolved under DATA_DIR; the default is DATA_DIR/easysub.db.
	SQLitePath string

	MCPEnabled bool
}

var C *Config

// Load 读取 .env（若存在，不覆盖已有环境变量）与环境变量。
func Load() *Config {
	loadDotEnv(".env")
	c := &Config{
		Listen:               env("LISTEN", ":"+env("PORT", "8000")),
		DataDir:              env("DATA_DIR", "data"),
		JWTSecret:            env("JWT_SECRET", ""),
		AccessTokenTTL:       time.Duration(envInt("ACCESS_TOKEN_EXPIRE_MINUTES", 60)) * time.Minute,
		RefreshTokenTTL:      time.Duration(envInt("REFRESH_TOKEN_EXPIRE_DAYS", 14)) * 24 * time.Hour,
		TZ:                   env("TZ", "Asia/Shanghai"),
		RequireAdminApproval: envBool("REQUIRE_ADMIN_APPROVAL", false),
		TelegramBotToken:     env("TELEGRAM_BOT_TOKEN", ""),
		ExchangeAPIBase:      strings.ToUpper(env("EXCHANGE_API_BASE", "USD")),
		ExchangeAPIURL:       env("EXCHANGE_API_URL", "https://open.er-api.com/v6/latest/"),
		ExchangeAPIKey:       env("EXCHANGE_API_KEY", ""),
		ReminderScanTime:     env("REMINDER_SCAN_TIME", "09:00"),
		SMTPHost:             env("SMTP_HOST", ""),
		SMTPPort:             envInt("SMTP_PORT", 587),
		SMTPUser:             env("SMTP_USER", ""),
		SMTPPassword:         env("SMTP_PASSWORD", ""),
		SMTPFrom:             env("SMTP_FROM", ""),
		SMTPTLS:              envBool("SMTP_TLS", true),
		AdminUsername:        env("ADMIN_USERNAME", "admin"),
		AdminPassword:        env("ADMIN_PASSWORD", "admin123"),
		AdminEmail:           env("ADMIN_EMAIL", "admin@example.com"),
		IconProvider:         env("ICON_PROVIDER", "https://www.google.com/s2/favicons?domain={domain}&sz=64"),
		IconFetchTimeout:     time.Duration(envInt("ICON_FETCH_TIMEOUT_SECONDS", 5)) * time.Second,
		IconWorkers:          envInt("ICON_FETCH_WORKERS", 4),
		SQLitePath:           env("SQLITE_PATH", "easysub.db"),
		MCPEnabled:           envBool("MCP_ENABLED", true),
	}
	if c.ExchangeAPIBase == "" {
		c.ExchangeAPIBase = "USD"
	}
	loc, err := time.LoadLocation(c.TZ)
	if err != nil {
		log.Printf("[config] 无法加载时区 %q（%v），使用本地时区", c.TZ, err)
		loc = time.Local
	}
	c.Location = loc
	_ = os.MkdirAll(c.DataDir, 0o755)
	if c.JWTSecret == "" {
		c.JWTSecret = persistentSecret(filepath.Join(c.DataDir, "jwt_secret"))
	}
	C = c
	return c
}

// Path 返回数据目录下的路径。
func (c *Config) Path(parts ...string) string {
	return filepath.Join(append([]string{c.DataDir}, parts...)...)
}

// Now 返回配置时区下的当前时间。
func Now() time.Time { return time.Now().In(C.Location) }

// Today 返回配置时区下今天 00:00。
func Today() time.Time {
	n := Now()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// persistentSecret：未配置 JWT_SECRET 时生成随机密钥并持久化，避免使用弱默认值。
func persistentSecret(path string) string {
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) >= 16 {
		return strings.TrimSpace(string(b))
	}
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	s := hex.EncodeToString(buf)
	if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
		log.Printf("[config] 无法保存随机 JWT 密钥：%v（重启后登录会失效，请设置 JWT_SECRET）", err)
	}
	return s
}

func env(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v, err := strconv.Atoi(env(k, "")); err == nil {
		return v
	}
	return def
}

func envBool(k string, def bool) bool {
	switch strings.ToLower(env(k, "")) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
}
