package notify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const wecomDefaultBase = "https://qyapi.weixin.qq.com"

// 自建应用的 access_token 缓存。企业微信对 gettoken 有频率限制，
// 且同一 Secret 重复获取会顶掉旧 token，因此必须缓存复用（官方有效期 7200s）。
var (
	wecomMu     sync.Mutex
	wecomTokens = map[string]wecomToken{}
)

type wecomToken struct {
	token   string
	expires time.Time
}

// 需要重新获取 token 的错误码：40014 不合法的 access_token / 42001 已过期 / 41001 缺少 token
func wecomTokenErr(code int) bool { return code == 40014 || code == 42001 || code == 41001 }

// truncBytes 按 UTF-8 字节数截断（企业微信 text 上限 2048B、markdown 4096B、textcard 描述 512B）。
func truncBytes(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	i := max(limit-3, 0)
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	return s[:i] + "..."
}

// wecomAPIBase API 代理地址；留空用官方域名。容忍用户把 /cgi-bin 一起粘进来。
func wecomAPIBase(conf map[string]any) string {
	base := strings.TrimRight(strings.TrimSpace(str(conf, "proxy_base")), "/")
	if base == "" {
		return wecomDefaultBase
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "https://" + base
	}
	return strings.TrimSuffix(base, "/cgi-bin")
}

// wecomGetToken 获取（并缓存）自建应用 access_token。
func wecomGetToken(ctx context.Context, conf map[string]any, force bool) (string, error) {
	corpID := strings.TrimSpace(str(conf, "corp_id"))
	secret := strings.TrimSpace(str(conf, "secret"))
	if corpID == "" || secret == "" {
		return "", errors.New("企业微信自建应用未配置企业ID / 应用Secret")
	}
	base := wecomAPIBase(conf)
	sum := sha256.Sum256([]byte(secret))
	key := base + "|" + corpID + "|" + hex.EncodeToString(sum[:])[:16]
	if !force {
		wecomMu.Lock()
		hit, ok := wecomTokens[key]
		wecomMu.Unlock()
		if ok && hit.expires.After(time.Now()) {
			return hit.token, nil
		}
	}
	r, err := get(ctx, defaultClient(), base+"/cgi-bin/gettoken", url.Values{"corpid": {corpID}, "corpsecret": {secret}})
	if err != nil {
		return "", err
	}
	data, err := r.checkedJSON()
	if err != nil {
		return "", err
	}
	token := str(data, "access_token")
	if jsonCode(data, "errcode", 0) != 0 || token == "" {
		return "", fmt.Errorf("企业微信获取 access_token 失败：%s %s"+
			"（请检查企业ID / 应用Secret，以及是否已在「企业可信IP」放行服务器出口 IP）",
			pyVal(data["errcode"]), pyVal(data["errmsg"]))
	}
	ttl := int(num(data, "expires_in", 7200))
	wecomMu.Lock()
	wecomTokens[key] = wecomToken{token: token, expires: time.Now().Add(time.Duration(max(60, ttl-200)) * time.Second)} // 提前 200s 续期
	wecomMu.Unlock()
	return token, nil
}

func listLen(v any) int {
	if s, ok := v.([]any); ok {
		return len(s)
	}
	return 0
}

func subMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// WecomAgentInfo 读取自建应用信息（校验配置用）：返回应用名、可见范围人数等。
func WecomAgentInfo(ctx context.Context, conf map[string]any) (map[string]any, error) {
	agentID := strings.TrimSpace(str(conf, "agent_id"))
	if agentID == "" {
		return nil, errors.New("企业微信自建应用未配置应用ID（AgentId）")
	}
	base := wecomAPIBase(conf)
	for attempt := 0; attempt < 2; attempt++ {
		token, err := wecomGetToken(ctx, conf, attempt > 0)
		if err != nil {
			return nil, err
		}
		r, err := get(ctx, defaultClient(), base+"/cgi-bin/agent/get", url.Values{"access_token": {token}, "agentid": {agentID}})
		if err != nil {
			return nil, err
		}
		data, err := r.checkedJSON()
		if err != nil {
			return nil, err
		}
		code := jsonCode(data, "errcode", 0)
		if wecomTokenErr(code) && attempt == 0 {
			continue
		}
		if code != 0 {
			return nil, fmt.Errorf("企业微信读取应用失败：%s %s", pyVal(data["errcode"]), pyVal(data["errmsg"]))
		}
		return map[string]any{
			"agentid":         data["agentid"],
			"name":            data["name"],
			"square_logo_url": data["square_logo_url"],
			"users":           listLen(subMap(data["allow_userinfos"])["user"]),
			"parties":         listLen(subMap(data["allow_partys"])["partyid"]),
			"tags":            listLen(subMap(data["allow_tags"])["tagid"]),
		}, nil
	}
	return nil, errors.New("企业微信读取应用失败：access_token 反复失效")
}

