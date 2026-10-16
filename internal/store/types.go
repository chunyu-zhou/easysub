package store

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Date 是只含日期的值：数据库存 DATE，JSON 序列化为 "2006-01-02"（与原版 API 一致）。
type Date struct{ time.Time }

const dateLayout = "2006-01-02"

func NewDate(t time.Time) Date {
	return Date{time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)}
}

func ParseDate(s string) (Date, error) {
	s = strings.TrimSpace(s)
	if len(s) > 10 {
		s = s[:10]
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return Date{}, err
	}
	return Date{t}, nil
}

// ParseDatePtr 解析可空日期，空串或非法格式返回 nil。
func ParseDatePtr(s string) *Date {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	d, err := ParseDate(s)
	if err != nil {
		return nil
	}
	return &d
}

func (d Date) String() string { return d.Format(dateLayout) }

func (d Date) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

func (d *Date) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	p, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = p
	return nil
}

func (d Date) Value() (driver.Value, error) { return d.String(), nil }

func (d *Date) Scan(v any) error {
	switch x := v.(type) {
	case time.Time:
		*d = NewDate(x)
	case string:
		p, err := ParseDate(x)
		if err != nil {
			return err
		}
		*d = p
	case []byte:
		p, err := ParseDate(string(x))
		if err != nil {
			return err
		}
		*d = p
	case nil:
		*d = Date{}
	default:
		return fmt.Errorf("store.Date: 无法解析 %T", v)
	}
	return nil
}

// AddDays 返回 d 加 n 天。
func (d Date) AddDays(n int) Date { return Date{d.AddDate(0, 0, n)} }

// DaysUntil 返回从 from 到 d 的天数（d - from）。
func (d Date) DaysUntil(from Date) int { return int(d.Sub(from.Time).Hours() / 24) }

// JSON 字段：数据库存 JSON 文本，读出为任意结构。
type JSON[T any] struct {
	V     T
	Valid bool
}

func JSONOf[T any](v T) JSON[T] { return JSON[T]{V: v, Valid: true} }

func (j JSON[T]) Value() (driver.Value, error) {
	if !j.Valid {
		return nil, nil
	}
	b, err := json.Marshal(j.V)
	return string(b), err
}

func (j *JSON[T]) Scan(v any) error {
	var raw []byte
	switch x := v.(type) {
	case nil:
		*j = JSON[T]{}
		return nil
	case string:
		raw = []byte(x)
	case []byte:
		raw = x
	default:
		return fmt.Errorf("store.JSON: 无法解析 %T", v)
	}
	if len(raw) == 0 || string(raw) == "null" {
		*j = JSON[T]{}
		return nil
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	*j = JSON[T]{V: out, Valid: true}
	return nil
}

func (j JSON[T]) MarshalJSON() ([]byte, error) {
	if !j.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(j.V)
}

func (j *JSON[T]) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*j = JSON[T]{}
		return nil
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*j = JSON[T]{V: out, Valid: true}
	return nil
}

// GormDataType 让 AutoMigrate 为 JSON 字段建 JSON/TEXT 列。
func (JSON[T]) GormDataType() string { return "json" }
