package icons

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"easysub/internal/config"
	"easysub/internal/secure"
)

// MaxBytes 单个图标上限 2MB。
const MaxBytes = 2 << 20

var allowedExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".svg": true, ".webp": true, ".gif": true, ".ico": true}

var ctypeExt = map[string]string{
	"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp", "image/gif": ".gif",
	"image/svg+xml": ".svg", "image/x-icon": ".ico", "image/vnd.microsoft.icon": ".ico",
}

// UploadDir 用户上传图标目录（由 /static/icons/ 静态直出）。
func UploadDir() string { return config.C.Path("icons") }

// ErrBadIcon 图标格式或大小不符合要求。
var ErrBadIcon = errors.New("bad icon")

func save(userID int, ext string, data []byte) (string, error) {
	if err := os.MkdirAll(UploadDir(), 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%d_%s%s", userID, secure.RandomToken(12), ext)
	name = safeSlug.ReplaceAllString(name, "")
	if err := os.WriteFile(filepath.Join(UploadDir(), name), data, 0o644); err != nil {
		return "", err
	}
	return "/static/icons/" + name, nil
}

// SaveUpload 保存用户上传的图标，返回访问路径。
func SaveUpload(userID int, filename string, r io.Reader) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedExt[ext] {
		return "", fmt.Errorf("%w：不支持的图标格式：%s", ErrBadIcon, ext)
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > MaxBytes {
		return "", fmt.Errorf("%w：图标过大（上限 2MB）", ErrBadIcon)
	}
	return save(userID, ext, data)
}

var fetchClient = &http.Client{Timeout: 10 * time.Second}

// FromURL 下载远程图标并保存到本地（10 秒超时）。
func FromURL(ctx context.Context, userID int, rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return "", fmt.Errorf("%w：请输入 http(s) 图标地址", ErrBadIcon)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("%w：%v", ErrBadIcon, err)
	}
	resp, err := fetchClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载失败：%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("下载失败：HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
	if err != nil {
		return "", fmt.Errorf("下载失败：%v", err)
	}
	if len(data) > MaxBytes {
		return "", fmt.Errorf("%w：图标过大（上限 2MB）", ErrBadIcon)
	}
	mt, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	ext, ok := ctypeExt[mt]
	if !ok {
		ext = strings.ToLower(path.Ext(req.URL.Path))
	}
	if !allowedExt[ext] {
		ext = ".png"
	}
	return save(userID, ext, data)
}
