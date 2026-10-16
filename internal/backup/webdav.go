package backup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/studio-b12/gowebdav"
	"gorm.io/gorm"

	"easysub/internal/config"
	"easysub/internal/secure"
	"easysub/internal/store"
)

// WebDAVConfig 是用户的 WebDAV 备份设置（存于 users.webdav_config，密码加密）。
type WebDAVConfig struct {
	URL         string `json:"url"`
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"`
	Path        string `json:"path"`
	AutoEnabled bool   `json:"auto_enabled"`
	Keep        int    `json:"keep"`
	Scope       string `json:"scope"` // self | all（仅管理员）
}

// WebDAVStatus 记录最近一次备份结果。
type WebDAVStatus struct {
	At      time.Time `json:"at"`
	File    string    `json:"file"`
	OK      bool      `json:"ok"`
	Message string    `json:"message"`
}

// RemoteFile 远程备份文件信息。
type RemoteFile struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

var fileNameRe = regexp.MustCompile(`^easysub-[A-Za-z0-9_.\-]+\.json$`)

func (c *WebDAVConfig) normalize() {
	c.URL = strings.TrimSpace(c.URL)
	c.Path = strings.Trim(strings.TrimSpace(c.Path), "/")
	if c.Path == "" {
		c.Path = "easysub"
	}
	if c.Keep <= 0 {
		c.Keep = 14
	}
	if c.Keep > 365 {
		c.Keep = 365
	}
	if c.Scope != "all" {
		c.Scope = "self"
	}
}

// LoadWebDAV 读取用户 WebDAV 配置（密码已解密）与最近状态。
func LoadWebDAV(u *store.User) (WebDAVConfig, *WebDAVStatus) {
	var c WebDAVConfig
	var st *WebDAVStatus
	if u.WebdavConfig.Valid {
		raw, _ := json.Marshal(u.WebdavConfig.V)
		_ = json.Unmarshal(raw, &c)
		if last, ok := u.WebdavConfig.V["last"]; ok {
			b, _ := json.Marshal(last)
			st = &WebDAVStatus{}
			if json.Unmarshal(b, st) != nil {
				st = nil
			}
		}
	}
	c.Password = secure.Decrypt(c.Password)
	c.normalize()
	return c, st
}

func storeWebDAV(u *store.User, c WebDAVConfig, st *WebDAVStatus) {
	c.normalize()
	c.Password = secure.Encrypt(c.Password)
	raw, _ := json.Marshal(c)
	m := map[string]any{}
	_ = json.Unmarshal(raw, &m)
	if st != nil {
		m["last"] = st
	} else if u.WebdavConfig.Valid {
		if last, ok := u.WebdavConfig.V["last"]; ok {
			m["last"] = last
		}
	}
	u.WebdavConfig = store.JSONOf(m)
}

// SaveWebDAV 保存配置；incoming.Password 为空时保留原密码。
func SaveWebDAV(db *gorm.DB, u *store.User, incoming WebDAVConfig) (WebDAVConfig, error) {
	old, _ := LoadWebDAV(u)
	if incoming.Password == "" {
		incoming.Password = old.Password
	}
	if !u.IsAdmin {
		incoming.Scope = "self"
	}
	if incoming.URL != "" && !strings.HasPrefix(incoming.URL, "http://") && !strings.HasPrefix(incoming.URL, "https://") {
		return incoming, errors.New("WebDAV 地址需以 http:// 或 https:// 开头")
	}
	storeWebDAV(u, incoming, nil)
	if err := db.Model(u).Update("webdav_config", u.WebdavConfig).Error; err != nil {
		return incoming, err
	}
	c, _ := LoadWebDAV(u)
	return c, nil
}

func newClient(c WebDAVConfig) (*gowebdav.Client, error) {
	if c.URL == "" {
		return nil, errors.New("尚未配置 WebDAV 地址")
	}
	cl := gowebdav.NewClient(c.URL, c.Username, c.Password)
	cl.SetTimeout(60 * time.Second)
	cl.SetHeader("User-Agent", "EasySub")
	return cl, nil
}

func friendly(err error) error {
	if err == nil {
		return nil
	}
	var se gowebdav.StatusError
	if errors.As(err, &se) {
		switch se.Status {
		case http.StatusUnauthorized:
			return errors.New("WebDAV 认证失败：请检查用户名 / 密码（坚果云等需使用「应用密码」）")
		case http.StatusForbidden:
			return errors.New("WebDAV 拒绝访问（403）：请检查目录权限")
		case http.StatusNotFound:
			return errors.New("WebDAV 路径不存在（404）")
		}
		return fmt.Errorf("WebDAV 返回 HTTP %d", se.Status)
	}
	return fmt.Errorf("WebDAV 请求失败：%v", err)
}

// TestWebDAV 测试连接：确保目录存在并可写。
func TestWebDAV(c WebDAVConfig) (string, error) {
	c.normalize()
	cl, err := newClient(c)
	if err != nil {
		return "", err
	}
	if err := cl.MkdirAll("/"+c.Path, 0o755); err != nil {
		return "", friendly(err)
	}
	probe := "/" + c.Path + "/.easysub-probe"
	if err := cl.Write(probe, []byte("ok"), 0o644); err != nil {
		return "", friendly(err)
	}
	_ = cl.Remove(probe)
	files, _ := ListRemote(c)
	return fmt.Sprintf("连接成功，目录 /%s 可写（已有 %d 个备份）", c.Path, len(files)), nil
}

