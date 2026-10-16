package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const defaultTitle = "省心订阅 EasySub"
const defaultNotifyTitle = "省心订阅 EasySub 通知"

func sendTelegram(ctx context.Context, conf map[string]any, subject, text string) error {
	token, chatID := str(conf, "bot_token"), str(conf, "chat_id")
	if token == "" || chatID == "" {
		return errors.New("Telegram 未配置 Bot Token 或 Chat ID")
	}
	return TelegramSend(ctx, chatID, text, token, str(conf, "api_base"), str(conf, "proxy"))
}

func sendFeishu(ctx context.Context, conf map[string]any, subject, text string) error {
	appID, secret := str(conf, "app_id"), str(conf, "app_secret")
	chatIDs := splitCSV(str(conf, "chat_ids"))
	if appID == "" || secret == "" || len(chatIDs) == 0 {
		return errors.New("飞书未配置 App ID / Secret / Chat IDs")
	}
	c := defaultClient()
	r, err := postJSON(ctx, c, "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal", nil, nil,
		map[string]any{"app_id": appID, "app_secret": secret})
	if err != nil {
		return err
	}
	data, err := r.checkedJSON()
	if err != nil {
		return err
	}
	token := str(data, "tenant_access_token")
	if token == "" {
		return fmt.Errorf("飞书获取 token 失败：%s", r.text())
	}
	content := string(marshal(map[string]any{"text": withSubject(subject, text)}))
	for _, cid := range chatIDs {
		resp, err := postJSON(ctx, c, "https://open.feishu.cn/open-apis/im/v1/messages",
			url.Values{"receive_id_type": {"chat_id"}},
			map[string]string{"Authorization": "Bearer " + token},
			map[string]any{"receive_id": cid, "msg_type": "text", "content": content})
		if err != nil {
			return err
		}
		d, err := resp.json()
		if err != nil {
			return err
		}
		if jsonCode(d, "code", 0) != 0 {
			return fmt.Errorf("飞书发送失败（%s）：%s", cid, pyVal(d["msg"]))
		}
	}
	return nil
}

func sendQQ(ctx context.Context, conf map[string]any, subject, text string) error {
	appID, secret := str(conf, "app_id"), str(conf, "app_secret")
	groups := splitCSV(str(conf, "group_ids"))
	users := splitCSV(str(conf, "user_ids"))
	if appID == "" || secret == "" {
		return errors.New("QQ 未配置 App ID / Secret")
	}
	if len(groups) == 0 && len(users) == 0 {
		return errors.New("QQ 未填写任何群聊或私聊 OpenID")
	}
	body := withSubject(subject, text)
	c := defaultClient()
	r, err := postJSON(ctx, c, "https://bots.qq.com/app/getAppAccessToken", nil, nil,
		map[string]any{"appId": appID, "clientSecret": secret})
	if err != nil {
		return err
	}
	data, err := r.checkedJSON()
	if err != nil {
		return err
	}
	token := str(data, "access_token")
	if token == "" {
		return fmt.Errorf("QQ 获取 access_token 失败：%s", r.text())
	}
	hdr := map[string]string{"Authorization": "QQBot " + token}
	payload := map[string]any{"content": body, "msg_type": 0}
	for _, gid := range groups {
		resp, err := postJSON(ctx, c, "https://api.sgroup.qq.com/v2/groups/"+url.PathEscape(gid)+"/messages", nil, hdr, payload)
		if err != nil {
			return err
		}
		if resp.status >= 300 {
			return fmt.Errorf("QQ 群 %s 发送失败：%s", gid, resp.text())
		}
	}
	for _, uid := range users {
		resp, err := postJSON(ctx, c, "https://api.sgroup.qq.com/v2/users/"+url.PathEscape(uid)+"/messages", nil, hdr, payload)
		if err != nil {
			return err
		}
		if resp.status >= 300 {
			return fmt.Errorf("QQ 用户 %s 发送失败：%s", uid, resp.text())
		}
	}
	return nil
}

