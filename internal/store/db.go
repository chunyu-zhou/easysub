package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"easysub/internal/config"
)

// DBConfig records the embedded SQLite database filename. It remains a JSON object
// so installations written by older versions can be normalized safely.
type DBConfig struct {
	Path string `json:"path"`
}

func defaultConfig() DBConfig {
	return DBConfig{Path: config.C.SQLitePath}
}

func (c DBConfig) normalized() DBConfig {
	c.Path = strings.TrimSpace(c.Path)
	if c.Path == "" {
		c.Path = config.C.SQLitePath
	}
	if c.Path == "" {
		c.Path = "easysub.db"
	}
	return c
}

var (
	mu       sync.RWMutex
	db       *gorm.DB
	lastTry  time.Time
	tryMu    sync.Mutex
	onReady  []func()
	readyRun bool
)

const reconnectInterval = 15 * time.Second

func configPath() string { return config.C.Path("db_config.json") }

// ConfigExists remains for compatibility with setup/status callers. Embedded SQLite
// is always available because its parent data directory is created during config load.
func ConfigExists() bool { return true }

// LoadConfig reads the stored filename when present. Older MySQL-shaped configuration
// files deliberately fall back to the embedded default instead of attempting a remote
// connection.
func LoadConfig() (DBConfig, error) {
	fallback := defaultConfig().normalized()
	b, err := os.ReadFile(configPath())
	if errors.Is(err, os.ErrNotExist) {
		return fallback, nil
	}
	if err != nil {
		log.Printf("[db] cannot read legacy database configuration: %v; using embedded SQLite", err)
		return fallback, nil
	}
	var raw struct {
		Path   string `json:"path"`
		Driver string `json:"driver"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		log.Printf("[db] cannot parse legacy database configuration: %v; using embedded SQLite", err)
		return fallback, nil
	}
	if raw.Driver != "" && !strings.EqualFold(raw.Driver, "sqlite") {
		log.Printf("[db] legacy database configuration (%s) ignored; using embedded SQLite", raw.Driver)
		return fallback, nil
	}
	return (DBConfig{Path: raw.Path}).normalized(), nil
}

func SaveConfig(c DBConfig) error {
	c = c.normalized()
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(configPath(), b, 0o600)
}

// Open opens the embedded SQLite database without running migrations.
func Open(c DBConfig) (*gorm.DB, error) {
	c = c.normalized()
	p := c.Path
	if !filepath.IsAbs(p) && !strings.HasPrefix(p, "file:") {
		p = config.C.Path(p)
	}
	if !strings.HasPrefix(p, "file:") {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, err
		}
	}
	g, err := gorm.Open(sqlite.Open(p+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(0)"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Warn), TranslateError: true})
	if err != nil {
		return nil, err
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	// SQLite uses a single writer. Keeping one connection avoids avoidable
	// `database is locked` failures while WAL keeps readers responsive.
	sqlDB.SetMaxOpenConns(1)
	return g, nil
}

// TestConnection opens the local SQLite file and reports its engine version.
func TestConnection(c DBConfig) (string, error) {
	g, err := Open(c)
	if err != nil {
		return "", err
	}
	sqlDB, _ := g.DB()
	defer sqlDB.Close()
	var version string
	err = g.Raw("SELECT sqlite_version()").Scan(&version).Error
	return "SQLite " + version, err
}

// Initialize 连接、建表迁移、写入预置数据并设为全局连接。
func Initialize(c DBConfig) error {
	g, err := Open(c)
	if err != nil {
		return err
	}
	if err := Migrate(g); err != nil {
		closeDB(g)
		return fmt.Errorf("建表失败：%w", err)
	}
	if err := SeedAll(g); err != nil {
		closeDB(g)
		return fmt.Errorf("初始化数据失败：%w", err)
	}
	mu.Lock()
	db = g
	mu.Unlock()
	runReady()
	return nil
}

func closeDB(g *gorm.DB) {
	if s, err := g.DB(); err == nil {
		_ = s.Close()
	}
}

// OnReady 注册数据库就绪后执行一次的回调（如启动定时任务）。
func OnReady(fn func()) {
	mu.Lock()
	defer mu.Unlock()
	onReady = append(onReady, fn)
}

func runReady() {
	mu.Lock()
	if readyRun {
		mu.Unlock()
		return
	}
	readyRun = true
	fns := onReady
	mu.Unlock()
	for _, fn := range fns {
		go fn()
	}
}

// DB 返回全局连接；未就绪时尝试用已保存的配置自动重连（节流）。
func DB() *gorm.DB {
	mu.RLock()
	g := db
	mu.RUnlock()
	if g != nil {
		return g
	}
	if ConnectSaved(false) {
		mu.RLock()
		defer mu.RUnlock()
		return db
	}
	return nil
}

func IsConfigured() bool {
	mu.RLock()
	defer mu.RUnlock()
	return db != nil
}

// ConnectSaved 用磁盘上的配置连接（启动重试与运行期自愈共用）。
func ConnectSaved(force bool) bool {
	if IsConfigured() {
		return true
	}
	tryMu.Lock()
	defer tryMu.Unlock()
	if IsConfigured() {
		return true
	}
	if !force && time.Since(lastTry) < reconnectInterval {
		return false
	}
	lastTry = time.Now()
	if !ConfigExists() {
		return false
	}
	c, err := LoadConfig()
	if err != nil {
		log.Printf("[db] 读取数据库配置失败：%v", err)
		return false
	}
	if err := Initialize(c); err != nil {
		log.Printf("[db] 连接失败：%v", err)
		return false
	}
	return true
}

// Migrate 自动建表 / 补列（只增不删，幂等）。
func Migrate(g *gorm.DB) error {
	return g.AutoMigrate(AllModels()...)
}

// SeedAll 写入系统预置分类与首个管理员。货币与付款方式按用户在首次访问时初始化。
func SeedAll(g *gorm.DB) error {
	var n int64
	g.Model(&Category{}).Where("is_system = ?", true).Count(&n)
	if n == 0 {
		for i, c := range DefaultCategories {
			icon, color := c.Icon, c.Color
			if err := g.Create(&Category{Name: c.Name, Icon: &icon, Color: &color, IsSystem: true, Sort: i}).Error; err != nil {
				return err
			}
		}
	}
	cfg := config.C
	if cfg.AdminUsername != "" && cfg.AdminPassword != "" {
		var cnt int64
		g.Model(&User{}).Where("username = ?", cfg.AdminUsername).Count(&cnt)
		if cnt == 0 {
			hash, err := HashPassword(cfg.AdminPassword)
			if err != nil {
				return err
			}
			u := User{Username: cfg.AdminUsername, Email: cfg.AdminEmail, PasswordHash: hash,
				IsAdmin: true, IsActive: true, EmailVerified: true, IsApproved: true,
				Locale: "zh", Theme: "light", BaseCurrency: "CNY"}
			if err := g.Create(&u).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// EnsureUserDefaults 首次访问时为用户复制默认货币与付款方式（并迁移旧版共享数据）。
func EnsureUserDefaults(g *gorm.DB, u *User) {
	if !u.CurrenciesSeeded {
		if err := seedUserCurrencies(g, u); err != nil {
			log.Printf("[seed] 用户 %d 货币初始化失败：%v", u.ID, err)
		}
	}
	if !u.PaymentMethodsSeeded {
		if err := seedUserPaymentMethods(g, u); err != nil {
			log.Printf("[seed] 用户 %d 付款方式初始化失败：%v", u.ID, err)
		}
	}
}

func seedUserCurrencies(g *gorm.DB, u *User) error {
	return g.Transaction(func(tx *gorm.DB) error {
		var existing []UserCurrency
		tx.Where("user_id = ?", u.ID).Find(&existing)
		have := map[string]bool{}
		for _, c := range existing {
			have[c.Code] = true
		}
		sort := len(existing)
		add := func(def CurrencyDef, custom bool, rate *float64) error {
			if def.Code == "" || have[def.Code] {
				return nil
			}
			have[def.Code] = true
			sort++
			return tx.Create(&UserCurrency{UserID: u.ID, Code: def.Code, Name: def.Name, NameZh: def.NameZh,
				Symbol: def.Symbol, Sort: sort, IsCustom: custom, ManualRate: rate}).Error
		}
		if len(existing) == 0 {
			for _, d := range DefaultCurrencies {
				if err := add(d, false, nil); err != nil {
					return err
				}
			}
		}
		// 迁移：旧版该用户的自定义货币
		if tx.Migrator().HasTable("currencies") {
			var legacy []LegacyCurrency
			tx.Where("is_custom = ? AND user_id = ?", true, u.ID).Find(&legacy)
			for _, l := range legacy {
				if err := add(CurrencyDef{Code: strings.ToUpper(l.Code), Name: l.Name, NameZh: l.Name, Symbol: l.Symbol}, true, nil); err != nil {
					return err
				}
			}
		}
		// 订阅已在使用、但清单里没有的货币也补上，避免下拉框缺项
		var used []string
		tx.Model(&Subscription{}).Where("user_id = ?", u.ID).Distinct().Pluck("currency", &used)
		for _, code := range append(used, u.BaseCurrency) {
			code = strings.ToUpper(strings.TrimSpace(code))
			if def, ok := CatalogCurrency(code); ok {
				if err := add(def, false, nil); err != nil {
					return err
				}
			} else if err := add(CurrencyDef{Code: code, Name: code, NameZh: code}, true, nil); err != nil {
				return err
			}
		}
		u.CurrenciesSeeded = true
		return tx.Model(u).Update("currencies_seeded", true).Error
	})
}

func seedUserPaymentMethods(g *gorm.DB, u *User) error {
	return g.Transaction(func(tx *gorm.DB) error {
		var own int64
		tx.Model(&PaymentMethod{}).Where("user_id = ?", u.ID).Count(&own)
		var system []PaymentMethod
		tx.Where("user_id IS NULL").Order("id").Find(&system)
		if own == 0 {
			if len(system) > 0 {
				// 旧版系统预置项：复制为用户私有，并把订阅引用改到副本上
				for i, s := range system {
					cp := PaymentMethod{UserID: &u.ID, Name: s.Name, Icon: s.Icon, Sort: i}
					if err := tx.Create(&cp).Error; err != nil {
						return err
					}
					if err := tx.Model(&Subscription{}).Where("user_id = ? AND payment_method_id = ?", u.ID, s.ID).
						Update("payment_method_id", cp.ID).Error; err != nil {
						return err
					}
				}
			} else {
				for i, d := range DefaultPaymentMethods {
					icon := d.Icon
					if err := tx.Create(&PaymentMethod{UserID: &u.ID, Name: d.Name, Icon: &icon, Sort: i}).Error; err != nil {
						return err
					}
				}
			}
		}
		u.PaymentMethodsSeeded = true
		return tx.Model(u).Update("payment_methods_seeded", true).Error
	})
}

// ErrNotFound 统一判断记录不存在。
func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

// Log 写一条活动日志（失败忽略）。
func Log(action, detail string, u *User, level string) {
	g := DB()
	if g == nil {
		return
	}
	if level == "" {
		level = "info"
	}
	if r := []rune(detail); len(r) > 2000 {
		detail = string(r[:2000])
	}
	row := ActivityLog{Level: level, Action: action, CreatedAt: time.Now()}
	if detail != "" {
		row.Detail = &detail
	}
	if u != nil {
		row.UserID = &u.ID
		name := u.Username
		row.Username = &name
	}
	if err := g.Create(&row).Error; err != nil {
		log.Printf("[activity] 写日志失败：%v", err)
	}
}
