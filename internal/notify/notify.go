// Package notify 统一多渠道通知服务。
//
// 在 Telegram 之外支持：飞书 / QQ / Bark / Email / Pushplus / Server酱 / 企业微信 /
// 钉钉 / Discord / Slack / ntfy / Gotify / Webhook。
// 每个用户的渠道配置保存在 users.notify_config（JSON）中；Telegram 为兼容旧版，
// 未写入 notify_config 时回退读取旧的 telegram_* 列。
package notify

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"easysub/internal/config"
	"easysub/internal/secure"
	"easysub/internal/store"
)

// Config 为全部渠道配置：渠道名 -> 字段 -> 值。
type Config = map[string]map[string]any

// Channels 渠道顺序（与原版 _DEFAULTS 键顺序一致）。
var Channels = []string{
	"telegram", "feishu", "qq", "bark", "email", "pushplus", "serverchan",
	"wecom", "dingtalk", "discord", "slack", "ntfy", "gotify", "webhook",
}

// 各渠道需加密存储的敏感字段
var secretFields = map[string][]string{
	"telegram": {"bot_token"}, "feishu": {"app_secret"}, "qq": {"app_secret"},
	"email": {"password"}, "pushplus": {"token"}, "serverchan": {"sendkey"},
	"wecom":    {"secret"},
	"dingtalk": {"secret"}, "ntfy": {"token"}, "gotify": {"token"}, "webhook": {"secret"},
}

var defaults = Config{
	"telegram": {"enabled": false, "bot_token": "", "chat_id": "", "admin_id": "",
		"api_base": "", "proxy": ""},
	"feishu": {"enabled": false, "app_id": "", "app_secret": "", "chat_ids": ""},
	"qq":     {"enabled": false, "app_id": "", "app_secret": "", "group_ids": "", "user_ids": ""},
	"bark":   {"enabled": false, "urls": []any{}, "group": "", "level": "active", "icon": ""},
	"email": {"enabled": false, "host": "", "port": float64(465), "ssl": true, "username": "",
		"password": "", "from": "", "to": ""},
	"pushplus":   {"enabled": false, "token": "", "topic": "", "channel": "wechat"},
	"serverchan": {"enabled": false, "sendkey": ""},
	// 企业微信：mode=webhook 群机器人；mode=app 自建应用（企业ID+应用ID+Secret，可走 API 代理）
	"wecom": {"enabled": false, "mode": "webhook", "url": "",
		"corp_id": "", "agent_id": "", "secret": "", "proxy_base": "",
		"to_user": "@all", "to_party": "", "to_tag": "",
		"msg_type": "text", "card_url": ""},
	"dingtalk": {"enabled": false, "url": "", "secret": ""},
	"discord":  {"enabled": false, "url": ""},
	"slack":    {"enabled": false, "url": ""},
	"ntfy":     {"enabled": false, "server": "https://ntfy.sh", "topic": "", "token": ""},
	"gotify":   {"enabled": false, "server": "", "token": "", "priority": float64(5)},
	"webhook": {"enabled": false, "urls": []any{}, "secret": "", "headers": []any{}, "template": "",
		"timeout_ms": float64(5000), "max_retries": float64(3)},
}

// DefaultConfig 返回全部渠道默认配置的深拷贝。
func DefaultConfig() Config { return copyConfig(defaults) }

func copyConfig(c Config) Config {
	out := make(Config, len(c))
	for ch, sub := range c {
		out[ch] = deepCopy(sub).(map[string]any)
	}
	return out
}

func deepCopy(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, vv := range x {
			m[k] = deepCopy(vv)
		}
		return m
	case []any:
		s := make([]any, len(x))
		for i, vv := range x {
			s[i] = deepCopy(vv)
		}
		return s
	case []string:
		return append([]string(nil), x...)
	default:
		return v
	}
}

// merge 把 src 中默认结构已有的键覆盖进 cfg。
func merge(cfg Config, ch string, src map[string]any) {
	dst, ok := cfg[ch]
	if !ok || src == nil {
		return
	}
	for k, v := range src {
		if _, known := dst[k]; known {
			dst[k] = deepCopy(v)
		}
	}
}

// LoadConfig 读取某用户完整配置（解密敏感字段，含 telegram 旧列回退）。
func LoadConfig(u *store.User) Config {
	cfg := DefaultConfig()
	var saved map[string]map[string]any
	if u.NotifyConfig.Valid {
		saved = u.NotifyConfig.V
	}
	for ch, sub := range saved {
		merge(cfg, ch, sub)
	}
	// 解密敏感字段（历史明文原样返回）
	for ch, fields := range secretFields {
		for _, f := range fields {
			if s, ok := cfg[ch][f].(string); ok && s != "" {
				cfg[ch][f] = secure.Decrypt(s)
			}
		}
	}
	// 兼容旧版：telegram 尚未存入 notify_config 时用旧列回填
	if len(saved["telegram"]) == 0 {
		tg := cfg["telegram"]
		tg["enabled"] = u.TelegramEnabled
		tg["bot_token"] = deref(u.TelegramBotToken)
		tg["chat_id"] = deref(u.TelegramChatID)
		tg["admin_id"] = deref(u.TelegramAdminID)
		tg["api_base"] = deref(u.TelegramAPIBase)
		tg["proxy"] = deref(u.TelegramProxy)
	}
	return cfg
}

