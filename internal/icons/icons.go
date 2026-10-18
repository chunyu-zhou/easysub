// Package icons 提供商家图标库与图标下载。
//
// 修复原版的线程池问题：原版在请求处理中同步抓取外部 favicon（超时 12s），外网不可达时
// 每个图标都占住一个后端工作线程，页面一直「加载中」。现在：
//   - 已缓存的图标作为静态文件直接返回；
//   - 未缓存时立即 302 跳转到图标源，由浏览器直接加载（不占用后端），
//     同时把下载任务投递给有界的后台队列（固定 worker 数、短超时、去重、失败冷却），
//     队列满则直接丢弃，绝不阻塞请求。
package icons

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"easysub/internal/config"
)

func slugOf(domain string) string {
	return strings.NewReplacer(".", "_", "/", "_").Replace(domain)
}

// ExtraDomain 解析内置清单之外（用户自定义服务）的 slug；由 server 注入。
var ExtraDomain func(slug string) string

func domainForSlug(slug string) string {
	slug = strings.TrimSuffix(slug, ".png")
	for _, s := range services {
		if slugOf(s.Domain) == slug {
			return s.Domain
		}
	}
	if ExtraDomain != nil {
		return ExtraDomain(slug)
	}
	return ""
}

var safeSlug = regexp.MustCompile(`[^A-Za-z0-9_.\-]`)

// Library 管理图标库缓存与后台抓取队列。
type Library struct {
	dir      string
	provider string
	client   *http.Client
	queue    chan string

	mu       sync.Mutex
	inflight map[string]bool
	failedAt map[string]time.Time
}

const failCooldown = 30 * time.Minute

// NewLibrary 创建图标库并启动固定数量的后台 worker。
func NewLibrary(ctx context.Context) *Library {
	l := &Library{
		dir:      config.C.Path("icons", "library"),
		provider: config.C.IconProvider,
		client:   &http.Client{Timeout: config.C.IconFetchTimeout},
		queue:    make(chan string, 256),
		inflight: map[string]bool{},
		failedAt: map[string]time.Time{},
	}
	_ = os.MkdirAll(l.dir, 0o755)
	workers := config.C.IconWorkers
	if workers < 1 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		go l.worker(ctx)
	}
	return l
}

func (l *Library) sourceURL(domain string) string {
	return strings.ReplaceAll(l.provider, "{domain}", domain)
}

// ServeHTTP 处理 GET /api/icons/library/{slug}。
func (l *Library) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug := safeSlug.ReplaceAllString(r.PathValue("slug"), "")
	if !strings.HasSuffix(slug, ".png") {
		slug += ".png"
	}
	path := filepath.Join(l.dir, slug)
	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		w.Header().Set("Cache-Control", "public, max-age=604800")
		http.ServeFile(w, r, path)
		return
	}
	domain := domainForSlug(slug)
	if domain == "" {
		http.Error(w, `{"detail":"未知图标"}`, http.StatusNotFound)
		return
	}
	l.enqueue(slug)
	// 浏览器直接去图标源加载；短缓存，等后台缓存完成后下次即走本地文件
	w.Header().Set("Cache-Control", "public, max-age=300")
	http.Redirect(w, r, l.sourceURL(domain), http.StatusFound)
}

// enqueue 非阻塞投递；已在下载、近期失败或队列已满时直接跳过。
func (l *Library) enqueue(slug string) {
	l.mu.Lock()
	if l.inflight[slug] || time.Since(l.failedAt[slug]) < failCooldown {
		l.mu.Unlock()
		return
	}
	l.inflight[slug] = true
	l.mu.Unlock()
	select {
	case l.queue <- slug:
	default:
		l.done(slug, false)
	}
}

func (l *Library) done(slug string, ok bool) {
	l.mu.Lock()
	delete(l.inflight, slug)
	if ok {
		delete(l.failedAt, slug)
	} else {
		l.failedAt[slug] = time.Now()
	}
	l.mu.Unlock()
}

func (l *Library) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case slug := <-l.queue:
			err := l.fetch(ctx, slug)
			if err != nil {
				log.Printf("[icons] 缓存 %s 失败：%v", slug, err)
			}
			l.done(slug, err == nil)
		}
	}
}

func (l *Library) fetch(ctx context.Context, slug string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.sourceURL(domainForSlug(slug)), nil)
	if err != nil {
		return err
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &statusErr{resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes))
	if err != nil || len(data) == 0 {
		return err
	}
	tmp := filepath.Join(l.dir, slug+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(l.dir, slug))
}

type statusErr struct{ code int }

func (e *statusErr) Error() string { return "HTTP " + http.StatusText(e.code) }
