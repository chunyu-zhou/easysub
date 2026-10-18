// Package backup 负责数据导出 / 导入（JSON，兼容原版 export_version=1 格式）与 WebDAV 远程备份。
package backup

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"easysub/internal/billing"
	"easysub/internal/config"
	"easysub/internal/icons"
	"easysub/internal/notify"
	"easysub/internal/store"
)

const ExportVersion = 1

// ErrFormat 表示备份文件结构不正确。
var ErrFormat = errors.New("备份文件格式不正确")

func subDict(s store.Subscription) map[string]any {
	return map[string]any{
		"name": s.Name, "plan": s.Plan, "icon": s.Icon, "url": s.URL, "notes": s.Notes,
		"remark": s.Remark, "ipv4": s.IPv4, "ipv6": s.IPv6,
		"category_id": s.CategoryID, "payment_method_id": s.PaymentMethodID, "payment_account_id": s.PaymentAccountID,
		"bundle_id": s.BundleID,
		"amount":    s.Amount, "currency": s.Currency, "billing_type": s.BillingType,
		"cycle": s.Cycle, "cycle_count": s.CycleCount,
		"start_date": s.StartDate.String(), "next_renewal_date": s.NextRenewalDate,
		"end_date": s.EndDate, "last_renewed_at": s.LastRenewedAt,
		"trial_end": s.TrialEnd, "cancel_by": s.CancelBy,
		"card_last4": s.CardLast4, "card_expiry": s.CardExpiry,
		"is_active": s.IsActive, "auto_renew": s.AutoRenew, "show_in_calendar": s.ShowInCalendar,
		"sort": s.Sort, "family_members": s.FamilyMembers, "remind_days_before": s.RemindDaysBefore,
	}
}

// collect 汇总某用户的订阅及依赖实体（含订阅引用到的系统分类）。
func collect(db *gorm.DB, u *store.User) map[string]any {
	var subs []store.Subscription
	db.Where("user_id = ?", u.ID).Order("id").Find(&subs)
	usedCats := []int{0}
	for _, s := range subs {
		if s.CategoryID != nil {
			usedCats = append(usedCats, *s.CategoryID)
		}
	}
	var cats []store.Category
	db.Where("user_id = ? OR id IN ?", u.ID, usedCats).Order("id").Find(&cats)
	var pms []store.PaymentMethod
	db.Where("user_id = ?", u.ID).Order("sort, id").Find(&pms)
	var bundles []store.Bundle
	db.Where("user_id = ?", u.ID).Order("id").Find(&bundles)
	var curs []store.UserCurrency
	db.Where("user_id = ?", u.ID).Order("sort, id").Find(&curs)

	catOut := make([]map[string]any, 0, len(cats))
	for _, c := range cats {
		catOut = append(catOut, map[string]any{"id": c.ID, "name": c.Name, "icon": c.Icon, "color": c.Color, "sort": c.Sort, "is_system": c.IsSystem})
	}
	var accs []store.PaymentAccount
	db.Where("user_id = ?", u.ID).Order("sort, id").Find(&accs)
	accByPM := map[int][]map[string]any{}
	for _, a := range accs {
		accByPM[a.PaymentMethodID] = append(accByPM[a.PaymentMethodID], map[string]any{"id": a.ID, "account": a.Account, "note": a.Note, "sort": a.Sort})
	}
	pmOut := make([]map[string]any, 0, len(pms))
	for _, p := range pms {
		list := accByPM[p.ID]
		if list == nil {
			list = []map[string]any{}
		}
		pmOut = append(pmOut, map[string]any{"id": p.ID, "name": p.Name, "icon": p.Icon, "accounts": list, "sort": p.Sort, "is_system": false})
	}
	bOut := make([]map[string]any, 0, len(bundles))
	for _, b := range bundles {
		bOut = append(bOut, map[string]any{"id": b.ID, "name": b.Name, "note": b.Note})
	}
	cOut := make([]map[string]any, 0, len(curs))
	for _, c := range curs {
		cOut = append(cOut, map[string]any{"code": c.Code, "name": c.Name, "name_zh": c.NameZh, "symbol": c.Symbol, "sort": c.Sort, "manual_rate": c.ManualRate, "is_custom": c.IsCustom})
	}
	sOut := make([]map[string]any, 0, len(subs))
	for _, s := range subs {
		sOut = append(sOut, subDict(s))
	}
	var svcs []store.UserService
	db.Where("user_id = ?", u.ID).Order("sort, id").Find(&svcs)
	svcOut := make([]map[string]any, 0, len(svcs))
	for _, s := range svcs {
		svcOut = append(svcOut, map[string]any{"name": s.Name, "website": s.Website, "category": s.Category, "icon": s.Icon, "sort": s.Sort})
	}
	return map[string]any{
		"categories": catOut, "payment_methods": pmOut, "bundles": bOut,
		"currencies": cOut, "services": svcOut, "subscriptions": sOut,
	}
}

