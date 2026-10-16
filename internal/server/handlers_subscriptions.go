package server

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"easysub/internal/billing"
	"easysub/internal/config"
	"easysub/internal/store"
)

type subscriptionInput struct {
	Name             string      `json:"name"`
	Plan             *string     `json:"plan"`
	Icon             *string     `json:"icon"`
	URL              *string     `json:"url"`
	Notes            *string     `json:"notes"`
	Remark           *string     `json:"remark"`
	IPv4             *string     `json:"ipv4"`
	IPv6             *string     `json:"ipv6"`
	CategoryID       *int        `json:"category_id"`
	PaymentMethodID  *int        `json:"payment_method_id"`
	BundleID         *int        `json:"bundle_id"`
	Amount           float64     `json:"amount"`
	Currency         string      `json:"currency"`
	BillingType      string      `json:"billing_type"`
	Cycle            string      `json:"cycle"`
	CycleCount       int         `json:"cycle_count"`
	StartDate        store.Date  `json:"start_date"`
	NextRenewalDate  *store.Date `json:"next_renewal_date"`
	EndDate          *store.Date `json:"end_date"`
	TrialEnd         *store.Date `json:"trial_end"`
	CancelBy         *store.Date `json:"cancel_by"`
	CardLast4        *string     `json:"card_last4"`
	CardExpiry       *string     `json:"card_expiry"`
	IsActive         *bool       `json:"is_active"`
	AutoRenew        *bool       `json:"auto_renew"`
	ShowInCalendar   *bool       `json:"show_in_calendar"`
	FamilyMembers    []string    `json:"family_members"`
	RemindDaysBefore string      `json:"remind_days_before"`
}

func boolValue(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}
func ptrString(v *string, max int) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	r := []rune(s)
	if len(r) > max {
		s = string(r[:max])
	}
	return &s
}

func (s *Server) validateSubscription(c *Ctx, in *subscriptionInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 128 {
		return Err(422, "subscription name is required and must be at most 128 characters")
	}
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if in.Currency == "" {
		in.Currency = c.User.BaseCurrency
	}
	var count int64
	c.DB.Model(&store.UserCurrency{}).Where("user_id = ? AND code = ?", c.User.ID, in.Currency).Count(&count)
	if count == 0 {
		return Err(422, "currency is not in your currency list")
	}
	if in.Amount < 0 {
		return Err(422, "amount cannot be negative")
	}
	if in.BillingType == "" {
		in.BillingType = "recurring"
	}
	if in.BillingType != "recurring" && in.BillingType != "one_time" {
		return Err(422, "invalid billing type")
	}
	if in.Cycle == "" {
		in.Cycle = "month"
	}
	if in.Cycle != "day" && in.Cycle != "week" && in.Cycle != "month" && in.Cycle != "year" {
		return Err(422, "invalid billing cycle")
	}
	if in.CycleCount < 1 {
		in.CycleCount = 1
	}
	if in.CycleCount > 999 {
		return Err(422, "cycle count is too large")
	}
	if in.StartDate.IsZero() {
		in.StartDate = store.NewDate(config.Now())
	}
	if in.CategoryID != nil {
		var category store.Category
		if err := c.DB.Where("id = ? AND (user_id = ? OR is_system = ?)", *in.CategoryID, c.User.ID, true).First(&category).Error; err != nil {
			return Err(422, "category not found")
		}
	}
	if in.PaymentMethodID != nil {
		var pm store.PaymentMethod
		if err := c.DB.Where("id = ? AND user_id = ?", *in.PaymentMethodID, c.User.ID).First(&pm).Error; err != nil {
			return Err(422, "payment method not found")
		}
	}
	if in.BundleID != nil {
		var bundle store.Bundle
		if err := c.DB.Where("id = ? AND user_id = ?", *in.BundleID, c.User.ID).First(&bundle).Error; err != nil {
			return Err(422, "bundle not found")
		}
	}
	return nil
}

