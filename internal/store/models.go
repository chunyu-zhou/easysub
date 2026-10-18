package store

import (
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Table and column names retain the upstream shape where practical so exports and API payloads remain compatible.

type User struct {
	ID               int        `gorm:"primaryKey" json:"id"`
	Username         string     `gorm:"size:64;uniqueIndex" json:"username"`
	Email            string     `gorm:"size:255;uniqueIndex" json:"email"`
	PasswordHash     string     `gorm:"size:255" json:"-"`
	IsAdmin          bool       `gorm:"default:false" json:"is_admin"`
	IsActive         bool       `gorm:"default:true" json:"is_active"`
	EmailVerified    bool       `gorm:"default:true" json:"email_verified"`
	IsApproved       bool       `gorm:"default:true" json:"is_approved"`
	EmailCode        *string    `gorm:"size:16" json:"-"`
	EmailCodeExpires *time.Time `json:"-"`

	Locale        string      `gorm:"size:8;default:zh" json:"locale"`
	Theme         string      `gorm:"size:32;default:light" json:"theme"`
	BaseCurrency  string      `gorm:"size:8;default:CNY" json:"base_currency"`
	CategoryOrder JSON[[]int] `json:"category_order"`

	TelegramEnabled  bool    `gorm:"default:false" json:"telegram_enabled"`
	TelegramBotToken *string `gorm:"size:128" json:"telegram_bot_token"`
	TelegramChatID   *string `gorm:"size:64" json:"telegram_chat_id"`
	TelegramAdminID  *string `gorm:"size:64" json:"telegram_admin_id"`
	TelegramAPIBase  *string `gorm:"column:telegram_api_base;size:255" json:"telegram_api_base"`
	TelegramProxy    *string `gorm:"size:255" json:"telegram_proxy"`

	NotifyConfig   JSON[map[string]map[string]any] `json:"-"`
	CalendarToken  *string                         `gorm:"size:64;index" json:"-"`
	MonthlyBudget  *float64                        `json:"monthly_budget"`
	NotifySettings JSON[map[string]any]            `json:"notify_settings"`

	TotpSecret  *string `gorm:"size:64" json:"-"`
	TotpEnabled bool    `gorm:"default:false" json:"totp_enabled"`

	// 新增：WebDAV 备份配置（密码加密存储）
	WebdavConfig JSON[map[string]any] `gorm:"column:webdav_config" json:"-"`
	// 新增：是否已为该用户初始化个人货币 / 付款方式（首次访问时从默认值复制）
	CurrenciesSeeded     bool `gorm:"default:false" json:"-"`
	PaymentMethodsSeeded bool `gorm:"default:false" json:"-"`
	ServicesSeeded       bool `gorm:"default:false" json:"-"`

	CreatedAt time.Time `json:"created_at"`
}

type Category struct {
	ID       int     `gorm:"primaryKey" json:"id"`
	UserID   *int    `gorm:"index" json:"-"`
	Name     string  `gorm:"size:64" json:"name"`
	Icon     *string `gorm:"size:128" json:"icon"`
	Color    *string `gorm:"size:16" json:"color"`
	IsSystem bool    `gorm:"default:false" json:"is_system"`
	Sort     int     `gorm:"default:0" json:"sort"`
}

// PaymentMethod：每个用户独立一份（首次使用时由默认值复制），可填写账号与备注。
// user_id 为空的行是旧版遗留的系统预置项，仅用于兼容迁移，不再展示。
type PaymentMethod struct {
	ID       int     `gorm:"primaryKey" json:"id"`
	UserID   *int    `gorm:"index" json:"-"`
	Name     string  `gorm:"size:64" json:"name"`
	Icon     *string `gorm:"size:512" json:"icon"` // emoji 或图片 URL
	Account  *string `gorm:"size:255" json:"account"`
	Note     *string `gorm:"size:500" json:"note"`
	Sort     int     `gorm:"default:0" json:"sort"`
	IsSystem bool    `gorm:"default:false" json:"is_system"`
}

// UserCurrency：每个用户自己的货币清单（可增删），初始化时写入默认的 21 种常用货币。
type UserCurrency struct {
	ID     int    `gorm:"primaryKey" json:"id"`
	UserID int    `gorm:"uniqueIndex:uq_user_currency" json:"-"`
	Code   string `gorm:"size:16;uniqueIndex:uq_user_currency" json:"code"`
	Name   string `gorm:"size:64" json:"name"`    // 英文名
	NameZh string `gorm:"size:64" json:"name_zh"` // 中文名
	Symbol string `gorm:"size:16" json:"symbol"`
	Sort   int    `gorm:"default:0" json:"sort"`
	// 手动汇率：1 单位系统基准货币（EXCHANGE_API_BASE）= ManualRate 单位本货币。
	// 汇率接口没有该货币（如自定义币种）时使用；为空则使用接口汇率。
	ManualRate *float64  `json:"manual_rate"`
	IsCustom   bool      `gorm:"default:false" json:"is_custom"`
	CreatedAt  time.Time `json:"-"`
}

func (UserCurrency) TableName() string { return "user_currencies" }

// UserService：每个用户自己的服务库（订阅表单的服务联想与按分类浏览），
// 首次使用时从内置清单复制，之后可增删改。
type UserService struct {
	ID       int     `gorm:"primaryKey" json:"id"`
	UserID   int     `gorm:"index" json:"-"`
	Name     string  `gorm:"size:128" json:"name"`
	Website  string  `gorm:"size:512" json:"website"`
	Domain   string  `gorm:"size:255;index" json:"domain"` // 由 Website 解析，用于图标库取 favicon
	Category string  `gorm:"size:32" json:"category"`      // 服务分类 key（streaming / ai / ...）
	Icon     *string `gorm:"size:512" json:"custom_icon"`  // 自定义图标（emoji / URL）；为空时使用 Domain 的 favicon
	Sort     int     `gorm:"default:0" json:"sort"`
}

func (UserService) TableName() string { return "user_services" }

var domainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// NormalizeDomain 从网址或域名中取出小写主机名；无法识别时返回空串。
func NormalizeDomain(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if !domainRe.MatchString(host) {
		return ""
	}
	return host
}

// LegacyCurrency 是原版的全局货币表，仅在迁移旧数据时读取。
type LegacyCurrency struct {
	Code     string `gorm:"primaryKey;size:8"`
	Name     string `gorm:"size:64"`
	Symbol   string `gorm:"size:8"`
	IsCustom bool
	UserID   *int
}

func (LegacyCurrency) TableName() string { return "currencies" }

type ExchangeRate struct {
	ID        int       `gorm:"primaryKey"`
	Base      string    `gorm:"size:8;uniqueIndex:uq_base_quote;index"`
	Quote     string    `gorm:"size:8;uniqueIndex:uq_base_quote;index"`
	Rate      float64   `gorm:"type:double"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

type Subscription struct {
	ID     int `gorm:"primaryKey" json:"id"`
	UserID int `gorm:"index" json:"-"`

	Name   string  `gorm:"size:128" json:"name"`
	Plan   *string `gorm:"size:128" json:"plan"`
	Icon   *string `gorm:"size:512" json:"icon"`
	URL    *string `gorm:"column:url;size:512" json:"url"`
	Notes  *string `gorm:"type:text" json:"notes"`
	Remark *string `gorm:"size:255" json:"remark"`
	IPv4   *string `gorm:"column:ipv4;size:64" json:"ipv4"`
	IPv6   *string `gorm:"column:ipv6;size:64" json:"ipv6"`

	CategoryID      *int `json:"category_id"`
	PaymentMethodID *int `json:"payment_method_id"`
	BundleID        *int `json:"bundle_id"`

	Amount   float64 `gorm:"type:double;default:0" json:"amount"`
	Currency string  `gorm:"size:16;default:CNY" json:"currency"`

	BillingType string `gorm:"size:16;default:recurring" json:"billing_type"`
	Cycle       string `gorm:"size:16;default:month" json:"cycle"`
	CycleCount  int    `gorm:"default:1" json:"cycle_count"`

	StartDate       Date  `gorm:"type:date" json:"start_date"`
	NextRenewalDate *Date `gorm:"type:date" json:"next_renewal_date"`
	EndDate         *Date `gorm:"type:date" json:"end_date"`
	LastRenewedAt   *Date `gorm:"type:date" json:"last_renewed_at"`

	TrialEnd   *Date   `gorm:"type:date" json:"trial_end"`
	CancelBy   *Date   `gorm:"type:date" json:"cancel_by"`
	CardLast4  *string `gorm:"column:card_last4;size:8" json:"card_last4"`
	CardExpiry *string `gorm:"size:8" json:"card_expiry"`

	IsActive       bool `gorm:"default:true" json:"is_active"`
	AutoRenew      bool `gorm:"default:true" json:"auto_renew"`
	ShowInCalendar bool `gorm:"default:true" json:"show_in_calendar"`
	Sort           int  `gorm:"default:0" json:"sort"`

	FamilyMembers    JSON[[]string] `json:"family_members"`
	RemindDaysBefore string         `gorm:"size:64;default:7,6,5,4,3,2,1" json:"remind_days_before"`

	CreatedAt time.Time `json:"created_at"`
}

type Bundle struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	UserID    int       `gorm:"index" json:"-"`
	Name      string    `gorm:"size:128" json:"name"`
	Note      *string   `gorm:"size:255" json:"note"`
	CreatedAt time.Time `json:"-"`
}

type ApiToken struct {
	ID         int    `gorm:"primaryKey"`
	UserID     int    `gorm:"index"`
	Name       string `gorm:"size:64"`
	Token      string `gorm:"size:64;uniqueIndex"`
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

type ActivityLog struct {
	ID        int       `gorm:"primaryKey"`
	UserID    *int      `gorm:"index"`
	Username  *string   `gorm:"size:64"`
	Level     string    `gorm:"size:16;default:info"`
	Action    string    `gorm:"size:64"`
	Detail    *string   `gorm:"type:text"`
	CreatedAt time.Time `gorm:"index"`
}

func (ActivityLog) TableName() string { return "activity_log" }

type NotificationLog struct {
	ID             int       `gorm:"primaryKey"`
	SubscriptionID int       `gorm:"index"`
	UserID         int       `gorm:"index"`
	DaysBefore     int       `gorm:"column:days_before"`
	Channel        string    `gorm:"size:16;default:telegram"`
	Status         string    `gorm:"size:16"`
	Message        *string   `gorm:"type:text"`
	SentAt         time.Time `gorm:"index"`
}

func (NotificationLog) TableName() string { return "notification_log" }

// AllModels 供 AutoMigrate 使用。
func AllModels() []any {
	return []any{
		&User{}, &Category{}, &PaymentMethod{}, &UserCurrency{}, &UserService{}, &ExchangeRate{},
		&Bundle{}, &Subscription{}, &ApiToken{}, &ActivityLog{}, &NotificationLog{},
	}
}
