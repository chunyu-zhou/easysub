package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"easysub/internal/config"
	"easysub/internal/store"
)

func init() { config.C = &config.Config{JWTSecret: "test"} }

func TestStripMD(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"*粗体* _斜体_ `code`": "粗体 斜体 code",
		`a\*b\_c` + "\\`d": "abcd",
		`\[链接](x)`:         "[链接](x)",
		"plain text 无标记":   "plain text 无标记",
	}
	for in, want := range cases {
		if got := StripMD(in); got != want {
			t.Errorf("StripMD(%q)=%q want %q", in, got, want)
		}
	}
}

func TestRenderTemplate(t *testing.T) {
	got := renderTemplate("[{{ event }}] {{subject}}: {{text}} @{{timestamp}} {{unknown}}",
		map[string]any{"text": "T", "subject": "S", "event": "reminder", "timestamp": int64(42)})
	want := "[reminder] S: T @42 {{unknown}}"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestTruncBytes(t *testing.T) {
	if truncBytes("abc", 10) != "abc" {
		t.Fatal("short text should be unchanged")
	}
	s := strings.Repeat("中", 10) // 30 字节
	got := truncBytes(s, 10)
	if !utf8.ValidString(got) || len(got) > 10 || !strings.HasSuffix(got, "...") {
		t.Fatalf("bad truncation %q (%d bytes)", got, len(got))
	}
	if got != "中中..." {
		t.Fatalf("got %q", got)
	}
}

func TestSplitCSVAndInWindow(t *testing.T) {
	if got := splitCSV(" a, b，c ,, "); strings.Join(got, "|") != "a|b|c" {
		t.Fatalf("splitCSV=%v", got)
	}
	if !inWindow("23:30", "23:00", "07:00") || !inWindow("06:59", "23:00", "07:00") || inWindow("12:00", "23:00", "07:00") {
		t.Fatal("cross-midnight window wrong")
	}
	if !inWindow("12:00", "09:00", "18:00") || inWindow("18:00", "09:00", "18:00") {
		t.Fatal("same-day window wrong")
	}
}

func TestDefaultConfigIsDeepCopy(t *testing.T) {
	a := DefaultConfig()
	a["bark"]["urls"] = append(a["bark"]["urls"].([]any), "x")
	a["email"]["port"] = float64(25)
	b := DefaultConfig()
	if len(b["bark"]["urls"].([]any)) != 0 || b["email"]["port"] != float64(465) {
		t.Fatal("DefaultConfig not isolated")
	}
	if len(Channels) != 14 || len(b) != 14 {
		t.Fatal("expected 14 channels")
	}
}

func TestApplyLoadRoundTrip(t *testing.T) {
	u := &store.User{}
	in := map[string]any{
		"telegram": map[string]any{"enabled": true, "bot_token": "123:abc", "chat_id": "42", "bogus": 1},
		"email":    map[string]any{"enabled": true, "host": "smtp.x", "password": "p@ss"},
		"unknown":  map[string]any{"enabled": true},
	}
	cfg := ApplyConfig(u, in)
	if cfg["telegram"]["bot_token"] != "123:abc" || cfg["unknown"] != nil || cfg["telegram"]["bogus"] != nil {
		t.Fatalf("normalized config wrong: %v", cfg["telegram"])
	}
	if !u.TelegramEnabled || deref(u.TelegramBotToken) != "123:abc" || u.TelegramAdminID != nil {
		t.Fatal("legacy telegram columns not synced")
	}
	stored := u.NotifyConfig.V
	if tok := stored["telegram"]["bot_token"].(string); !strings.HasPrefix(tok, "enc:") {
		t.Fatalf("secret not encrypted: %q", tok)
	}
	if pw := stored["email"]["password"].(string); !strings.HasPrefix(pw, "enc:") {
		t.Fatalf("secret not encrypted: %q", pw)
	}
	// 经过 JSON 序列化后重新读取
	raw, _ := json.Marshal(u.NotifyConfig)
	var u2 store.User
	if err := json.Unmarshal(raw, &u2.NotifyConfig); err != nil {
		t.Fatal(err)
	}
	got := LoadConfig(&u2)
	if got["telegram"]["bot_token"] != "123:abc" || got["email"]["password"] != "p@ss" || got["email"]["port"] != float64(465) {
		t.Fatalf("round trip mismatch: %v / %v", got["telegram"], got["email"])
	}
	if !AnyEnabled(got) || AnyEnabled(DefaultConfig()) {
		t.Fatal("AnyEnabled wrong")
	}
}

func TestLoadConfigLegacyTelegram(t *testing.T) {
	tok, chat := "legacy-token", "99"
	u := &store.User{TelegramEnabled: true, TelegramBotToken: &tok, TelegramChatID: &chat}
	cfg := LoadConfig(u)
	tg := cfg["telegram"]
	if tg["enabled"] != true || tg["bot_token"] != tok || tg["chat_id"] != chat || tg["proxy"] != "" {
		t.Fatalf("legacy fallback wrong: %v", tg)
	}
}

func TestWebhookSignatureAndRetry(t *testing.T) {
	var hits atomic.Int32
	var gotSig, gotHdr string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(500)
			return
		}
		gotSig, gotHdr = r.Header.Get("X-EasySub-Signature"), r.Header.Get("X-Custom")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
	}))
	defer srv.Close()
	conf := DefaultConfig()["webhook"]
	conf["urls"] = []any{srv.URL}
	conf["secret"] = "s"
	conf["template"] = "{{subject}}|{{text}}"
	conf["headers"] = []any{map[string]any{"key": "X-Custom", "value": "v"}, map[string]any{"key": "Content-Type", "value": "x"}}
	if err := SendOne(context.Background(), "webhook", conf, "主题", "*正文*"); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 || len(gotSig) != 64 || gotHdr != "v" || gotBody["text"] != "主题|正文" || gotBody["event"] != "reminder" {
		t.Fatalf("hits=%d sig=%q hdr=%q body=%v", hits.Load(), gotSig, gotHdr, gotBody)
	}
}

func TestSendOneUnknownAndMissing(t *testing.T) {
	if err := SendOne(context.Background(), "nope", nil, "", ""); err == nil || err.Error() != "未知渠道：nope" {
		t.Fatalf("got %v", err)
	}
	if err := SendOne(context.Background(), "discord", map[string]any{}, "", "x"); err == nil || !strings.Contains(err.Error(), "Discord 未配置") {
		t.Fatalf("got %v", err)
	}
}
