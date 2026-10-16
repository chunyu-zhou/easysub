// Package scheduler 定时任务：到期提醒、试用 / 取消 / 卡到期提醒、每周汇总、汇率刷新与自动备份。
package scheduler

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"easysub/internal/backup"
	"easysub/internal/config"
	"easysub/internal/exchange"
	"easysub/internal/notify"
	"easysub/internal/store"
)

var (
	once sync.Once
	cr   *cron.Cron
	// scanMu 防止定时任务与手动触发的扫描并发执行，导致重复发送
	scanMu sync.Mutex
)

// Start 启动定时任务（幂等）。
func Start() {
	once.Do(func() {
		hour, minute := 9, 0
		if h, m, ok := strings.Cut(config.C.ReminderScanTime, ":"); ok {
			if hv, err := strconv.Atoi(h); err == nil {
				hour = hv
			}
			if mv, err := strconv.Atoi(m); err == nil {
				minute = mv
			}
		}
		cr = cron.New(cron.WithLocation(config.C.Location))
		add := func(spec, name string, fn func()) {
			if _, err := cr.AddFunc(spec, func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[scheduler] %s panic: %v", name, r)
					}
				}()
				fn()
			}); err != nil {
				log.Printf("[scheduler] 注册 %s 失败：%v", name, err)
			}
		}
		add(fmt.Sprintf("%d %d * * *", minute, hour), "reminder", func() { RunReminderScan(context.Background()) })
		add(fmt.Sprintf("%d %d * * *", minute, hour), "date-reminder", func() { RunDateReminders(context.Background()) })
		add("0 8 * * *", "digest", func() { RunWeeklyDigest(context.Background()) })
		add("0 4 * * *", "rates", func() {
			if db := store.DB(); db != nil {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				if _, err := exchange.Refresh(ctx, db); err != nil {
					log.Printf("[scheduler] 汇率刷新失败：%v", err)
				}
			}
		})
		add("30 3 * * *", "local-backup", func() {
			if db := store.DB(); db != nil {
				if _, err := backup.RunLocalBackup(db); err != nil {
					log.Printf("[scheduler] 本地自动备份失败：%v", err)
				}
			}
		})
		add("45 3 * * *", "webdav-backup", func() {
			if db := store.DB(); db != nil {
				ok, failed := backup.RunAutoWebDAV(db)
				if ok+failed > 0 {
					log.Printf("[scheduler] WebDAV 自动备份：成功 %d，失败 %d", ok, failed)
				}
			}
		})
		cr.Start()
		log.Printf("[scheduler] 定时任务已启动（提醒扫描 %02d:%02d，时区 %s）", hour, minute, config.C.TZ)
	})
}

// Stop 停止定时任务。
func Stop() {
	if cr != nil {
		<-cr.Stop().Done()
	}
}

