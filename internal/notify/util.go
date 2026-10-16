package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ---- 宽容的取值工具（配置来自 JSON，类型不定） ---- //

// truthy 近似 Python 的真值判断。
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case float64:
		return x != 0
	case float32:
		return x != 0
	case int:
		return x != 0
	case int64:
		return x != 0
	case json.Number:
		f, _ := x.Float64()
		return f != 0
	case []any:
		return len(x) > 0
	case []string:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	default:
		return true
	}
}

func toStr(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case bool:
		if x {
			return "True"
		}
		return "False"
	default:
		return fmt.Sprint(x)
	}
}

func str(m map[string]any, key string) string { return toStr(m[key]) }

func boolean(m map[string]any, key string) bool { return truthy(m[key]) }

func toNum(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	}
	return 0, false
}

// num 对应 Python 的 int(conf.get(key) or def)：值为假或无法解析时取默认值。
func num(m map[string]any, key string, def float64) float64 {
	v := m[key]
	if !truthy(v) {
		return def
	}
	if f, ok := toNum(v); ok {
		return f
	}
	return def
}

// strList 读取列表字段，过滤空值。
func strList(m map[string]any, key string) []string {
	var out []string
	switch x := m[key].(type) {
	case []any:
		for _, v := range x {
			if truthy(v) {
				out = append(out, toStr(v))
			}
		}
	case []string:
		for _, v := range x {
			if v != "" {
				out = append(out, v)
			}
		}
	case string:
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// pyVal 按 Python f-string 的风格格式化（None 显示为 None）。
func pyVal(v any) string {
	if v == nil {
		return "None"
	}
	return toStr(v)
}

// noCode 表示 JSON 中缺失或非数字的错误码。
const noCode = math.MinInt32

// jsonCode 读取 JSON 错误码；缺失时返回 def，非数字返回 noCode。
func jsonCode(data map[string]any, key string, def int) int {
	v, ok := data[key]
	if !ok {
		return def
	}
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return noCode
}

// ---- HTTP ---- //

const defaultTimeout = 15 * time.Second

func newClient(timeout time.Duration, proxy string) (*http.Client, error) {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		pu, err := url.Parse(proxy)
		if err != nil || pu.Host == "" {
			return nil, fmt.Errorf("代理地址无效：%s", proxy)
		}
		tr.Proxy = http.ProxyURL(pu)
	}
	return &http.Client{Timeout: timeout, Transport: tr}, nil
}

func defaultClient() *http.Client {
	c, _ := newClient(defaultTimeout, "")
	return c
}

type response struct {
	status int
	body   []byte
}

func (r *response) text() string { return string(r.body) }

// check 等价于 raise_for_status。
func (r *response) check() error {
	if r.status >= 400 {
		return fmt.Errorf("HTTP %d %s: %s", r.status, http.StatusText(r.status), truncRunes(r.text(), 300))
	}
	return nil
}

func (r *response) json() (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(r.body, &m); err != nil {
		return nil, fmt.Errorf("响应不是有效的 JSON（HTTP %d）：%s", r.status, truncRunes(r.text(), 300))
	}
	return m, nil
}

// checkedJSON 先检查状态码再解析 JSON。
func (r *response) checkedJSON() (map[string]any, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	return r.json()
}

// marshal 不转义 HTML 字符（与 Python json.dumps 一致）。
func marshal(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return bytes.TrimRight(buf.Bytes(), "\n")
}

type request struct {
	method  string
	url     string
	query   url.Values
	headers map[string]string
	body    []byte
	ctype   string
}

func do(ctx context.Context, c *http.Client, rq request) (*response, error) {
	u := rq.url
	if len(rq.query) > 0 {
		pu, err := url.Parse(u)
		if err != nil {
			return nil, fmt.Errorf("URL 无效：%w", err)
		}
		q := pu.Query()
		for k, vs := range rq.query {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		pu.RawQuery = q.Encode()
		u = pu.String()
	}
	var body io.Reader
	if rq.body != nil {
		body = bytes.NewReader(rq.body)
	}
	req, err := http.NewRequestWithContext(ctx, rq.method, u, body)
	if err != nil {
		return nil, fmt.Errorf("URL 无效：%s", redact(err))
	}
	if rq.ctype != "" {
		req.Header.Set("Content-Type", rq.ctype)
	}
	for k, v := range rq.headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, redact(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, redact(err)
	}
	return &response{status: resp.StatusCode, body: b}, nil
}

// redact 去掉 *url.Error 中的完整 URL，避免把 Token 等敏感参数带进错误信息。
func redact(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("%s 请求失败：%w", ue.Op, ue.Err)
	}
	return err
}

func postJSON(ctx context.Context, c *http.Client, u string, q url.Values, hdr map[string]string, payload any) (*response, error) {
	return do(ctx, c, request{method: http.MethodPost, url: u, query: q, headers: hdr,
		body: marshal(payload), ctype: "application/json"})
}

func get(ctx context.Context, c *http.Client, u string, q url.Values) (*response, error) {
	return do(ctx, c, request{method: http.MethodGet, url: u, query: q})
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
