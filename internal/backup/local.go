package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gorm.io/gorm"

	"easysub/internal/config"
)

// LocalKeep 本地自动备份保留份数。
const LocalKeep = 14

func LocalDir() string { return config.C.Path("backups") }

// RunLocalBackup 把整站数据导出到 data/backups/easysub-YYYYMMDD.json（同日覆盖）。
func RunLocalBackup(db *gorm.DB) (map[string]any, error) {
	if err := os.MkdirAll(LocalDir(), 0o755); err != nil {
		return nil, err
	}
	payload := ExportAll(db)
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("easysub-%s.json", time.Now().UTC().Format("20060102"))
	if err := os.WriteFile(filepath.Join(LocalDir(), name), data, 0o600); err != nil {
		return nil, err
	}
	files, _ := filepath.Glob(filepath.Join(LocalDir(), "easysub-*.json"))
	sort.Strings(files)
	for i := 0; i < len(files)-LocalKeep; i++ {
		_ = os.Remove(files[i])
	}
	users, _ := payload["users"].([]map[string]any)
	return map[string]any{"ok": true, "file": name, "users": len(users)}, nil
}

// ListLocal 列出本地自动备份。
func ListLocal() []map[string]any {
	files, _ := filepath.Glob(filepath.Join(LocalDir(), "easysub-*.json"))
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	out := make([]map[string]any, 0, len(files))
	for _, f := range files {
		st, err := os.Stat(f)
		if err != nil {
			continue
		}
		out = append(out, map[string]any{
			"name": filepath.Base(f), "size": st.Size(),
			"modified": st.ModTime().UTC().Format("2006-01-02T15:04:05"),
		})
	}
	return out
}
