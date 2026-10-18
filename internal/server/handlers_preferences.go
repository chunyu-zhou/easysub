package server

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"easysub/internal/config"
	"easysub/internal/exchange"
	"easysub/internal/store"
)

func (s *Server) listCurrencies(c *Ctx) (any, error) {
	var rows []store.UserCurrency
	if err := c.DB.Where("user_id = ?", c.User.ID).Order("sort,id").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
func (s *Server) currencyCatalog(*Ctx) (any, error) { return store.UniqueCatalog(), nil }
func (s *Server) createCurrency(c *Ctx) (any, error) {
	var in struct {
		Code       string   `json:"code"`
		Name       string   `json:"name"`
		NameZh     string   `json:"name_zh"`
		Symbol     string   `json:"symbol"`
		ManualRate *float64 `json:"manual_rate"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	if len(code) < 2 || len(code) > 16 {
		return nil, Err(422, "currency code must be 2 to 16 characters")
	}
	if in.ManualRate != nil && *in.ManualRate <= 0 {
		return nil, Err(422, "manual rate must be positive")
	}
	var n int64
	c.DB.Model(&store.UserCurrency{}).Where("user_id=? AND code=?", c.User.ID, code).Count(&n)
	if n > 0 {
		return nil, Err(409, "currency already exists")
	}
	custom := true
	if d, ok := store.CatalogCurrency(code); ok {
		if in.Name == "" {
			in.Name = d.Name
		}
		if in.NameZh == "" {
			in.NameZh = d.NameZh
		}
		if in.Symbol == "" {
			in.Symbol = d.Symbol
		}
		custom = false
	}
	if strings.TrimSpace(in.Name) == "" {
		in.Name = code
	}
	if strings.TrimSpace(in.NameZh) == "" {
		in.NameZh = in.Name
	}
	var max int
	c.DB.Model(&store.UserCurrency{}).Where("user_id=?", c.User.ID).Select("COALESCE(MAX(sort), -1)").Scan(&max)
	row := store.UserCurrency{UserID: c.User.ID, Code: code, Name: strings.TrimSpace(in.Name), NameZh: strings.TrimSpace(in.NameZh), Symbol: strings.TrimSpace(in.Symbol), ManualRate: in.ManualRate, IsCustom: custom, Sort: max + 1}
	if err := c.DB.Create(&row).Error; err != nil {
		return nil, err
	}
	return row, nil
}
func (s *Server) updateCurrency(c *Ctx) (any, error) {
	code := strings.ToUpper(strings.TrimSpace(c.R.PathValue("code")))
	var row store.UserCurrency
	if err := c.DB.Where("user_id=? AND code=?", c.User.ID, code).First(&row).Error; err != nil {
		return nil, Err(404, "currency not found")
	}
	var in struct {
		Name       string   `json:"name"`
		NameZh     string   `json:"name_zh"`
		Symbol     string   `json:"symbol"`
		ManualRate *float64 `json:"manual_rate"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	if in.ManualRate != nil && *in.ManualRate <= 0 {
		return nil, Err(422, "manual rate must be positive")
	}
	if strings.TrimSpace(in.Name) != "" {
		row.Name = strings.TrimSpace(in.Name)
	}
	if strings.TrimSpace(in.NameZh) != "" {
		row.NameZh = strings.TrimSpace(in.NameZh)
	}
	row.Symbol = strings.TrimSpace(in.Symbol)
	row.ManualRate = in.ManualRate
	if err := c.DB.Save(&row).Error; err != nil {
		return nil, err
	}
	return row, nil
}
func (s *Server) deleteCurrency(c *Ctx) (any, error) {
	code := strings.ToUpper(strings.TrimSpace(c.R.PathValue("code")))
	if code == c.User.BaseCurrency {
		return nil, Err(422, "cannot delete base currency")
	}
	var used int64
	c.DB.Model(&store.Subscription{}).Where("user_id=? AND currency=?", c.User.ID, code).Count(&used)
	if used > 0 {
		return nil, Err(422, "currency is used by subscriptions")
	}
	res := c.DB.Where("user_id=? AND code=?", c.User.ID, code).Delete(&store.UserCurrency{})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, Err(404, "currency not found")
	}
	return map[string]any{"status": "ok"}, nil
}
func (s *Server) reorderCurrencies(c *Ctx) (any, error) {
	var in struct {
		Codes []string `json:"codes"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	for i, code := range in.Codes {
		if err := c.DB.Model(&store.UserCurrency{}).Where("user_id=? AND code=?", c.User.ID, strings.ToUpper(code)).Update("sort", i).Error; err != nil {
			return nil, err
		}
	}
	return map[string]any{"status": "ok"}, nil
}
func (s *Server) restoreCurrencies(c *Ctx) (any, error) {
	var have []store.UserCurrency
	c.DB.Where("user_id=?", c.User.ID).Find(&have)
	exists := map[string]bool{}
	max := 0
	for _, x := range have {
		exists[x.Code] = true
		if x.Sort > max {
			max = x.Sort
		}
	}
	added := 0
	for _, d := range store.DefaultCurrencies {
		if exists[d.Code] {
			continue
		}
		max++
		if err := c.DB.Create(&store.UserCurrency{UserID: c.User.ID, Code: d.Code, Name: d.Name, NameZh: d.NameZh, Symbol: d.Symbol, Sort: max}).Error; err != nil {
			return nil, err
		}
		added++
	}
	return map[string]any{"added": added}, nil
}
func (s *Server) currencyRates(c *Ctx) (any, error) {
	return map[string]any{"base": config.C.ExchangeAPIBase, "updated_at": exchange.LastUpdated(c.DB)}, nil
}
func (s *Server) currencyRateTable(c *Ctx) (any, error) {
	var currencies []store.UserCurrency
	c.DB.Where("user_id=?", c.User.ID).Order("sort,id").Find(&currencies)
	conv := c.Conv()
	items := make([]map[string]any, 0, len(currencies))
	for _, x := range currencies {
		items = append(items, map[string]any{"code": x.Code, "name": x.Name, "name_zh": x.NameZh, "symbol": x.Symbol, "manual_rate": x.ManualRate, "per_unit_in_base": conv.Convert(1, x.Code, c.User.BaseCurrency)})
	}
	return map[string]any{"base": c.User.BaseCurrency, "updated_at": exchange.LastUpdated(c.DB), "items": items}, nil
}
func (s *Server) refreshRates(c *Ctx) (any, error) {
	ctx, cancel := context.WithTimeout(c.Context(), 25*time.Second)
	defer cancel()
	n, err := exchange.Refresh(ctx, c.DB)
	if err != nil {
		return nil, Err(502, "could not refresh exchange rates: %v", err)
	}
	return map[string]any{"updated": n}, nil
}
func (s *Server) autoRefreshRates(c *Ctx) (any, error) {
	updated, n := exchange.RefreshIfStale(c.Context(), c.DB)
	return map[string]any{"updated": updated, "count": n}, nil
}

func paymentDTO(p store.PaymentMethod, accounts []store.PaymentAccount) map[string]any {
	if accounts == nil {
		accounts = []store.PaymentAccount{}
	}
	return map[string]any{"id": p.ID, "name": p.Name, "icon": p.Icon, "accounts": accounts, "sort": p.Sort, "is_system": p.IsSystem}
}

// paymentAccountsByMethod 读取用户全部付款账号，按付款方式分组。
func paymentAccountsByMethod(c *Ctx) map[int][]store.PaymentAccount {
	var rows []store.PaymentAccount
	c.DB.Where("user_id = ?", c.User.ID).Order("sort, id").Find(&rows)
	out := map[int][]store.PaymentAccount{}
	for _, a := range rows {
		out[a.PaymentMethodID] = append(out[a.PaymentMethodID], a)
	}
	return out
}

func (s *Server) listPaymentMethods(c *Ctx) (any, error) {
	var rows []store.PaymentMethod
	if err := c.DB.Where("user_id=?", c.User.ID).Order("sort,id").Find(&rows).Error; err != nil {
		return nil, err
	}
	accounts := paymentAccountsByMethod(c)
	out := make([]map[string]any, 0, len(rows))
	for _, x := range rows {
		out = append(out, paymentDTO(x, accounts[x.ID]))
	}
	return out, nil
}

type paymentAccountInput struct {
	ID      int    `json:"id"`
	Account string `json:"account"`
	Note    string `json:"note"`
}

type paymentMethodInput struct {
	Name     string                 `json:"name"`
	Icon     *string                `json:"icon"`
	Accounts *[]paymentAccountInput `json:"accounts"`
	// 兼容旧版 API：单个账号 / 备注（未传 accounts 时，创建付款方式会生成一个账号）
	Account *string `json:"account"`
	Note    *string `json:"note"`
}

func (in *paymentMethodInput) validate() error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 64 {
		return Err(422, "payment method name is required")
	}
	if in.Accounts != nil {
		if len(*in.Accounts) > 50 {
			return Err(422, "too many accounts")
		}
		for i := range *in.Accounts {
			a := &(*in.Accounts)[i]
			a.Account = truncRunes(strings.TrimSpace(a.Account), 255)
			a.Note = truncRunes(strings.TrimSpace(a.Note), 500)
		}
	}
	return nil
}

func truncRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// syncPaymentAccounts 让付款方式的账号与提交的列表一致：带 id 的更新、无 id 的新建、未出现的删除。
// 空白行（账号和备注都为空）忽略。被删除账号上的订阅引用清空。
func syncPaymentAccounts(tx *gorm.DB, userID, methodID int, list []paymentAccountInput) error {
	var existing []store.PaymentAccount
	if err := tx.Where("user_id = ? AND payment_method_id = ?", userID, methodID).Find(&existing).Error; err != nil {
		return err
	}
	byID := map[int]*store.PaymentAccount{}
	for i := range existing {
		byID[existing[i].ID] = &existing[i]
	}
	keep := map[int]bool{}
	sort := 0
	for _, a := range list {
		if a.Account == "" && a.Note == "" {
			continue
		}
		if row, ok := byID[a.ID]; ok && !keep[a.ID] {
			row.Account, row.Note, row.Sort = a.Account, a.Note, sort
			if err := tx.Save(row).Error; err != nil {
				return err
			}
			keep[a.ID] = true
		} else {
			row := store.PaymentAccount{UserID: userID, PaymentMethodID: methodID, Account: a.Account, Note: a.Note, Sort: sort}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		sort++
	}
	for id := range byID {
		if keep[id] {
			continue
		}
		if err := tx.Model(&store.Subscription{}).Where("user_id = ? AND payment_account_id = ?", userID, id).
			Update("payment_account_id", nil).Error; err != nil {
			return err
		}
		if err := tx.Delete(&store.PaymentAccount{}, id).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) savePaymentMethod(c *Ctx, row *store.PaymentMethod, in paymentMethodInput, isNew bool) (any, error) {
	row.Name = in.Name
	row.Icon = ptrString(in.Icon, 512)
	list := in.Accounts
	if list == nil && isNew && (in.Account != nil || in.Note != nil) {
		one := []paymentAccountInput{{Account: strings.TrimSpace(deref(in.Account)), Note: strings.TrimSpace(deref(in.Note))}}
		list = &one
	}
	err := c.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(row).Error; err != nil {
			return err
		}
		if list == nil {
			return nil
		}
		return syncPaymentAccounts(tx, c.User.ID, row.ID, *list)
	})
	if err != nil {
		return nil, err
	}
	return paymentDTO(*row, paymentAccountsByMethod(c)[row.ID]), nil
}

func (s *Server) createPaymentMethod(c *Ctx) (any, error) {
	var in paymentMethodInput
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	if err := in.validate(); err != nil {
		return nil, err
	}
	var max int
	c.DB.Model(&store.PaymentMethod{}).Where("user_id=?", c.User.ID).Select("COALESCE(MAX(sort),-1)").Scan(&max)
	row := store.PaymentMethod{UserID: &c.User.ID, Sort: max + 1}
	return s.savePaymentMethod(c, &row, in, true)
}
func (s *Server) updatePaymentMethod(c *Ctx) (any, error) {
	id, err := c.PathInt("id")
	if err != nil {
		return nil, err
	}
	var row store.PaymentMethod
	if err := c.DB.Where("id=? AND user_id=?", id, c.User.ID).First(&row).Error; err != nil {
		return nil, Err(404, "payment method not found")
	}
	var in paymentMethodInput
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	if err := in.validate(); err != nil {
		return nil, err
	}
	return s.savePaymentMethod(c, &row, in, false)
}
func (s *Server) deletePaymentMethod(c *Ctx) (any, error) {
	id, err := c.PathInt("id")
	if err != nil {
		return nil, err
	}
	err = c.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id=? AND user_id=?", id, c.User.ID).Delete(&store.PaymentMethod{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return Err(404, "payment method not found")
		}
		if err := tx.Where("user_id=? AND payment_method_id=?", c.User.ID, id).Delete(&store.PaymentAccount{}).Error; err != nil {
			return err
		}
		return tx.Model(&store.Subscription{}).Where("user_id=? AND payment_method_id=?", c.User.ID, id).
			Updates(map[string]any{"payment_method_id": nil, "payment_account_id": nil}).Error
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": "ok"}, nil
}
func (s *Server) reorderPaymentMethods(c *Ctx) (any, error) {
	var in struct {
		IDs []int `json:"ids"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	for i, id := range in.IDs {
		if err := c.DB.Model(&store.PaymentMethod{}).Where("id=? AND user_id=?", id, c.User.ID).Update("sort", i).Error; err != nil {
			return nil, err
		}
	}
	return map[string]any{"status": "ok"}, nil
}
func (s *Server) restorePaymentMethods(c *Ctx) (any, error) {
	var n int64
	c.DB.Model(&store.PaymentMethod{}).Where("user_id=?", c.User.ID).Count(&n)
	if n > 0 {
		return map[string]any{"added": 0}, nil
	}
	for i, d := range store.DefaultPaymentMethods {
		icon := d.Icon
		if err := c.DB.Create(&store.PaymentMethod{UserID: &c.User.ID, Name: d.Name, Icon: &icon, Sort: i}).Error; err != nil {
			return nil, err
		}
	}
	return map[string]any{"added": len(store.DefaultPaymentMethods)}, nil
}