func nowStamp() string { return time.Now().UTC().Format("2006-01-02T15:04:05") }

// ExportUser 导出单个用户数据。
func ExportUser(db *gorm.DB, u *store.User) map[string]any {
	store.EnsureUserDefaults(db, u)
	out := collect(db, u)
	out["export_version"] = ExportVersion
	out["app"] = "EasySub"
	out["exported_at"] = nowStamp()
	out["user"] = map[string]any{
		"username": u.Username, "base_currency": u.BaseCurrency, "locale": u.Locale,
		"theme": u.Theme, "notify_config": notify.LoadConfig(u),
	}
	return out
}

// ExportAll 整站备份（含账户与密码哈希），仅管理员使用。
func ExportAll(db *gorm.DB) map[string]any {
	var users []store.User
	db.Order("id").Find(&users)
	blocks := make([]map[string]any, 0, len(users))
	for i := range users {
		u := &users[i]
		store.EnsureUserDefaults(db, u)
		b := collect(db, u)
		b["user"] = map[string]any{
			"username": u.Username, "email": u.Email, "password_hash": u.PasswordHash,
			"is_admin": u.IsAdmin, "is_active": u.IsActive, "is_approved": u.IsApproved,
			"email_verified": u.EmailVerified, "locale": u.Locale, "theme": u.Theme,
			"base_currency": u.BaseCurrency, "category_order": u.CategoryOrder,
			"monthly_budget": u.MonthlyBudget, "notify_settings": u.NotifySettings,
			"notify_config": notify.LoadConfig(u),
		}
		blocks = append(blocks, b)
	}
	return map[string]any{
		"export_version": ExportVersion, "app": "EasySub", "scope": "all",
		"exported_at": nowStamp(), "users": blocks,
	}
}

// IsFullBackup 判断数据是否为整站备份。
func IsFullBackup(data map[string]any) bool {
	_, ok := data["users"].([]any)
	return ok
}

// ---------- 恢复 ----------

func str(m map[string]any, k string) string {
	switch v := m[k].(type) {
	case string:
		return v
	case float64:
		return strings.TrimSuffix(fmt.Sprintf("%v", v), ".0")
	}
	return ""
}

func strPtr(m map[string]any, k string, n int) *string {
	s := str(m, k)
	if s == "" {
		return nil
	}
	if r := []rune(s); len(r) > n {
		s = string(r[:n])
	}
	return &s
}

