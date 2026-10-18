package server

import (
	"strings"

	"easysub/internal/icons"
	"easysub/internal/store"
)

// 服务库（ChatGPT、Netflix 等）：每个用户一份，可增删改；订阅表单的联想与浏览也读取这里。

func (s *Server) listServices(c *Ctx) (any, error) {
	icons.EnsureUserServices(c.DB, c.User)
	var rows []store.UserService
	if err := c.DB.Where("user_id = ?", c.User.ID).Order("sort, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]icons.Entry, 0, len(rows))
	for _, r := range rows {
		out = append(out, icons.ToEntry(r))
	}
	return out, nil
}

func (s *Server) serviceCategories(*Ctx) (any, error) { return icons.Categories(), nil }

type serviceInput struct {
	Name     string  `json:"name"`
	Website  string  `json:"website"`
	Category string  `json:"category"`
	Icon     *string `json:"icon"`
}

// apply 校验输入并写入 row。
func (in serviceInput) apply(row *store.UserService) error {
	name := strings.TrimSpace(in.Name)
	if name == "" || len([]rune(name)) > 128 {
		return Err(422, "服务名称不能为空，且不超过 128 个字符")
	}
	website := strings.TrimSpace(in.Website)
	domain := store.NormalizeDomain(website)
	if website != "" && domain == "" {
		return Err(422, "官网地址格式不正确")
	}
	if website != "" && !strings.Contains(website, "://") {
		website = "https://" + website
	}
	if len(website) > 512 {
		return Err(422, "官网地址过长")
	}
	row.Name, row.Website, row.Domain = name, website, domain
	row.Category = icons.ValidCategory(strings.TrimSpace(in.Category))
	row.Icon = ptrString(in.Icon, 512)
	return nil
}

func (s *Server) duplicateService(c *Ctx, name string, exceptID int) bool {
	var n int64
	c.DB.Model(&store.UserService{}).Where("user_id = ? AND LOWER(name) = ? AND id <> ?", c.User.ID, strings.ToLower(name), exceptID).Count(&n)
	return n > 0
}

func (s *Server) createService(c *Ctx) (any, error) {
	icons.EnsureUserServices(c.DB, c.User)
	var in serviceInput
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	row := store.UserService{UserID: c.User.ID}
	if err := in.apply(&row); err != nil {
		return nil, err
	}
	if s.duplicateService(c, row.Name, 0) {
		return nil, Err(409, "已存在同名服务")
	}
	c.DB.Model(&store.UserService{}).Where("user_id = ?", c.User.ID).Select("COALESCE(MAX(sort), -1) + 1").Scan(&row.Sort)
	if err := c.DB.Create(&row).Error; err != nil {
		return nil, err
	}
	return icons.ToEntry(row), nil
}

func (s *Server) updateService(c *Ctx) (any, error) {
	id, err := c.PathInt("id")
	if err != nil {
		return nil, err
	}
	var row store.UserService
	if err := c.DB.Where("id = ? AND user_id = ?", id, c.User.ID).First(&row).Error; err != nil {
		return nil, Err(404, "服务不存在")
	}
	var in serviceInput
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	if err := in.apply(&row); err != nil {
		return nil, err
	}
	if s.duplicateService(c, row.Name, row.ID) {
		return nil, Err(409, "已存在同名服务")
	}
	if err := c.DB.Save(&row).Error; err != nil {
		return nil, err
	}
	return icons.ToEntry(row), nil
}

func (s *Server) deleteService(c *Ctx) (any, error) {
	id, err := c.PathInt("id")
	if err != nil {
		return nil, err
	}
	res := c.DB.Where("id = ? AND user_id = ?", id, c.User.ID).Delete(&store.UserService{})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, Err(404, "服务不存在")
	}
	return map[string]any{"status": "ok"}, nil
}

func (s *Server) restoreServices(c *Ctx) (any, error) {
	icons.EnsureUserServices(c.DB, c.User)
	added, err := icons.AddMissingBuiltins(c.DB, c.User.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"added": added}, nil
}

// serviceDomainForSlug 让图标库为用户自定义服务的域名取 favicon（仅限已登记的域名）。
func serviceDomainForSlug(slug string) string {
	db := store.DB()
	if db == nil || slug == "" {
		return ""
	}
	var domains []string
	db.Model(&store.UserService{}).
		Where("domain <> '' AND REPLACE(REPLACE(domain, '.', '_'), '/', '_') = ?", slug).
		Limit(1).Pluck("domain", &domains)
	if len(domains) == 0 {
		return ""
	}
	return domains[0]
}