func sendBark(ctx context.Context, conf map[string]any, subject, text string) error {
	urls := strList(conf, "urls")
	if len(urls) == 0 {
		return errors.New("Bark 未配置任何目标 URL")
	}
	title := subject
	if title == "" {
		title = defaultTitle
	}
	payload := map[string]any{"title": title, "body": text}
	for _, k := range []string{"group", "level", "icon"} {
		if v := str(conf, k); v != "" {
			payload[k] = v
		}
	}
	c := defaultClient()
	for _, u := range urls {
		r, err := postJSON(ctx, c, strings.TrimRight(u, "/"), nil, nil, payload)
		if err != nil {
			return err
		}
		data, err := r.checkedJSON()
		if err != nil {
			return err
		}
		if code := jsonCode(data, "code", 200); code != 200 && code != 0 {
			return fmt.Errorf("Bark 推送失败：%s", r.text())
		}
	}
	return nil
}

func sendPushplus(ctx context.Context, conf map[string]any, subject, text string) error {
	token := str(conf, "token")
	if token == "" {
		return errors.New("Pushplus 未配置 Token")
	}
	title := subject
	if title == "" {
		title = defaultNotifyTitle
	}
	body := map[string]any{"token": token, "title": title, "content": text, "template": "txt"}
	if v := str(conf, "topic"); v != "" {
		body["topic"] = v
	}
	if v := str(conf, "channel"); v != "" {
		body["channel"] = v
	}
	r, err := postJSON(ctx, defaultClient(), "https://www.pushplus.plus/send", nil, nil, body)
	if err != nil {
		return err
	}
	data, err := r.checkedJSON()
	if err != nil {
		return err
	}
	if jsonCode(data, "code", noCode) != 200 {
		return fmt.Errorf("Pushplus 发送失败：%s", pyVal(data["msg"]))
	}
	return nil
}

func sendServerchan(ctx context.Context, conf map[string]any, subject, text string) error {
	key := str(conf, "sendkey")
	if key == "" {
		return errors.New("Server酱未配置 SendKey")
	}
	title := subject
	if title == "" {
		title = defaultTitle
	}
	form := url.Values{"title": {title}, "desp": {text}}
	r, err := do(ctx, defaultClient(), request{method: http.MethodPost,
		url:  "https://sctapi.ftqq.com/" + url.PathEscape(key) + ".send",
		body: []byte(form.Encode()), ctype: "application/x-www-form-urlencoded"})
	if err != nil {
		return err
	}
	data, err := r.checkedJSON()
	if err != nil {
		return err
	}
	if jsonCode(data, "code", 0) != 0 {
		return fmt.Errorf("Server酱发送失败：%s", r.text())
	}
	return nil
}

func sendDingtalk(ctx context.Context, conf map[string]any, subject, text string) error {
	u := str(conf, "url")
	if u == "" {
		return errors.New("钉钉未配置机器人 Webhook")
	}
	if secret := str(conf, "secret"); secret != "" {
		ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(ts + "\n" + secret))
		sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		u = u + "&timestamp=" + ts + "&sign=" + sign
	}
	r, err := postJSON(ctx, defaultClient(), u, nil, nil,
		map[string]any{"msgtype": "text", "text": map[string]any{"content": withSubject(subject, text)}})
	if err != nil {
		return err
	}
	data, err := r.checkedJSON()
	if err != nil {
		return err
	}
	if jsonCode(data, "errcode", 0) != 0 {
		return fmt.Errorf("钉钉发送失败：%s", r.text())
	}
	return nil
}

func sendDiscord(ctx context.Context, conf map[string]any, subject, text string) error {
	u := str(conf, "url")
	if u == "" {
		return errors.New("Discord 未配置 Webhook URL")
	}
	content := text
	if subject != "" {
		content = "**" + subject + "**\n" + text
	}
	r, err := postJSON(ctx, defaultClient(), u, nil, nil, map[string]any{"content": truncRunes(content, 1900)})
	if err != nil {
		return err
	}
	return r.check()
}