func trunc(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func num(m map[string]any, k string, def float64) float64 {
	if v, ok := m[k].(float64); ok {
		return v
	}
	return def
}

func boolean(m map[string]any, k string, def bool) bool {
	if v, ok := m[k].(bool); ok {
		return v
	}
	return def
}

func intKey(m map[string]any, k string) (int, bool) {
	if v, ok := m[k].(float64); ok {
		return int(v), true
	}
	return 0, false
}

func list(m map[string]any, k string) []map[string]any {
	raw, _ := m[k].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, x := range raw {
		if mm, ok := x.(map[string]any); ok {
			out = append(out, mm)
		}
	}
	return out
}

func mapRef(m map[string]any, k string, idMap map[int]int) *int {
	if id, ok := intKey(m, k); ok {
		if v, ok := idMap[id]; ok {
			return &v
		}
	}
	return nil
}

// restoreEntities 把一份导出数据恢复到用户名下（在事务 tx 中），返回导入的订阅数。
func restoreEntities(tx *gorm.DB, u *store.User, data map[string]any, replace bool) (int, error) {
	store.EnsureUserDefaults(tx, u)
	if replace {
		if err := tx.Where("user_id = ?", u.ID).Delete(&store.NotificationLog{}).Error; err != nil {
			return 0, err
		}
		if err := tx.Where("user_id = ?", u.ID).Delete(&store.Subscription{}).Error; err != nil {
			return 0, err
		}
	}

	var cats []store.Category
	tx.Where("user_id = ? OR is_system = ?", u.ID, true).Find(&cats)
	existingCats := map[string]int{}
	for _, c := range cats {
		existingCats[c.Name] = c.ID
	}
	var pms []store.PaymentMethod
	tx.Where("user_id = ?", u.ID).Find(&pms)
	existingPMs := map[string]*store.PaymentMethod{}
	for i := range pms {
		existingPMs[pms[i].Name] = &pms[i]
	}
	var bundles []store.Bundle
	tx.Where("user_id = ?", u.ID).Find(&bundles)
	existingBundles := map[string]int{}
	for _, b := range bundles {
		existingBundles[b.Name] = b.ID
	}

	catMap, pmMap, bundleMap := map[int]int{}, map[int]int{}, map[int]int{}
	legacyAcc := map[int]int{} // 旧版备份：原付款方式 id -> 新建的账号 id

	for _, c := range list(data, "categories") {
		name := str(c, "name")
		if name == "" {
			continue
		}
		id, ok := existingCats[name]
		if !ok {
			row := store.Category{Name: trunc(name, 64), Icon: strPtr(c, "icon", 128), Color: strPtr(c, "color", 16),
				Sort: int(num(c, "sort", 0)), UserID: &u.ID}
			if err := tx.Create(&row).Error; err != nil {
				return 0, err
			}
			id = row.ID
			existingCats[name] = id
		}
		if old, ok := intKey(c, "id"); ok {
			catMap[old] = id
		}
	}

	accMap := map[int]int{}
	for _, p := range list(data, "payment_methods") {
		name := str(p, "name")
		if name == "" {
			continue
		}
		row, ok := existingPMs[name]
		if !ok {
			row = &store.PaymentMethod{Name: trunc(name, 64), Icon: strPtr(p, "icon", 512), UserID: &u.ID, Sort: int(num(p, "sort", 0))}
			if err := tx.Create(row).Error; err != nil {
				return 0, err
			}
			existingPMs[name] = row
		}
		if old, ok := intKey(p, "id"); ok {
			pmMap[old] = row.ID
		}
		// 账号：新版为 accounts 列表；旧版备份为付款方式上的单个 account / note
		accIn := list(p, "accounts")
		if _, isList := p["accounts"].([]any); !isList && (str(p, "account") != "" || str(p, "note") != "") {
			accIn = []map[string]any{{"account": str(p, "account"), "note": str(p, "note")}}
		}
		if len(accIn) == 0 {
			continue
		}
		var have []store.PaymentAccount
		tx.Where("user_id = ? AND payment_method_id = ?", u.ID, row.ID).Find(&have)
		type accKey struct{ account, note string }
		byKey := map[accKey]int{}
		for _, a := range have {
			byKey[accKey{a.Account, a.Note}] = a.ID
		}
		for i, a := range accIn {
			acc, note := trunc(str(a, "account"), 255), trunc(str(a, "note"), 500)
			if acc == "" && note == "" {
				continue
			}
			id, dup := byKey[accKey{acc, note}]
			if !dup {
				ar := store.PaymentAccount{UserID: u.ID, PaymentMethodID: row.ID, Account: acc, Note: note, Sort: len(have) + i}
				if err := tx.Create(&ar).Error; err != nil {
					return 0, err
				}
				id = ar.ID
				byKey[accKey{acc, note}] = id
			}
			if old, ok := intKey(a, "id"); ok {
				accMap[old] = id
			}
			// 旧版备份没有账号 id：用付款方式 id 作为键，供订阅映射
			if _, ok := intKey(a, "id"); !ok {
				if old, ok := intKey(p, "id"); ok {
					legacyAcc[old] = id
				}
			}
		}
	}

	for _, b := range list(data, "bundles") {
		name := str(b, "name")
		if name == "" {
			continue
		}
		id, ok := existingBundles[name]
		if !ok {
			row := store.Bundle{Name: trunc(name, 128), Note: strPtr(b, "note", 255), UserID: u.ID}
			if err := tx.Create(&row).Error; err != nil {
				return 0, err
			}
			id = row.ID
			existingBundles[name] = id
		}
		if old, ok := intKey(b, "id"); ok {
			bundleMap[old] = id
		}
	}

	var curs []store.UserCurrency
	tx.Where("user_id = ?", u.ID).Find(&curs)
	haveCur := map[string]bool{}
	for _, c := range curs {
		haveCur[c.Code] = true
	}
	for _, c := range list(data, "currencies") {
		code := strings.ToUpper(str(c, "code"))
		if code == "" || haveCur[code] {
			continue
		}
		haveCur[code] = true
		def, inCatalog := store.CatalogCurrency(code)
		row := store.UserCurrency{UserID: u.ID, Code: trunc(code, 16), Name: str(c, "name"), NameZh: str(c, "name_zh"),
			Symbol: str(c, "symbol"), Sort: int(num(c, "sort", float64(len(haveCur)))), IsCustom: boolean(c, "is_custom", !inCatalog)}
		if row.Name == "" {
			row.Name = def.Name
		}
		if row.NameZh == "" {
			row.NameZh = def.NameZh
		}
		if row.Name == "" {
			row.Name = code
		}
		if r := num(c, "manual_rate", 0); r > 0 {
			row.ManualRate = &r
		}
		if err := tx.Create(&row).Error; err != nil {
			return 0, err
		}
	}

	// 服务库：备份中有而当前没有（按名称）的服务补回；已有的不覆盖
	if svcIn := list(data, "services"); len(svcIn) > 0 {
		icons.EnsureUserServices(tx, u)
		var svcs []store.UserService
		tx.Where("user_id = ?", u.ID).Find(&svcs)
		haveSvc := map[string]bool{}
		for _, s := range svcs {
			haveSvc[strings.ToLower(s.Name)] = true
		}
		for _, s := range svcIn {
			name := trunc(strings.TrimSpace(str(s, "name")), 128)
			if name == "" || haveSvc[strings.ToLower(name)] {
				continue
			}
			haveSvc[strings.ToLower(name)] = true
			website := trunc(str(s, "website"), 512)
			row := store.UserService{UserID: u.ID, Name: name, Website: website, Domain: store.NormalizeDomain(website),
				Category: icons.ValidCategory(str(s, "category")), Icon: strPtr(s, "icon", 512), Sort: int(num(s, "sort", 0))}
			if err := tx.Create(&row).Error; err != nil {
				return 0, err
			}
		}
	}

	today := store.NewDate(config.Now())
	count := 0
	for _, s := range list(data, "subscriptions") {
		start := today
		if d := store.ParseDatePtr(str(s, "start_date")); d != nil {
			start = *d
		}
		billingType := str(s, "billing_type")
		if billingType == "" {
			billingType = "recurring"
		}
		currency := str(s, "currency")
		if currency == "" {
			currency = u.BaseCurrency
		}
		name := str(s, "name")
		if name == "" {
			name = "导入订阅"
		}
		sub := store.Subscription{
			UserID: u.ID, Name: trunc(name, 128), Plan: strPtr(s, "plan", 128), Icon: strPtr(s, "icon", 512),
			URL: strPtr(s, "url", 512), Remark: strPtr(s, "remark", 255),
			IPv4: strPtr(s, "ipv4", 64), IPv6: strPtr(s, "ipv6", 64),
			CategoryID: mapRef(s, "category_id", catMap), PaymentMethodID: mapRef(s, "payment_method_id", pmMap),
			PaymentAccountID: mapRef(s, "payment_account_id", accMap),
			BundleID:         mapRef(s, "bundle_id", bundleMap),
			Amount:           num(s, "amount", 0), Currency: trunc(currency, 16), BillingType: billingType,
			Cycle: str(s, "cycle"), CycleCount: int(num(s, "cycle_count", 1)), StartDate: start,
			NextRenewalDate: store.ParseDatePtr(str(s, "next_renewal_date")),
			EndDate:         store.ParseDatePtr(str(s, "end_date")),
			LastRenewedAt:   store.ParseDatePtr(str(s, "last_renewed_at")),
			TrialEnd:        store.ParseDatePtr(str(s, "trial_end")),
			CancelBy:        store.ParseDatePtr(str(s, "cancel_by")),
			CardLast4:       strPtr(s, "card_last4", 8), CardExpiry: strPtr(s, "card_expiry", 8),
			IsActive: boolean(s, "is_active", true), AutoRenew: boolean(s, "auto_renew", true),
			ShowInCalendar: boolean(s, "show_in_calendar", true), Sort: int(num(s, "sort", 0)),
			RemindDaysBefore: str(s, "remind_days_before"),
		}
		if v := str(s, "notes"); v != "" {
			sub.Notes = &v
		}
		if raw, ok := s["family_members"].([]any); ok {
			fm := make([]string, 0, len(raw))
			for _, x := range raw {
				if xs, ok := x.(string); ok {
					fm = append(fm, xs)
				}
			}
			sub.FamilyMembers = store.JSONOf(fm)
		}
		if sub.PaymentAccountID == nil && sub.PaymentMethodID != nil {
			if _, hasAcc := s["payment_account_id"]; !hasAcc {
				sub.PaymentAccountID = mapRef(s, "payment_method_id", legacyAcc)
			}
		}
		if sub.Cycle == "" {
			sub.Cycle = "month"
		}
		if sub.CycleCount < 1 {
			sub.CycleCount = 1
		}
		if sub.RemindDaysBefore == "" {
			sub.RemindDaysBefore = "7,6,5,4,3,2,1"
		}
		if billingType == "recurring" && sub.NextRenewalDate == nil {
			nr := billing.NextRenewal(start, sub.Cycle, sub.CycleCount, today)
			sub.NextRenewalDate = &nr
		}
		if billingType == "one_time" {
			sub.NextRenewalDate = nil
		}
		if err := tx.Create(&sub).Error; err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

// ImportUser 把单用户备份恢复到 u 名下。
func ImportUser(db *gorm.DB, u *store.User, data map[string]any, replace bool) (int, error) {
	if _, ok := data["subscriptions"].([]any); !ok {
		if IsFullBackup(data) {
			return 0, fmt.Errorf("%w：这是整站备份文件，请使用整站恢复", ErrFormat)
		}
		return 0, fmt.Errorf("%w：缺少 subscriptions", ErrFormat)
	}
	count := 0
	err := db.Transaction(func(tx *gorm.DB) error {
		n, err := restoreEntities(tx, u, data, replace)
		if err != nil {
			return err
		}
		count = n
		if ud, ok := data["user"].(map[string]any); ok {
			if nc, ok := ud["notify_config"].(map[string]any); ok {
				notify.ApplyConfig(u, nc)
				return tx.Model(u).Select("notify_config", "telegram_enabled", "telegram_bot_token", "telegram_chat_id",
					"telegram_admin_id", "telegram_api_base", "telegram_proxy").Updates(u).Error
			}
		}
		return nil
	})
	return count, err
}

// ImportAllResult 整站恢复结果。
type ImportAllResult struct {
	Users        int `json:"users"`
	CreatedUsers int `json:"created_users"`
	Imported     int `json:"imported"`
}

// ImportAll 整站恢复：按用户名匹配账户，不存在则用备份中的账户信息新建。
func ImportAll(db *gorm.DB, data map[string]any, replace bool) (ImportAllResult, error) {
	var res ImportAllResult
	usersIn := list(data, "users")
	if _, ok := data["users"].([]any); !ok {
		return res, fmt.Errorf("%w：缺少 users（请使用整站备份文件）", ErrFormat)
	}
	res.Users = len(usersIn)
	err := db.Transaction(func(tx *gorm.DB) error {
		for _, ub := range usersIn {
			meta, _ := ub["user"].(map[string]any)
			if meta == nil {
				continue
			}
			username := str(meta, "username")
			if username == "" {
				continue
			}
			var target store.User
			err := tx.Where("username = ?", username).First(&target).Error
			if store.IsNotFound(err) {
				hash := str(meta, "password_hash")
				if hash == "" {
					hash, _ = store.HashPassword(username + "@reset")
				}
				email := str(meta, "email")
				if email == "" {
					email = username + "@example.com"
				}
				target = store.User{Username: trunc(username, 64), Email: email, PasswordHash: hash,
					IsAdmin: boolean(meta, "is_admin", false), IsActive: boolean(meta, "is_active", true),
					IsApproved: boolean(meta, "is_approved", true), EmailVerified: boolean(meta, "email_verified", true),
					Locale: or(str(meta, "locale"), "zh"), Theme: or(str(meta, "theme"), "light"),
					BaseCurrency: or(str(meta, "base_currency"), "CNY")}
				if raw, ok := meta["category_order"].([]any); ok {
					order := make([]int, 0, len(raw))
					for _, x := range raw {
						if f, ok := x.(float64); ok {
							order = append(order, int(f))
						}
					}
					target.CategoryOrder = store.JSONOf(order)
				}
				if err := tx.Create(&target).Error; err != nil {
					return err
				}
				res.CreatedUsers++
			} else if err != nil {
				return err
			}
			if nc, ok := meta["notify_config"].(map[string]any); ok {
				notify.ApplyConfig(&target, nc)
				if err := tx.Save(&target).Error; err != nil {
					return err
				}
			}
			n, err := restoreEntities(tx, &target, ub, replace)
			if err != nil {
				return err
			}
			res.Imported += n
		}
		return nil
	})
	return res, err
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