func parseDays(raw string) []int {
	var out []int
	for _, p := range strings.Split(raw, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func alreadySent(db *gorm.DB, subID, daysBefore int, today store.Date) bool {
	var n int64
	start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, config.C.Location)
	db.Model(&store.NotificationLog{}).
		Where("subscription_id = ? AND days_before = ? AND status = ? AND sent_at >= ? AND sent_at < ?",
			subID, daysBefore, "sent", start, start.Add(24*time.Hour)).Count(&n)
	return n > 0
}

func channelLabel(ok []string) string {
	switch len(ok) {
	case 0:
		return "none"
	case 1:
		return ok[0]
	}
	return fmt.Sprintf("multi:%d", len(ok))
}

func splitResults(rs []notify.Result) (ok, errs []string) {
	for _, r := range rs {
		if r.OK {
			ok = append(ok, r.Channel)
		} else {
			errs = append(errs, r.Channel+": "+r.Error)
		}
	}
	return
}

type userCache struct {
	db    *gorm.DB
	users map[int]*store.User
	cfgs  map[int]notify.Config
}

func newUserCache(db *gorm.DB) *userCache {
	return &userCache{db: db, users: map[int]*store.User{}, cfgs: map[int]notify.Config{}}
}

// get 返回启用了任一通知渠道的用户；否则返回 nil。
func (c *userCache) get(id int) *store.User {
	if u, ok := c.users[id]; ok {
		return u
	}
	var u store.User
	if err := c.db.First(&u, id).Error; err != nil {
		c.users[id] = nil
		return nil
	}
	cfg := notify.LoadConfig(&u)
	if !notify.AnyEnabled(cfg) {
		c.users[id] = nil
		return nil
	}
	c.users[id] = &u
	c.cfgs[id] = cfg
	return &u
}

// RunReminderScan 扫描即将续费的订阅并推送提醒。
func RunReminderScan(ctx context.Context) map[string]any {
	db := store.DB()
	if db == nil {
		return map[string]any{"sent": 0, "failed": 0, "skipped": "数据库未配置"}
	}
	scanMu.Lock()
	defer scanMu.Unlock()
	today := store.NewDate(config.Now())
	var subs []store.Subscription
	db.Where("is_active = ? AND billing_type = ? AND next_renewal_date IS NOT NULL", true, "recurring").Find(&subs)
	users := newUserCache(db)
	sent, failed := 0, 0
	for _, sub := range subs {
		u := users.get(sub.UserID)
		if u == nil {
			continue
		}
		daysLeft := sub.NextRenewalDate.DaysUntil(today)
		// 免打扰时段：非紧急（>=2 天）提醒暂缓，次日再评估
		if daysLeft >= 2 && notify.InQuietHours(u) {
			continue
		}
		for _, n := range parseDays(sub.RemindDaysBefore) {
			if daysLeft != n || alreadySent(db, sub.ID, n, today) {
				continue
			}
			textMD := BuildReminderText(db, &sub, u, daysLeft)
			results := notify.Dispatch(ctx, u, "续费提醒："+sub.Name, notify.StripMD(textMD), textMD, "reminder")
			ok, errs := splitResults(results)
			msg := textMD
			status := "sent"
			if len(ok) == 0 {
				status = "failed"
				msg = strings.Join(errs, "; ")
				if msg == "" {
					msg = "无可用渠道"
				}
			}
			db.Create(&store.NotificationLog{SubscriptionID: sub.ID, UserID: u.ID, DaysBefore: n,
				Channel: channelLabel(ok), Status: status, Message: &msg, SentAt: time.Now()})
			if len(ok) > 0 {
				sent++
				store.Log("notify.reminder", fmt.Sprintf("已提醒「%s」（提前 %d 天，渠道：%s）", sub.Name, n, strings.Join(ok, ", ")), u, "info")
			}
			if len(errs) > 0 {
				failed++
				level := "warn"
				if len(ok) == 0 {
					level = "error"
				}
				store.Log("notify.reminder", fmt.Sprintf("提醒「%s」部分渠道失败：%s", sub.Name, strings.Join(errs, "; ")), u, level)
			}
		}
	}
	return map[string]any{"sent": sent, "failed": failed}
}

// RunDateReminders 试用结束 / 取消截止 / 付款卡到期提醒（哨兵 days_before：-1 / -2 / -3）。
func RunDateReminders(ctx context.Context) map[string]any {
	db := store.DB()
	if db == nil {
		return map[string]any{"sent": 0, "skipped": "数据库未配置"}
	}
	scanMu.Lock()
	defer scanMu.Unlock()
	today := store.NewDate(config.Now())
	var subs []store.Subscription
	db.Where("is_active = ?", true).Find(&subs)
	users := newUserCache(db)
	sent := 0
	for _, sub := range subs {
		u := users.get(sub.UserID)
		if u == nil {
			continue
		}
		type event struct {
			code          int
			subject, body string
		}
		var events []event
		when := func(d int) string {
			if d == 0 {
				return "今天"
			}
			return fmt.Sprintf("还有 %d 天", d)
		}
		if sub.TrialEnd != nil {
			if d := sub.TrialEnd.DaysUntil(today); d == 3 || d == 1 || d == 0 {
				events = append(events, event{-1, "试用将结束：" + sub.Name,
					fmt.Sprintf("「%s」的免费试用%s结束（%s）。如不想被自动扣费，请及时取消或确认转正。", sub.Name, when(d), sub.TrialEnd)})
			}
		}
		if sub.CancelBy != nil {
			if d := sub.CancelBy.DaysUntil(today); d == 3 || d == 1 || d == 0 {
				events = append(events, event{-2, "取消截止：" + sub.Name,
					fmt.Sprintf("「%s」的取消截止日%s到（%s）。若要取消请在此之前操作。", sub.Name, when(d), sub.CancelBy)})
			}
		}
		if sub.CardExpiry != nil && cardExpiring(*sub.CardExpiry, today) {
			tail := ""
			if sub.CardLast4 != nil && *sub.CardLast4 != "" {
				tail = "（尾号 " + *sub.CardLast4 + "）"
			}
			events = append(events, event{-3, "付款卡将到期：" + sub.Name,
				fmt.Sprintf("「%s」绑定的付款卡%s将于 %s 到期，请及时更换，避免自动续费失败。", sub.Name, tail, *sub.CardExpiry)})
		}
		for _, ev := range events {
			if alreadySent(db, sub.ID, ev.code, today) {
				continue
			}
			ok, _ := splitResults(notify.Dispatch(ctx, u, ev.subject, ev.body, "", "reminder"))
			status := "failed"
			if len(ok) > 0 {
				status = "sent"
				sent++
			}
			body := ev.body
			db.Create(&store.NotificationLog{SubscriptionID: sub.ID, UserID: u.ID, DaysBefore: ev.code,
				Channel: channelLabel(ok), Status: status, Message: &body, SentAt: time.Now()})
		}
	}
	return map[string]any{"sent": sent}
}

// cardExpiring 卡有效期 MM/YY 在 45 天内到期则返回 true。
func cardExpiring(mmyy string, today store.Date) bool {
	mm, yy, ok := strings.Cut(strings.ReplaceAll(mmyy, " ", ""), "/")
	if !ok {
		return false
	}
	m, err1 := strconv.Atoi(mm)
	y, err2 := strconv.Atoi(yy)
	if err1 != nil || err2 != nil || m < 1 || m > 12 {
		return false
	}
	if len(yy) == 2 {
		y += 2000
	}
	lastDay := time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC)
	days := store.NewDate(lastDay).DaysUntil(today)
	return days >= 0 && days <= 45
}

