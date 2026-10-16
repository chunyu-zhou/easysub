// Package server 实现 HTTP API（路径与原 Python 版一致）、静态资源与前端托管。
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"easysub/internal/config"
	"easysub/internal/exchange"
	"easysub/internal/icons"
	"easysub/internal/secure"
	"easysub/internal/store"
)

const Version = "2.0.0"

// HTTPError 携带状态码的错误，序列化为 {"detail": "..."}。
type HTTPError struct {
	Code    int
	Detail  string
	Headers map[string]string
}

func (e *HTTPError) Error() string { return e.Detail }

func Err(code int, format string, args ...any) *HTTPError {
	return &HTTPError{Code: code, Detail: fmt.Sprintf(format, args...)}
}

// Ctx 是每个请求的上下文。
type Ctx struct {
	W    http.ResponseWriter
	R    *http.Request
	DB   *gorm.DB
	User *store.User

	conv     *exchange.Converter
	convOnce sync.Once
}

func (c *Ctx) Context() context.Context { return c.R.Context() }

// Conv 返回当前用户的换算器（每请求加载一次汇率）。
func (c *Ctx) Conv() *exchange.Converter {
	c.convOnce.Do(func() {
		uid := 0
		if c.User != nil {
			uid = c.User.ID
		}
		c.conv = exchange.NewConverter(c.DB, uid)
	})
	return c.conv
}

// Bind 解析 JSON 请求体。
func (c *Ctx) Bind(v any) error {
	body, err := io.ReadAll(io.LimitReader(c.R.Body, 64<<20))
	if err != nil {
		return Err(400, "读取请求失败：%v", err)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, v); err != nil {
		return Err(422, "请求格式不正确：%v", err)
	}
	return nil
}

// BindFields 解析为原始字段表，用于「只更新提交了的字段」的 PATCH/PUT。
func (c *Ctx) BindFields() (map[string]json.RawMessage, error) {
	m := map[string]json.RawMessage{}
	return m, c.Bind(&m)
}

// Apply 把提交的字段写入 allowed 中对应的指针。
func Apply(raw map[string]json.RawMessage, allowed map[string]any) error {
	for k, v := range raw {
		ptr, ok := allowed[k]
		if !ok {
			continue
		}
		if err := json.Unmarshal(v, ptr); err != nil {
			return Err(422, "字段 %s 格式不正确：%v", k, err)
		}
	}
	return nil
}

func (c *Ctx) PathInt(name string) (int, error) {
	n, err := strconv.Atoi(c.R.PathValue(name))
	if err != nil {
		return 0, Err(422, "路径参数 %s 不正确", name)
	}
	return n, nil
}

func (c *Ctx) QueryInt(name string, def int) int {
	if n, err := strconv.Atoi(c.R.URL.Query().Get(name)); err == nil {
		return n
	}
	return def
}

func (c *Ctx) QueryBool(name string) (bool, bool) {
	v := strings.ToLower(c.R.URL.Query().Get(name))
	switch v {
	case "1", "true", "yes":
		return true, true
	case "0", "false", "no":
		return false, true
	}
	return false, false
}

// Raw 表示处理函数已自行写出响应。
type Raw struct{}

type handlerFunc func(c *Ctx) (any, error)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	var he *HTTPError
	if errors.As(err, &he) {
		for k, v := range he.Headers {
			w.Header().Set(k, v)
		}
		writeJSON(w, he.Code, map[string]string{"detail": he.Detail})
		return
	}
	msg := strings.SplitN(err.Error(), "\n", 2)[0]
	if r := []rune(msg); len(r) > 300 {
		msg = string(r[:300])
	}
	writeJSON(w, 500, map[string]string{"detail": "服务器内部错误：" + msg})
}

type access int

const (
	public access = iota // 无需数据库
	withDB               // 需要数据库，不需要登录
	authed               // 需要登录
	admin                // 需要管理员
)

// Server 聚合路由与依赖。
type Server struct {
	mux   *http.ServeMux
	icons *icons.Library
	web   fs.FS
}