// ApplyConfig 把前端提交的配置合并进默认结构后写回 u.NotifyConfig（敏感字段加密），
// 同步 telegram 旧列，返回规范化后的明文配置。调用方负责持久化 u。
func ApplyConfig(u *store.User, incoming map[string]any) Config {
	cfg := DefaultConfig()
	for ch, sub := range incoming {
		if m, ok := sub.(map[string]any); ok {
			merge(cfg, ch, m)
		}
	}
	// 同步 telegram 到旧列（保持明文，验证机器人/测试等旧路径直接可用）
	tg := cfg["telegram"]
	u.TelegramEnabled = truthy(tg["enabled"])
	u.TelegramBotToken = ptrOrNil(str(tg, "bot_token"))
	u.TelegramChatID = ptrOrNil(str(tg, "chat_id"))
	u.TelegramAdminID = ptrOrNil(str(tg, "admin_id"))
	u.TelegramAPIBase = ptrOrNil(str(tg, "api_base"))
	u.TelegramProxy = ptrOrNil(str(tg, "proxy"))
	// 敏感字段加密后再写入 notify_config
	stored := copyConfig(cfg)
	for ch, fields := range secretFields {
		for _, f := range fields {
			if v := stored[ch][f]; truthy(v) {
				stored[ch][f] = secure.Encrypt(toStr(v))
			}
		}
	}
	u.NotifyConfig = store.JSONOf(stored)
	return cfg
}

// AnyEnabled 是否至少启用了一个渠道。
func AnyEnabled(cfg Config) bool {
	for _, ch := range Channels {
		if truthy(cfg[ch]["enabled"]) {
			return true
		}
	}
	return false
}

// ---- 文本处理 ---- //

var mdRe = regexp.MustCompile("[*_`]")

// StripMD 去掉 Markdown 强调符号，供不支持 Markdown 的渠道使用。
func StripMD(s string) string {
	if s == "" {
		return ""
	}
	s = strings.NewReplacer(`\*`, "*", `\_`, "_", "\\`", "`", `\[`, "[").Replace(s)
	return mdRe.ReplaceAllString(s, "")
}

func splitCSV(s string) []string {
	var out []string
	for _, x := range strings.Split(strings.ReplaceAll(s, "，", ","), ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

func withSubject(subject, text string) string {
	if subject != "" {
		return subject + "\n\n" + text
	}
	return text
}

// ---- 发送与分发 ---- //

type sender func(ctx context.Context, conf map[string]any, subject, text string) error

var senders = map[string]sender{
	"telegram":   sendTelegram,
	"feishu":     sendFeishu,
	"qq":         sendQQ,
	"bark":       sendBark,
	"email":      sendEmail,
	"pushplus":   sendPushplus,
	"serverchan": sendServerchan,
	"wecom":      sendWecom,
	"dingtalk":   sendDingtalk,
	"discord":    sendDiscord,
	"slack":      sendSlack,
	"ntfy":       sendNtfy,
	"gotify":     sendGotify,
	"webhook": func(ctx context.Context, conf map[string]any, subject, text string) error {
		return sendWebhook(ctx, conf, subject, text, "reminder")
	},
}

// SendOne 按单个渠道配置发送（供测试按钮使用）。非 Telegram 渠道用纯文本。
func SendOne(ctx context.Context, channel string, conf map[string]any, subject, text string) error {
	fn, ok := senders[channel]
	if !ok {
		return fmt.Errorf("未知渠道：%s", channel)
	}
	if channel != "telegram" {
		text = StripMD(text)
	}
	return fn(ctx, conf, subject, text)
}

// Result 单个渠道的推送结果。
type Result struct {
	Channel string `json:"channel"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

// Dispatch 向该用户所有已启用渠道推送，返回每个渠道的结果。
func Dispatch(ctx context.Context, u *store.User, subject, textPlain, textMD, event string) []Result {
	if event == "" {
		event = "reminder"
	}
	cfg := LoadConfig(u)
	var results []Result
	for _, ch := range Channels {
		conf := cfg[ch]
		if !truthy(conf["enabled"]) {
			continue
		}
		var err error
		switch ch {
		case "telegram":
			body := textMD
			if body == "" {
				body = textPlain
			}
			err = sendTelegram(ctx, conf, subject, body)
		case "webhook":
			err = sendWebhook(ctx, conf, subject, textPlain, event)
		default:
			err = senders[ch](ctx, conf, subject, textPlain)
		}
		if err != nil {
			results = append(results, Result{Channel: ch, OK: false, Error: err.Error()})
		} else {
			results = append(results, Result{Channel: ch, OK: true})
		}
	}
	return results
}

// InQuietHours 当前本地时间是否落在用户设置的免打扰时段内。支持跨午夜（如 23:00-07:00）。
func InQuietHours(u *store.User) bool {
	if !u.NotifySettings.Valid {
		return false
	}
	st := u.NotifySettings.V
	q1, q2 := str(st, "quiet_start"), str(st, "quiet_end")
	if q1 == "" || q2 == "" {
		return false
	}
	return inWindow(config.Now().Format("15:04"), q1, q2)
}

func inWindow(now, q1, q2 string) bool {
	if q1 <= q2 {
		return q1 <= now && now < q2
	}
	return now >= q1 || now < q2 // 跨午夜窗口
}
