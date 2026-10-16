// Package exchange 拉取并缓存汇率，提供金额换算。
//
// 汇率以系统基准货币（EXCHANGE_API_BASE，默认 USD）存储：rate = 1 base 可换多少 quote。
// 用户自定义货币可设置手动汇率（同样语义），接口无该币种时使用。
package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"easysub/internal/config"
	"easysub/internal/store"
)

var client = &http.Client{Timeout: 20 * time.Second}

func base() string { return strings.ToUpper(config.C.ExchangeAPIBase) }

// Fetch 从汇率接口拉取以 base 为基准的汇率表。
func Fetch(ctx context.Context) (map[string]float64, error) {
	u := strings.TrimRight(config.C.ExchangeAPIURL, "/") + "/" + base()
	if config.C.ExchangeAPIKey != "" {
		u += "?access_key=" + url.QueryEscape(config.C.ExchangeAPIKey)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("汇率接口返回 HTTP %d", resp.StatusCode)
	}
	var data struct {
		Rates           map[string]float64 `json:"rates"`
		ConversionRates map[string]float64 `json:"conversion_rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	rates := data.Rates
	if len(rates) == 0 {
		rates = data.ConversionRates
	}
	if len(rates) == 0 {
		return nil, fmt.Errorf("汇率接口未返回 rates")
	}
	out := make(map[string]float64, len(rates))
	for k, v := range rates {
		out[strings.ToUpper(k)] = v
	}
	return out, nil
}

// Refresh 拉取并写入数据库，返回更新条数。
func Refresh(ctx context.Context, db *gorm.DB) (int, error) {
	rates, err := Fetch(ctx)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	rows := make([]store.ExchangeRate, 0, len(rates))
	for q, r := range rates {
		rows = append(rows, store.ExchangeRate{Base: base(), Quote: q, Rate: r, UpdatedAt: now})
	}
	err = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "base"}, {Name: "quote"}},
		DoUpdates: clause.AssignmentColumns([]string{"rate", "updated_at"}),
	}).CreateInBatches(rows, 200).Error
	return len(rows), err
}

// LastUpdated 返回最近一次汇率更新时间。
func LastUpdated(db *gorm.DB) *time.Time {
	var row store.ExchangeRate
	if err := db.Where("base = ?", base()).Order("updated_at desc").First(&row).Error; err != nil {
		return nil
	}
	return &row.UpdatedAt
}

// RefreshIfStale 仅在汇率过期（默认 12 小时）时联网刷新，失败静默。
func RefreshIfStale(ctx context.Context, db *gorm.DB) (bool, int) {
	if t := LastUpdated(db); t != nil && time.Since(*t) < 12*time.Hour {
		return false, 0
	}
	n, err := Refresh(ctx, db)
	if err != nil {
		return false, 0
	}
	return true, n
}

// Converter 在一次请求内复用的换算器（一次性加载全部汇率，避免逐条查询）。
type Converter struct {
	base  string
	rates map[string]float64
}

// NewConverter 加载系统汇率，并叠加该用户货币的手动汇率（userID=0 则不叠加）。
func NewConverter(db *gorm.DB, userID int) *Converter {
	c := &Converter{base: base(), rates: map[string]float64{}}
	var rows []store.ExchangeRate
	db.Where("base = ?", c.base).Find(&rows)
	for _, r := range rows {
		c.rates[r.Quote] = r.Rate
	}
	if userID > 0 {
		var curs []store.UserCurrency
		db.Where("user_id = ? AND manual_rate IS NOT NULL", userID).Find(&curs)
		for _, cu := range curs {
			if cu.ManualRate != nil && *cu.ManualRate > 0 {
				c.rates[cu.Code] = *cu.ManualRate
			}
		}
	}
	return c
}

func (c *Converter) rate(code string) float64 {
	if code == c.base {
		return 1
	}
	return c.rates[code]
}

// Convert 换算金额；缺少汇率时原样返回。
func (c *Converter) Convert(amount float64, from, to string) float64 {
	from, to = strings.ToUpper(from), strings.ToUpper(to)
	if from == to {
		return amount
	}
	rf, rt := c.rate(from), c.rate(to)
	if rf == 0 || rt == 0 {
		return amount
	}
	return amount / rf * rt
}

// Has 是否有该货币的汇率。
func (c *Converter) Has(code string) bool { return c.rate(strings.ToUpper(code)) != 0 }

// Round2 保留两位小数。
func Round2(v float64) float64 { return math.Round(v*100) / 100 }

// Round 保留 n 位小数。
func Round(v float64, n int) float64 {
	p := math.Pow(10, float64(n))
	return math.Round(v*p) / p
}