// wecomPayload 按消息类型构造消息体（text / markdown / textcard）。
func wecomPayload(conf map[string]any, subject, text string) map[string]any {
	msgType := strings.TrimSpace(str(conf, "msg_type"))
	body := withSubject(subject, text)
	switch msgType {
	case "markdown":
		return map[string]any{"msgtype": "markdown", "markdown": map[string]any{"content": truncBytes(body, 4096)}}
	case "textcard":
		title := subject
		if title == "" {
			title = defaultTitle
		}
		link := str(conf, "card_url")
		if link == "" {
			link = "https://github.com/suyijun8182/easysub"
		}
		return map[string]any{"msgtype": "textcard", "textcard": map[string]any{
			"title":       truncBytes(title, 128),
			"description": truncBytes(text, 512),
			"url":         link,
			"btntxt":      "查看详情",
		}}
	}
	return map[string]any{"msgtype": "text", "text": map[string]any{"content": truncBytes(body, 2048)}}
}

// sendWecomRobot 群机器人 Webhook 模式。
func sendWecomRobot(ctx context.Context, conf map[string]any, subject, text string) error {
	u := strings.TrimSpace(str(conf, "url"))
	if u == "" {
		return errors.New("企业微信未配置群机器人 Webhook")
	}
	payload := wecomPayload(conf, subject, text)
	if payload["msgtype"] == "textcard" { // 群机器人不支持 textcard，降级为 text
		payload = map[string]any{"msgtype": "text", "text": map[string]any{"content": truncBytes(withSubject(subject, text), 2048)}}
	}
	r, err := postJSON(ctx, defaultClient(), u, nil, nil, payload)
	if err != nil {
		return err
	}
	data, err := r.checkedJSON()
	if err != nil {
		return err
	}
	if jsonCode(data, "errcode", 0) != 0 {
		return fmt.Errorf("企业微信群机器人发送失败：%s %s", pyVal(data["errcode"]), pyVal(data["errmsg"]))
	}
	return nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// sendWecomApp 自建应用模式（企业ID + 应用ID + Secret）。
func sendWecomApp(ctx context.Context, conf map[string]any, subject, text string) error {
	agentID := strings.TrimSpace(str(conf, "agent_id"))
	if agentID == "" {
		return errors.New("企业微信自建应用未配置应用ID（AgentId）")
	}
	toUser := strings.TrimSpace(str(conf, "to_user"))
	toParty := strings.TrimSpace(str(conf, "to_party"))
	toTag := strings.TrimSpace(str(conf, "to_tag"))
	if toUser == "" && toParty == "" && toTag == "" {
		toUser = "@all" // 三者全空则发给应用可见范围内全部成员
	}
	payload := wecomPayload(conf, subject, text)
	payload["touser"], payload["toparty"], payload["totag"] = toUser, toParty, toTag
	if isDigits(agentID) {
		n, _ := strconv.Atoi(agentID)
		payload["agentid"] = n
	} else {
		payload["agentid"] = agentID
	}
	payload["safe"], payload["enable_duplicate_check"] = 0, 0
	base := wecomAPIBase(conf)
	for attempt := 0; attempt < 2; attempt++ { // token 失效时强制刷新重试一次
		token, err := wecomGetToken(ctx, conf, attempt > 0)
		if err != nil {
			return err
		}
		r, err := postJSON(ctx, defaultClient(), base+"/cgi-bin/message/send", url.Values{"access_token": {token}}, nil, payload)
		if err != nil {
			return err
		}
		data, err := r.checkedJSON()
		if err != nil {
			return err
		}
		code := jsonCode(data, "errcode", 0)
		if wecomTokenErr(code) && attempt == 0 {
			continue
		}
		if code != 0 {
			hint := ""
			switch code {
			case 81013:
				hint = "（接收人不在应用可见范围内，请到「应用管理→可见范围」添加成员）"
			case 60020, 301014:
				hint = "（服务器 IP 未在「企业可信IP」白名单中，或需要配置 API 代理）"
			case 40056:
				hint = "（应用ID AgentId 不正确）"
			}
			return fmt.Errorf("企业微信发送失败：%s %s%s", pyVal(data["errcode"]), pyVal(data["errmsg"]), hint)
		}
		// errcode=0 但接收人被剔除时，消息实际没人收到，需要报错而不是假装成功
		var invalid []string
		for _, k := range []string{"invaliduser", "invalidparty", "invalidtag"} {
			if v := strings.Trim(toStr(data[k]), "| "); v != "" {
				invalid = append(invalid, v)
			}
		}
		if len(invalid) > 0 {
			return fmt.Errorf("企业微信部分接收人无效：%s（请核对成员 UserID，并确认其在应用可见范围内）",
				strings.Join(invalid, " / "))
		}
		return nil
	}
	return nil
}

func sendWecom(ctx context.Context, conf map[string]any, subject, text string) error {
	if str(conf, "mode") == "app" {
		return sendWecomApp(ctx, conf, subject, text)
	}
	return sendWecomRobot(ctx, conf, subject, text)
}