// New 创建 HTTP 服务；web 为前端构建产物（可为 nil）。
func New(ctx context.Context, web fs.FS) *Server {
	// Embedded SQLite needs no remote service or wizard credentials. Start opening,
	// migrating, and seeding it as soon as the HTTP server is constructed; requests
	// retain the existing 503 response only if the local filesystem is unavailable.
	store.ConnectSaved(true)

	s := &Server{mux: http.NewServeMux(), icons: icons.NewLibrary(ctx), web: web}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return s.cors(s.mux) }

func (s *Server) handle(pattern string, lvl access, fn handlerFunc) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[panic] %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
				writeErr(w, fmt.Errorf("%v", rec))
			}
		}()
		c := &Ctx{W: w, R: r}
		if lvl >= withDB {
			c.DB = store.DB()
			if c.DB == nil {
				writeErr(w, Err(503, "数据库尚未配置或暂时不可用，请稍后重试；若为首次安装请在安装向导中配置"))
				return
			}
			c.DB = c.DB.WithContext(r.Context())
		}
		if lvl >= authed {
			u, err := authenticate(c)
			if err != nil {
				writeErr(w, err)
				return
			}
			if lvl == admin && !u.IsAdmin {
				writeErr(w, Err(403, "需要管理员权限"))
				return
			}
			c.User = u
		}
		out, err := fn(c)
		if err != nil {
			if !errors.As(err, new(*HTTPError)) {
				log.Printf("[error] %s %s: %v", r.Method, r.URL.Path, err)
			}
			writeErr(w, err)
			return
		}
		if _, ok := out.(Raw); ok {
			return
		}
		writeJSON(w, 200, out)
	})
}

// BearerToken 从 Authorization 头读取令牌。
func BearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// Authenticate 校验 JWT 访问令牌或 API Token，返回用户。供 MCP 等复用。
func Authenticate(db *gorm.DB, token string) (*store.User, error) {
	unauth := &HTTPError{Code: 401, Detail: "无效或过期的凭证", Headers: map[string]string{"WWW-Authenticate": "Bearer"}}
	if token == "" {
		return nil, &HTTPError{Code: 401, Detail: "未登录", Headers: unauth.Headers}
	}
	uid := secure.DecodeToken(token, "access")
	if uid == 0 {
		var row store.ApiToken
		if err := db.Where("token = ?", token).First(&row).Error; err == nil {
			now := time.Now()
			db.Model(&row).Update("last_used_at", now)
			uid = row.UserID
		}
	}
	if uid == 0 {
		return nil, unauth
	}
	var u store.User
	if err := db.First(&u, uid).Error; err != nil {
		return nil, Err(401, "用户不存在")
	}
	if !u.IsActive {
		return nil, Err(403, "账户已被禁用")
	}
	store.EnsureUserDefaults(db, &u)
	return &u, nil
}

func authenticate(c *Ctx) (*store.User, error) {
	return Authenticate(c.DB, BearerToken(c.R))
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Expose-Headers", "X-2FA-Required, Content-Disposition, Mcp-Session-Id")
		}
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			if h := r.Header.Get("Access-Control-Request-Headers"); h != "" {
				w.Header().Set("Access-Control-Allow-Headers", h)
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// staticDir 静态目录直出（带缓存头），不经过任何业务逻辑。
func staticDir(dir string) http.Handler {
	fsrv := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=2592000, immutable")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if strings.HasSuffix(strings.ToLower(r.URL.Path), ".svg") {
			// 用户上传的 SVG 可能含脚本：禁止其作为文档执行
			w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
		}
		fsrv.ServeHTTP(w, r)
	})
}

// spa 托管前端构建产物：存在的文件直接返回，其余回退 index.html（history 路由）。
func (s *Server) spa() http.Handler {
	if s.web == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend not built", http.StatusNotFound)
		})
	}
	fsrv := http.FileServer(http.FS(s.web))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p != "" {
			if st, err := fs.Stat(s.web, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				fsrv.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(s.web, "index.html")
		if err != nil {
			http.Error(w, "frontend not built", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}

func ensureDir(dir string) string {
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func init() {
	// 保证 config 包在测试等场景下未加载时也有默认值
	if config.C == nil {
		config.Load()
	}
}
