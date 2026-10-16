package server

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"time"

	"easysub/internal/billing"
	"easysub/internal/store"
)

func monthlyCost(c *Ctx, sub store.Subscription) float64 {
	return c.Conv().Convert(sub.Amount, sub.Currency, c.User.BaseCurrency) * billing.MonthlyFactor(sub.Cycle, sub.CycleCount)
}
func scopedCategories(c *Ctx) map[int]string {
	var rows []store.Category
	c.DB.Where("user_id=? OR is_system=?", c.User.ID, true).Find(&rows)
	m := map[int]string{}
	for _, x := range rows {
		m[x.ID] = x.Name
	}
	return m
}
func categoryName(m map[int]string, id *int) string {
	if id != nil && m[*id] != "" {
		return m[*id]
	}
	return "Uncategorized"
}
func activeRecurring(c *Ctx) []store.Subscription {
	var rows []store.Subscription
	c.DB.Where("user_id=? AND is_active=? AND billing_type=?", c.User.ID, true, "recurring").Find(&rows)
	return rows
}
func dtoList(c *Ctx, rows []store.Subscription) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, x := range rows {
		out = append(out, subscriptionDTO(c, x))
	}
	return out
}

func (s *Server) dashboard(c *Ctx) (any, error) {
	rows := activeRecurring(c)
	today := store.NewDate(time.Now())
	month := 0.0
	upcoming := []store.Subscription{}
	expired := 0
	for _, x := range rows {
		month += monthlyCost(c, x)
		if x.NextRenewalDate == nil {
			continue
		}
		d := x.NextRenewalDate.DaysUntil(today)
		if d < 0 {
			expired++
		}
		if d >= 0 && d <= 30 {
			upcoming = append(upcoming, x)
		}
	}
	sort.Slice(upcoming, func(i, j int) bool { return upcoming[i].NextRenewalDate.Before(upcoming[j].NextRenewalDate.Time) })
	var recent []store.Subscription
	c.DB.Where("user_id=?", c.User.ID).Order("created_at desc").Limit(8).Find(&recent)
	return map[string]any{"base_currency": c.User.BaseCurrency, "month_spend": month, "year_spend": month * 12, "active_count": len(rows), "expired_count": expired, "monthly_budget": c.User.MonthlyBudget, "upcoming": dtoList(c, upcoming), "recent": dtoList(c, recent)}, nil
}
func (s *Server) reportInsights(c *Ctx) (any, error) {
	rows := activeRecurring(c)
	cats := scopedCategories(c)
	by := map[string]float64{}
	for _, x := range rows {
		n := categoryName(cats, x.CategoryID)
		by[n] += monthlyCost(c, x)
	}
	total := 0.0
	for _, v := range by {
		total += v
	}
	out := make([]map[string]any, 0, len(by))
	for n, v := range by {
		p := 0.0
		if total > 0 {
			p = v / total * 100
		}
		out = append(out, map[string]any{"category": n, "monthly": v, "percent": p})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["monthly"].(float64) > out[j]["monthly"].(float64) })
	return map[string]any{"base_currency": c.User.BaseCurrency, "monthly_total": total, "breakdown": out}, nil
}
func (s *Server) reportExpired(c *Ctx) (any, error) {
	rows := activeRecurring(c)
	today := store.NewDate(time.Now())
	out := []store.Subscription{}
	for _, x := range rows {
		if x.NextRenewalDate != nil && x.NextRenewalDate.Before(today.Time) {
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NextRenewalDate.After(out[j].NextRenewalDate.Time) })
	return dtoList(c, out), nil
}
func (s *Server) reportRanking(c *Ctx) (any, error) {
	rows := activeRecurring(c)
	sort.Slice(rows, func(i, j int) bool { return monthlyCost(c, rows[i]) > monthlyCost(c, rows[j]) })
	out := dtoList(c, rows)
	for i := range out {
		out[i]["amount_in_base"] = monthlyCost(c, rows[i])
	}
	return out, nil
}
func (s *Server) reportOneTime(c *Ctx) (any, error) {
	var rows []store.Subscription
	c.DB.Where("user_id=? AND billing_type=?", c.User.ID, "one_time").Find(&rows)
	return dtoList(c, rows), nil
}
func (s *Server) reportUpcoming(c *Ctx) (any, error) {
	days := c.QueryInt("days", 30)
	if days < 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	today := store.NewDate(time.Now())
	rows := activeRecurring(c)
	out := []store.Subscription{}
	for _, x := range rows {
		if x.NextRenewalDate != nil && x.NextRenewalDate.DaysUntil(today) >= 0 && x.NextRenewalDate.DaysUntil(today) <= days {
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NextRenewalDate.Before(out[j].NextRenewalDate.Time) })
	return dtoList(c, out), nil
}
func (s *Server) reportCategoryDetail(c *Ctx) (any, error) {
	var rows []store.Subscription
	c.DB.Where("user_id=? AND is_active=?", c.User.ID, true).Find(&rows)
	cats := scopedCategories(c)
	rec := map[string]map[string]any{}
	one := map[string]map[string]any{}
	for _, x := range rows {
		n := categoryName(cats, x.CategoryID)
		if x.BillingType == "recurring" {
			d := rec[n]
			if d == nil {
				d = map[string]any{"category": n, "count": 0, "monthly": 0.0}
				rec[n] = d
			}
			d["count"] = d["count"].(int) + 1
			d["monthly"] = d["monthly"].(float64) + monthlyCost(c, x)
		} else {
			d := one[n]
			if d == nil {
				d = map[string]any{"category": n, "count": 0, "total": 0.0}
				one[n] = d
			}
			d["count"] = d["count"].(int) + 1
			d["total"] = d["total"].(float64) + c.Conv().Convert(x.Amount, x.Currency, c.User.BaseCurrency)
		}
	}
	r := []map[string]any{}
	o := []map[string]any{}
	rt, ot := 0.0, 0.0
	for _, v := range rec {
		rt += v["monthly"].(float64)
		r = append(r, v)
	}
	for _, v := range one {
		ot += v["total"].(float64)
		o = append(o, v)
	}
	sort.Slice(r, func(i, j int) bool { return r[i]["monthly"].(float64) > r[j]["monthly"].(float64) })
	sort.Slice(o, func(i, j int) bool { return o[i]["total"].(float64) > o[j]["total"].(float64) })
	return map[string]any{"base_currency": c.User.BaseCurrency, "recurring": r, "one_time": o, "recurring_monthly_total": rt, "one_time_total": ot}, nil
}
func (s *Server) reportRecentPayments(c *Ctx) (any, error) {
	limit := c.QueryInt("limit", 20)
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	var rows []store.Subscription
	c.DB.Where("user_id=?", c.User.ID).Find(&rows)
	cats := scopedCategories(c)
	out := []map[string]any{}
	for _, x := range rows {
		var date *store.Date
		if x.LastRenewedAt != nil {
			date = x.LastRenewedAt
		} else if x.BillingType == "one_time" {
			d := x.StartDate
			date = &d
		}
		if date == nil {
			continue
		}
		out = append(out, map[string]any{"id": x.ID, "name": x.Name, "plan": x.Plan, "remark": x.Remark, "icon": x.Icon, "category": categoryName(cats, x.CategoryID), "date": date, "amount": x.Amount, "currency": x.Currency, "amount_in_base": c.Conv().Convert(x.Amount, x.Currency, c.User.BaseCurrency), "billing_type": x.BillingType})
	}
	sort.Slice(out, func(i, j int) bool { return fmt.Sprint(out[i]["date"]) > fmt.Sprint(out[j]["date"]) })
	if len(out) > limit {
		out = out[:limit]
	}
	return map[string]any{"base_currency": c.User.BaseCurrency, "items": out}, nil
}
func (s *Server) exportReportCSV(c *Ctx) (any, error) {
	var rows []store.Subscription
	c.DB.Where("user_id=?", c.User.ID).Find(&rows)
	cats := scopedCategories(c)
	var b bytes.Buffer
	b.Write([]byte("\xEF\xBB\xBF"))
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"name", "category", "billing_type", "amount", "currency", "monthly", "yearly", "next_renewal_date", "active"})
	for _, x := range rows {
		month := 0.0
		if x.BillingType == "recurring" {
			month = monthlyCost(c, x)
		}
		next := ""
		if x.NextRenewalDate != nil {
			next = x.NextRenewalDate.String()
		}
		_ = w.Write([]string{x.Name, categoryName(cats, x.CategoryID), x.BillingType, fmt.Sprintf("%.2f", x.Amount), x.Currency, fmt.Sprintf("%.2f", month), fmt.Sprintf("%.2f", month*12), next, fmt.Sprint(x.IsActive)})
	}
	w.Flush()
	c.W.Header().Set("Content-Type", "text/csv; charset=utf-8")
	c.W.Header().Set("Content-Disposition", "attachment; filename=easysub-report.csv")
	_, _ = c.W.Write(b.Bytes())
	return Raw{}, nil
}
