package notify

import (
	"context"
	"errors"
	"strings"
)

// TelegramDefaultAPIBase 官方 Bot API 地址；可通过 api_base 指定反代。
const TelegramDefaultAPIBase = "https://api.telegram.org"

func tgURL(apiBase, token, method string) string {
	if apiBase == "" {
		apiBase = TelegramDefaultAPIBase
	}
	return strings.TrimRight(apiBase, "/") + "/bot" + token + "/" + method
}

func tgCall(ctx context.Context, token, apiBase, proxy, method string, payload any) (map[string]any, error) {
	if token == "" {
		return nil, errors.New("未配置 Bot Token")
	}
	c, err := newClient(defaultTimeout, proxy)
	if err != nil {
		return nil, err
	}
	var r *response
	if payload == nil {
		r, err = get(ctx, c, tgURL(apiBase, token, method), nil)
	} else {
		r, err = postJSON(ctx, c, tgURL(apiBase, token, method), nil, nil, payload)
	}
	if err != nil {
		return nil, err
	}
	return r.checkedJSON()
}

// TelegramGetMe 调用 getMe 校验 Bot Token。
func TelegramGetMe(ctx context.Context, token, apiBase, proxy string) (map[string]any, error) {
	return tgCall(ctx, token, apiBase, proxy, "getMe", nil)
}

// TelegramGetUpdates 辅助绑定：用户向 Bot 发消息后，从这里读取 chat_id。
func TelegramGetUpdates(ctx context.Context, token, apiBase, proxy string) (map[string]any, error) {
	return tgCall(ctx, token, apiBase, proxy, "getUpdates", nil)
}

// TelegramSend 以 Markdown 模式发送消息。
func TelegramSend(ctx context.Context, chatID, text, token, apiBase, proxy string) error {
	_, err := tgCall(ctx, token, apiBase, proxy, "sendMessage",
		map[string]any{"chat_id": chatID, "text": text, "parse_mode": "Markdown"})
	return err
}