// RunWeeklyDigest 每周汇总（按用户设定的星期几，0=周一）。
func RunWeeklyDigest(ctx context.Context) map[string]any {
	db := store.DB()
	if db == nil {
		return map[string]any{"sent": 0, "skipped": "数据库未配置"}
	}
	today := store.NewDate(config.Now())
	weekday := (int(today.Weekday()) + 6) % 7
	var users []store.User
	db.Where("is_active = ?", true).Find(&users)
	sent := 0
	for i := range users {
		u := &users[i]
		st := u.NotifySettings.V
		if enabled, _ := st["digest_enabled"].(bool); !enabled {
			continue
		}
		if toInt(st["digest_weekday"]) != weekday {
			continue
		}
		if !notify.AnyEnabled(notify.LoadConfig(u)) {
			continue
		}
		var subs []store.Subscription
		db.Where("user_id = ? AND is_active = ? AND billing_type = ? AND next_renewal_date IS NOT NULL", u.ID, true, "recurring").
			Order("next_renewal_date").Find(&subs)
		var upcoming, overdue []store.Subscription
		for _, s := range subs {
			d := s.NextRenewalDate.DaysUntil(today)
			if d < 0 {
				overdue = append(overdue, s)
			} else if d <= 30 {
				upcoming = append(upcoming, s)
			}
		}
		if len(upcoming) == 0 && len(overdue) == 0 {
			continue
		}
		lines := []string{"📅 *本周订阅汇总*", ""}
		if len(overdue) > 0 {
			lines = append(lines, fmt.Sprintf("⚠️ 已过期 %d 项：", len(overdue)))
			for _, s := range head(overdue, 10) {
				lines = append(lines, fmt.Sprintf("· %s（%s）", escapeMD(s.Name), s.NextRenewalDate))
			}
			lines = append(lines, "")
		}
		if len(upcoming) > 0 {
			lines = append(lines, fmt.Sprintf("🔔 未来 30 天将续费 %d 项：", len(upcoming)))
			for _, s := range head(upcoming, 15) {
				lines = append(lines, fmt.Sprintf("· %s — %s（%d 天，%.2f %s）", escapeMD(s.Name), s.NextRenewalDate,
					s.NextRenewalDate.DaysUntil(today), s.Amount, s.Currency))
			}
		}
		textMD := strings.Join(lines, "\n")
		notify.Dispatch(ctx, u, "本周订阅汇总", notify.StripMD(textMD), textMD, "digest")
		sent++
	}
	return map[string]any{"sent": sent}
}