// ListRemote 列出远程备份文件（新到旧）。
func ListRemote(c WebDAVConfig) ([]RemoteFile, error) {
	cl, err := newClient(c)
	if err != nil {
		return nil, err
	}
	infos, err := cl.ReadDir("/" + c.Path)
	if err != nil {
		var se gowebdav.StatusError
		if errors.As(err, &se) && se.Status == http.StatusNotFound {
			return []RemoteFile{}, nil
		}
		return nil, friendly(err)
	}
	out := make([]RemoteFile, 0, len(infos))
	for _, fi := range infos {
		if fi.IsDir() || !fileNameRe.MatchString(fi.Name()) {
			continue
		}
		out = append(out, RemoteFile{Name: fi.Name(), Size: fi.Size(), Modified: fi.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out, nil
}

// RunWebDAVBackup 立即备份到 WebDAV 并清理旧文件，同时记录状态。
func RunWebDAVBackup(db *gorm.DB, u *store.User) (RemoteFile, error) {
	c, _ := LoadWebDAV(u)
	file, err := doBackup(db, u, c)
	st := &WebDAVStatus{At: time.Now(), File: file.Name, OK: err == nil}
	if err != nil {
		st.Message = err.Error()
	} else {
		st.Message = fmt.Sprintf("已上传 %s（%d 字节）", file.Name, file.Size)
	}
	storeWebDAV(u, c, st)
	db.Model(u).Update("webdav_config", u.WebdavConfig)
	level := "info"
	if err != nil {
		level = "error"
	}
	store.Log("backup.webdav", "WebDAV 备份："+st.Message, u, level)
	return file, err
}

func doBackup(db *gorm.DB, u *store.User, c WebDAVConfig) (RemoteFile, error) {
	cl, err := newClient(c)
	if err != nil {
		return RemoteFile{}, err
	}
	var payload map[string]any
	scope := "user"
	if c.Scope == "all" && u.IsAdmin {
		payload, scope = ExportAll(db), "full"
	} else {
		payload = ExportUser(db, u)
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return RemoteFile{}, err
	}
	name := fmt.Sprintf("easysub-%s-%s-%s.json", scope, sanitize(u.Username), config.Now().Format("20060102-150405"))
	if err := cl.MkdirAll("/"+c.Path, 0o755); err != nil {
		return RemoteFile{}, friendly(err)
	}
	if err := cl.WriteStream("/"+path.Join(c.Path, name), bytes.NewReader(data), 0o644); err != nil {
		return RemoteFile{}, friendly(err)
	}
	prune(cl, c, scope, u.Username)
	return RemoteFile{Name: name, Size: int64(len(data)), Modified: time.Now()}, nil
}

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9_.\-]`)

func sanitize(s string) string {
	s = unsafeChars.ReplaceAllString(s, "_")
	if s == "" {
		return "user"
	}
	return s
}

// prune 只清理同类型（同 scope + 同用户）的旧备份，保留最近 Keep 份。
func prune(cl *gowebdav.Client, c WebDAVConfig, scope, username string) {
	files, err := ListRemote(c)
	if err != nil {
		return
	}
	prefix := fmt.Sprintf("easysub-%s-%s-", scope, sanitize(username))
	kept := 0
	for _, f := range files {
		if !strings.HasPrefix(f.Name, prefix) {
			continue
		}
		kept++
		if kept > c.Keep {
			_ = cl.Remove("/" + path.Join(c.Path, f.Name))
		}
	}
}

// ReadRemote 下载并解析远程备份文件。
func ReadRemote(c WebDAVConfig, name string) (map[string]any, error) {
	if !fileNameRe.MatchString(name) {
		return nil, errors.New("非法的备份文件名")
	}
	cl, err := newClient(c)
	if err != nil {
		return nil, err
	}
	rc, err := cl.ReadStream("/" + path.Join(c.Path, name))
	if err != nil {
		return nil, friendly(err)
	}
	defer rc.Close()
	raw, err := io.ReadAll(io.LimitReader(rc, 64<<20))
	if err != nil {
		return nil, friendly(err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("%w：不是有效的 JSON", ErrFormat)
	}
	return data, nil
}

// DeleteRemote 删除远程备份文件。
func DeleteRemote(c WebDAVConfig, name string) error {
	if !fileNameRe.MatchString(name) {
		return errors.New("非法的备份文件名")
	}
	cl, err := newClient(c)
	if err != nil {
		return err
	}
	return friendly(cl.Remove("/" + path.Join(c.Path, name)))
}

// RunAutoWebDAV 定时任务：为所有开启自动备份的用户执行一次 WebDAV 备份。
func RunAutoWebDAV(db *gorm.DB) (ok, failed int) {
	var users []store.User
	db.Where("is_active = ? AND webdav_config IS NOT NULL", true).Find(&users)
	for i := range users {
		c, _ := LoadWebDAV(&users[i])
		if !c.AutoEnabled || c.URL == "" {
			continue
		}
		if _, err := RunWebDAVBackup(db, &users[i]); err != nil {
			failed++
		} else {
			ok++
		}
	}
	return
}
