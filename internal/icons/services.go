package icons

import (
	"log"
	"strings"

	"gorm.io/gorm"

	"easysub/internal/store"
)

type service struct{ Name, Domain, Category string }

// Entry 是服务库中的一项（订阅表单联想 / 按分类浏览 / 设置页管理共用）。
type Entry struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Domain        string `json:"domain"`
	Website       string `json:"website"`
	Category      string `json:"category"`
	CategoryLabel string `json:"category_label"`
	Slug          string `json:"slug"`
	Icon          string `json:"icon"`        // 实际显示用图标
	CustomIcon    string `json:"custom_icon"` // 用户填写的图标；为空表示使用 favicon
	Sort          int    `json:"sort"`
}

// Category 是服务分类。
type Category struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// OtherCategory 是未归类服务使用的分类 key。
const OtherCategory = "other"

// Categories 返回全部服务分类（含「其它」）。
func Categories() []Category {
	out := make([]Category, 0, len(categoryLabels)+1)
	for _, l := range categoryLabels {
		out = append(out, Category{Key: l[0], Label: l[1]})
	}
	return append(out, Category{Key: OtherCategory, Label: "其它 / Other"})
}

// ValidCategory 把未知分类归为「其它」。
func ValidCategory(key string) string {
	for _, l := range categoryLabels {
		if l[0] == key {
			return key
		}
	}
	return OtherCategory
}

func categoryLabel(key string) string {
	for _, c := range Categories() {
		if c.Key == key {
			return c.Label
		}
	}
	return key
}

// ToEntry 把用户服务转换为清单项。
func ToEntry(s store.UserService) Entry {
	e := Entry{ID: s.ID, Name: s.Name, Domain: s.Domain, Website: s.Website, Category: s.Category,
		CategoryLabel: categoryLabel(s.Category), Sort: s.Sort}
	if s.Domain != "" {
		e.Slug = slugOf(s.Domain)
	} else {
		e.Slug = "svc-" + strings.ToLower(strings.ReplaceAll(s.Name, " ", "-"))
	}
	if s.Icon != nil && strings.TrimSpace(*s.Icon) != "" {
		e.CustomIcon = strings.TrimSpace(*s.Icon)
		e.Icon = e.CustomIcon
	} else if s.Domain != "" {
		e.Icon = "/api/icons/library/" + e.Slug + ".png"
	}
	return e
}

// EnsureUserServices 首次使用时把内置服务复制到用户名下（按名称去重，可重复调用）。
func EnsureUserServices(db *gorm.DB, u *store.User) {
	if u.ServicesSeeded {
		return
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if _, err := AddMissingBuiltins(tx, u.ID); err != nil {
			return err
		}
		u.ServicesSeeded = true
		return tx.Model(u).Update("services_seeded", true).Error
	})
	if err != nil {
		log.Printf("[seed] 用户 %d 服务库初始化失败：%v", u.ID, err)
	}
}

// AddMissingBuiltins 补回用户缺少的内置服务，返回新增数量。
func AddMissingBuiltins(tx *gorm.DB, userID int) (int, error) {
	var have []store.UserService
	if err := tx.Where("user_id = ?", userID).Find(&have).Error; err != nil {
		return 0, err
	}
	names := map[string]bool{}
	sort := -1
	for _, s := range have {
		names[strings.ToLower(s.Name)] = true
		if s.Sort > sort {
			sort = s.Sort
		}
	}
	added := 0
	for _, s := range services {
		if names[strings.ToLower(s.Name)] {
			continue
		}
		sort++
		row := store.UserService{UserID: userID, Name: s.Name, Website: "https://" + s.Domain, Domain: s.Domain,
			Category: s.Category, Sort: sort}
		if err := tx.Create(&row).Error; err != nil {
			return added, err
		}
		added++
	}
	return added, nil
}