func head(s []store.Subscription, n int) []store.Subscription {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	}
	return 0
}

var cycleCN = map[string]string{"day": "天", "week": "周", "month": "个月", "year": "年"}

func escapeMD(s string) string {
	return strings.NewReplacer("_", `\_`, "*", `\*`, "`", "\\`", "[", `\[`).Replace(s)
}

// BuildReminderText 构造一条信息完整的续费提醒（Markdown）。
func BuildReminderText(db *gorm.DB, sub *store.Subscription, u *store.User, daysLeft int) string {
	amount := fmt.Sprintf("%.2f %s", sub.Amount, sub.Currency)
	conv := exchange.NewConverter(db, u.ID)
	baseStr := ""
	if !strings.EqualFold(sub.Currency, u.BaseCurrency) {
		baseStr = fmt.Sprintf("（≈ %.2f %s）", conv.Convert(sub.Amount, sub.Currency, u.BaseCurrency), u.BaseCurrency)
	}
	var head, when string
	if daysLeft <= 0 {
		when, head = "⚠️ *今天到期*", "🔔 *续费提醒*｜今天就到期啦"
	} else {
		when, head = fmt.Sprintf("还有 *%d* 天", daysLeft), fmt.Sprintf("🔔 *续费提醒*｜还有 %d 天到期", daysLeft)
	}
	unit := cycleCN[sub.Cycle]
	if unit == "" {
		unit = sub.Cycle
	}
	cycleStr := "每" + unit
	if sub.CycleCount > 1 {
		cycleStr = fmt.Sprintf("每 %d %s", sub.CycleCount, unit)
	}
	title := escapeMD(sub.Name)
	if sub.Plan != nil && *sub.Plan != "" {
		title += "（" + escapeMD(*sub.Plan) + "）"
	}
	lines := []string{head, "", "📦 项目：*" + title + "*"}
	if sub.CategoryID != nil {
		var cat store.Category
		if db.First(&cat, *sub.CategoryID).Error == nil {
			lines = append(lines, "🗂️ 分类："+escapeMD(cat.Name))
		}
	}
	lines = append(lines, fmt.Sprintf("📅 到期：*%s*（%s）", sub.NextRenewalDate, when))
	lines = append(lines, fmt.Sprintf("💰 金额：*%s*%s · %s", amount, baseStr, cycleStr))
	if sub.PaymentMethodID != nil {
		var pm store.PaymentMethod
		if db.First(&pm, *sub.PaymentMethodID).Error == nil {
			line := "💳 付款：" + escapeMD(pm.Name)
			if pm.Account != nil && *pm.Account != "" {
				line += "（" + escapeMD(*pm.Account) + "）"
			}
			lines = append(lines, line)
		}
	}
	autoRenew := "关"
	if sub.AutoRenew {
		autoRenew = "开"
	}
	lines = append(lines, "🔁 自动续费："+autoRenew)
	if sub.FamilyMembers.Valid && len(sub.FamilyMembers.V) > 0 {
		lines = append(lines, "👨‍👩‍👧 家庭成员："+escapeMD(strings.Join(sub.FamilyMembers.V, "、")))
	}
	if sub.Remark != nil && *sub.Remark != "" {
		lines = append(lines, "📝 备注："+escapeMD(*sub.Remark))
	}
	if sub.URL != nil && *sub.URL != "" {
		lines = append(lines, "🔗 官网："+*sub.URL)
	}
	lines = append(lines, "")
	if daysLeft <= 0 {
		lines = append(lines, "👉 别忘了今天处理一下，保号 / 续费就万无一失～")
	} else {
		lines = append(lines, "👉 早点安排续费，省心又安心，避免到期失效～")
	}
	return strings.Join(lines, "\n")
}
