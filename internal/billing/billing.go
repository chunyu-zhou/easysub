// Package billing 处理订阅周期的日期推算。
package billing

import (
	"time"

	"easysub/internal/store"
)

// AddCycle 在日期 d 上加 count 个 cycle 周期（月末自动对齐，如 1/31 + 1 月 = 2/28）。
func AddCycle(d store.Date, cycle string, count int) store.Date {
	if count < 1 {
		count = 1
	}
	switch cycle {
	case "day":
		return d.AddDays(count)
	case "week":
		return d.AddDays(7 * count)
	case "year":
		return addMonths(d, 12*count)
	default:
		return addMonths(d, count)
	}
}

func addMonths(d store.Date, months int) store.Date {
	idx := int(d.Month()) - 1 + months
	year := d.Year() + idx/12
	month := idx%12 + 1
	if idx < 0 && idx%12 != 0 {
		year--
		month = idx%12 + 13
	}
	last := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
	day := d.Day()
	if day > last {
		day = last
	}
	return store.Date{Time: time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)}
}

// NextRenewal 从 start 起按周期推进，返回今天及之后的第一个续费日。
func NextRenewal(start store.Date, cycle string, count int, today store.Date) store.Date {
	nxt := start
	for i := 0; nxt.Before(today.Time) && i < 1000; i++ {
		nxt = AddCycle(nxt, cycle, count)
	}
	return nxt
}

// MonthlyFactor 把一个周期金额折算为每月的系数。
func MonthlyFactor(cycle string, count int) float64 {
	n := float64(count)
	if n < 1 {
		n = 1
	}
	switch cycle {
	case "day":
		return 30 / n
	case "week":
		return 52.0 / 12 / n
	case "month":
		return 1 / n
	case "year":
		return 1.0 / 12 / n
	}
	return 1
}