func subscriptionFrom(in *subscriptionInput, old *store.Subscription) store.Subscription {
	out := store.Subscription{Name: in.Name, Plan: ptrString(in.Plan, 128), Icon: ptrString(in.Icon, 512), URL: ptrString(in.URL, 512), Notes: ptrString(in.Notes, 100000), Remark: ptrString(in.Remark, 255), IPv4: ptrString(in.IPv4, 64), IPv6: ptrString(in.IPv6, 64), CategoryID: in.CategoryID, PaymentMethodID: in.PaymentMethodID, BundleID: in.BundleID, Amount: in.Amount, Currency: in.Currency, BillingType: in.BillingType, Cycle: in.Cycle, CycleCount: in.CycleCount, StartDate: in.StartDate, NextRenewalDate: in.NextRenewalDate, EndDate: in.EndDate, TrialEnd: in.TrialEnd, CancelBy: in.CancelBy, CardLast4: ptrString(in.CardLast4, 8), CardExpiry: ptrString(in.CardExpiry, 8), FamilyMembers: store.JSONOf(in.FamilyMembers), RemindDaysBefore: strings.TrimSpace(in.RemindDaysBefore)}
	if out.RemindDaysBefore == "" {
		out.RemindDaysBefore = "7,6,5,4,3,2,1"
	}
	if old == nil {
		out.IsActive = boolValue(in.IsActive, true)
		out.AutoRenew = boolValue(in.AutoRenew, true)
		out.ShowInCalendar = boolValue(in.ShowInCalendar, true)
	} else {
		out.IsActive = boolValue(in.IsActive, old.IsActive)
		out.AutoRenew = boolValue(in.AutoRenew, old.AutoRenew)
		out.ShowInCalendar = boolValue(in.ShowInCalendar, old.ShowInCalendar)
		out.LastRenewedAt = old.LastRenewedAt
		out.Sort = old.Sort
		out.ID = old.ID
		out.UserID = old.UserID
		out.CreatedAt = old.CreatedAt
	}
	if out.BillingType == "one_time" {
		out.NextRenewalDate = nil
		out.AutoRenew = false
	} else if out.NextRenewalDate == nil {
		d := billing.NextRenewal(out.StartDate, out.Cycle, out.CycleCount, store.NewDate(config.Now()))
		out.NextRenewalDate = &d
	}
	return out
}

func subscriptionDTO(c *Ctx, sub store.Subscription) map[string]any {
	return map[string]any{"id": sub.ID, "name": sub.Name, "plan": sub.Plan, "icon": sub.Icon, "url": sub.URL, "notes": sub.Notes, "remark": sub.Remark, "ipv4": sub.IPv4, "ipv6": sub.IPv6, "category_id": sub.CategoryID, "payment_method_id": sub.PaymentMethodID, "bundle_id": sub.BundleID, "amount": sub.Amount, "amount_in_base": c.Conv().Convert(sub.Amount, sub.Currency, c.User.BaseCurrency), "currency": sub.Currency, "billing_type": sub.BillingType, "cycle": sub.Cycle, "cycle_count": sub.CycleCount, "start_date": sub.StartDate, "next_renewal_date": sub.NextRenewalDate, "end_date": sub.EndDate, "last_renewed_at": sub.LastRenewedAt, "trial_end": sub.TrialEnd, "cancel_by": sub.CancelBy, "card_last4": sub.CardLast4, "card_expiry": sub.CardExpiry, "is_active": sub.IsActive, "auto_renew": sub.AutoRenew, "show_in_calendar": sub.ShowInCalendar, "sort": sub.Sort, "family_members": sub.FamilyMembers, "remind_days_before": sub.RemindDaysBefore, "created_at": sub.CreatedAt}
}

func (s *Server) listSubscriptions(c *Ctx) (any, error) {
	q := c.DB.Where("user_id = ?", c.User.ID)
	if v, ok := c.QueryBool("active"); ok {
		q = q.Where("is_active = ?", v)
	}
	if bt := c.R.URL.Query().Get("billing_type"); bt != "" {
		q = q.Where("billing_type = ?", bt)
	}
	var rows []store.Subscription
	if err := q.Order("sort, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, subscriptionDTO(c, row))
	}
	return out, nil
}