func sendSlack(ctx context.Context, conf map[string]any, subject, text string) error {
	u := str(conf, "url")
	if u == "" {
		return errors.New("Slack 未配置 Webhook URL")
	}
	body := text
	if subject != "" {
		body = "*" + subject + "*\n" + text
	}
	r, err := postJSON(ctx, defaultClient(), u, nil, nil, map[string]any{"text": body})
	if err != nil {
		return err
	}
	return r.check()
}

func sendNtfy(ctx context.Context, conf map[string]any, subject, text string) error {
	server := str(conf, "server")
	if server == "" {
		server = "https://ntfy.sh"
	}
	server = strings.TrimRight(server, "/")
	topic := str(conf, "topic")
	if topic == "" {
		return errors.New("ntfy 未配置 Topic")
	}
	hdr := map[string]string{}
	if tok := str(conf, "token"); tok != "" {
		hdr["Authorization"] = "Bearer " + tok
	}
	r, err := do(ctx, defaultClient(), request{method: http.MethodPost, url: server + "/" + topic,
		headers: hdr, body: []byte(withSubject(subject, text))})
	if err != nil {
		return err
	}
	return r.check()
}

func sendGotify(ctx context.Context, conf map[string]any, subject, text string) error {
	server := strings.TrimRight(str(conf, "server"), "/")
	token := str(conf, "token")
	if server == "" || token == "" {
		return errors.New("Gotify 未配置 Server / Token")
	}
	title := subject
	if title == "" {
		title = defaultTitle
	}
	r, err := postJSON(ctx, defaultClient(), server+"/message", url.Values{"token": {token}}, nil,
		map[string]any{"title": title, "message": text, "priority": int(num(conf, "priority", 5))})
	if err != nil {
		return err
	}
	return r.check()
}

// ---- Webhook ---- //

var tplRe = regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`)

// renderTemplate 替换 {{var}}，未知变量原样保留。
func renderTemplate(tpl string, vars map[string]any) string {
	return tplRe.ReplaceAllStringFunc(tpl, func(m string) string {
		name := tplRe.FindStringSubmatch(m)[1]
		if v, ok := vars[name]; ok {
			return toStr(v)
		}
		return m
	})
}

type webhookPayload struct {
	Text      string `json:"text"`
	Subject   string `json:"subject"`
	Event     string `json:"event"`
	Timestamp int64  `json:"timestamp"`
}

func sendWebhook(ctx context.Context, conf map[string]any, subject, text, event string) error {
	urls := strList(conf, "urls")
	if len(urls) == 0 {
		return errors.New("Webhook 未配置任何目标 URL")
	}
	ts := time.Now().Unix()
	rendered := text
	if tpl := str(conf, "template"); tpl != "" {
		rendered = renderTemplate(tpl, map[string]any{"text": text, "subject": subject, "event": event, "timestamp": ts})
	}
	raw := marshal(webhookPayload{Text: rendered, Subject: subject, Event: event, Timestamp: ts})

	hdr := map[string]string{}
	if hs, ok := conf["headers"].([]any); ok {
		for _, h := range hs {
			m, ok := h.(map[string]any)
			if !ok {
				continue
			}
			k := str(m, "key")
			if k == "" {
				continue
			}
			if lk := strings.ToLower(k); lk == "content-type" || lk == "x-easysub-signature" {
				continue
			}
			hdr[k] = str(m, "value")
		}
	}
	if secret := str(conf, "secret"); secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(raw)
		hdr["X-EasySub-Signature"] = hex.EncodeToString(mac.Sum(nil))
	}

	timeoutMs := max(1, int(num(conf, "timeout_ms", 5000)))
	retries := max(0, int(num(conf, "max_retries", 0)))
	c, _ := newClient(time.Duration(timeoutMs)*time.Millisecond, "")
	for _, u := range urls {
		var lastErr error
		for attempt := 0; attempt <= retries; attempt++ {
			r, err := do(ctx, c, request{method: http.MethodPost, url: u, headers: hdr, body: raw, ctype: "application/json"})
			if err == nil {
				err = r.check()
			}
			lastErr = err
			if err == nil || ctx.Err() != nil {
				break
			}
		}
		if lastErr != nil {
			return fmt.Errorf("Webhook 发送失败（%s）：%v", u, lastErr)
		}
	}
	return nil
}