func (s *Server) createSubscription(c *Ctx) (any, error) {
	var in subscriptionInput
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	if err := s.validateSubscription(c, &in); err != nil {
		return nil, err
	}
	row := subscriptionFrom(&in, nil)
	row.UserID = c.User.ID
	var n int64
	c.DB.Model(&store.Subscription{}).Where("user_id = ?", c.User.ID).Count(&n)
	row.Sort = int(n)
	if err := c.DB.Create(&row).Error; err != nil {
		return nil, Err(500, "could not create subscription: %v", err)
	}
	store.Log("subscription.create", "created subscription: "+row.Name, c.User, "info")
	return subscriptionDTO(c, row), nil
}
func (s *Server) getSubscription(c *Ctx) (*store.Subscription, error) {
	id, err := c.PathInt("id")
	if err != nil {
		return nil, err
	}
	var row store.Subscription
	if err := c.DB.Where("id = ? AND user_id = ?", id, c.User.ID).First(&row).Error; err != nil {
		return nil, Err(404, "subscription not found")
	}
	return &row, nil
}
func (s *Server) updateSubscription(c *Ctx) (any, error) {
	row, err := s.getSubscription(c)
	if err != nil {
		return nil, err
	}
	var in subscriptionInput
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	if err := s.validateSubscription(c, &in); err != nil {
		return nil, err
	}
	next := subscriptionFrom(&in, row)
	if err := c.DB.Save(&next).Error; err != nil {
		return nil, Err(500, "could not update subscription: %v", err)
	}
	return subscriptionDTO(c, next), nil
}
func (s *Server) deleteSubscription(c *Ctx) (any, error) {
	row, err := s.getSubscription(c)
	if err != nil {
		return nil, err
	}
	var in struct {
		Password string `json:"password"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	if !checkPassword(in.Password, c.User.PasswordHash) {
		return nil, Err(422, "password is incorrect")
	}
	if err := c.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("subscription_id = ?", row.ID).Delete(&store.NotificationLog{}).Error; err != nil {
			return err
		}
		return tx.Delete(row).Error
	}); err != nil {
		return nil, Err(500, "could not delete subscription: %v", err)
	}
	return map[string]any{"status": "ok"}, nil
}
func (s *Server) renewSubscription(c *Ctx) (any, error) {
	row, err := s.getSubscription(c)
	if err != nil {
		return nil, err
	}
	if row.BillingType != "recurring" {
		return nil, Err(422, "one-time purchases cannot be renewed")
	}
	var in struct {
		Mode string `json:"mode"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	from := store.NewDate(config.Now())
	if in.Mode == "due" && row.NextRenewalDate != nil {
		from = *row.NextRenewalDate
	}
	next := billing.AddCycle(from, row.Cycle, row.CycleCount)
	today := store.NewDate(config.Now())
	for next.Before(today.Time) {
		next = billing.AddCycle(next, row.Cycle, row.CycleCount)
	}
	row.LastRenewedAt = &from
	row.NextRenewalDate = &next
	if err := c.DB.Save(row).Error; err != nil {
		return nil, err
	}
	return subscriptionDTO(c, *row), nil
}
func (s *Server) reorderSubscriptions(c *Ctx) (any, error) {
	var in struct {
		OrderedIDs []int `json:"ordered_ids"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	return map[string]any{"status": "ok"}, c.DB.Transaction(func(tx *gorm.DB) error {
		for i, id := range in.OrderedIDs {
			if err := tx.Model(&store.Subscription{}).Where("id = ? AND user_id = ?", id, c.User.ID).Update("sort", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *Server) exportSubscriptionsCSV(c *Ctx) (any, error) {
	var rows []store.Subscription
	c.DB.Where("user_id = ?", c.User.ID).Order("sort,id").Find(&rows)
	var b bytes.Buffer
	b.Write([]byte("\xEF\xBB\xBF"))
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"name", "plan", "amount", "currency", "billing_type", "cycle", "cycle_count", "start_date", "next_renewal_date", "url", "remark"})
	for _, x := range rows {
		next := ""
		if x.NextRenewalDate != nil {
			next = x.NextRenewalDate.String()
		}
		_ = w.Write([]string{x.Name, deref(x.Plan), fmt.Sprintf("%.2f", x.Amount), x.Currency, x.BillingType, x.Cycle, strconv.Itoa(x.CycleCount), x.StartDate.String(), next, deref(x.URL), deref(x.Remark)})
	}
	w.Flush()
	c.W.Header().Set("Content-Type", "text/csv; charset=utf-8")
	c.W.Header().Set("Content-Disposition", "attachment; filename=easysub-subscriptions.csv")
	_, _ = c.W.Write(b.Bytes())
	return Raw{}, nil
}
func (s *Server) importSubscriptionsCSV(c *Ctx) (any, error) {
	var in struct {
		Content string `json:"content"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(in.Content, string([]byte{0xef, 0xbb, 0xbf}))))
	records, err := r.ReadAll()
	if err != nil || len(records) < 2 {
		return nil, Err(422, "invalid CSV")
	}
	imported := 0
	for _, row := range records[1:] {
		if len(row) < 8 || strings.TrimSpace(row[0]) == "" {
			continue
		}
		amount, _ := strconv.ParseFloat(row[2], 64)
		cycleCount, _ := strconv.Atoi(row[6])
		start, err := store.ParseDate(row[7])
		if err != nil {
			start = store.NewDate(config.Now())
		}
		sub := store.Subscription{UserID: c.User.ID, Name: strings.TrimSpace(row[0]), Amount: amount, Currency: strings.ToUpper(row[3]), BillingType: row[4], Cycle: row[5], CycleCount: cycleCount, StartDate: start, IsActive: true, AutoRenew: true, ShowInCalendar: true, RemindDaysBefore: "7,6,5,4,3,2,1"}
		if len(row) > 1 {
			sub.Plan = ptr(row[1])
		}
		if len(row) > 8 {
			sub.NextRenewalDate = store.ParseDatePtr(row[8])
		}
		if len(row) > 9 {
			sub.URL = ptr(row[9])
		}
		if len(row) > 10 {
			sub.Remark = ptr(row[10])
		}
		if sub.CycleCount < 1 {
			sub.CycleCount = 1
		}
		if sub.BillingType != "one_time" {
			sub.BillingType = "recurring"
		}
		if sub.Currency == "" {
			sub.Currency = c.User.BaseCurrency
		}
		if sub.NextRenewalDate == nil && sub.BillingType == "recurring" {
			d := billing.NextRenewal(sub.StartDate, sub.Cycle, sub.CycleCount, store.NewDate(config.Now()))
			sub.NextRenewalDate = &d
		}
		if c.DB.Create(&sub).Error == nil {
			imported++
		}
	}
	return map[string]any{"imported": imported}, nil
}
func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
func ptr(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

func (s *Server) listCategories(c *Ctx) (any, error) {
	var rows []store.Category
	if err := c.DB.Where("user_id = ? OR is_system = ?", c.User.ID, true).Order("sort,id").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
func (s *Server) listBundles(c *Ctx) (any, error) {
	var rows []store.Bundle
	if err := c.DB.Where("user_id = ?", c.User.ID).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
func (s *Server) createBundle(c *Ctx) (any, error) {
	var in struct {
		Name string  `json:"name"`
		Note *string `json:"note"`
	}
	if err := c.Bind(&in); err != nil {
		return nil, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 128 {
		return nil, Err(422, "bundle name is required")
	}
	row := store.Bundle{UserID: c.User.ID, Name: in.Name, Note: ptrString(in.Note, 255)}
	if err := c.DB.Create(&row).Error; err != nil {
		return nil, err
	}
	return row, nil
}
func (s *Server) deleteBundle(c *Ctx) (any, error) {
	id, err := c.PathInt("id")
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": "ok"}, c.DB.Transaction(func(tx *gorm.DB) error {
		tx.Model(&store.Subscription{}).Where("user_id=? AND bundle_id=?", c.User.ID, id).Update("bundle_id", nil)
		res := tx.Where("id=? AND user_id=?", id, c.User.ID).Delete(&store.Bundle{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return Err(404, "bundle not found")
		}
		return nil
	})
}

var _ = http.MethodGet
var _ = sort.Slice
var _ = time.Now
