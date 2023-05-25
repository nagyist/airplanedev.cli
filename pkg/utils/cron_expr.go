package utils

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/airplanedev/cronexpr"
	"github.com/pkg/errors"
)

type CronExpr struct {
	Minute     string `json:"minute,omitempty"`
	Hour       string `json:"hour,omitempty"`
	DayOfMonth string `json:"dayOfMonth,omitempty"`
	Month      string `json:"month,omitempty"`
	DayOfWeek  string `json:"dayOfWeek,omitempty"`

	// cached so it doesn't need to be recalculated
	expr *cronexpr.Expression
}

func (ce CronExpr) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{})
	m["minute"] = ce.Minute
	m["hour"] = ce.Hour
	m["dayOfMonth"] = ce.DayOfMonth
	m["month"] = ce.Month
	m["dayOfWeek"] = ce.DayOfWeek
	return json.Marshal(m)
}

func (ce CronExpr) Value() (driver.Value, error) {
	return json.Marshal(ce)
}

func (ce *CronExpr) Scan(src interface{}) error {
	source, ok := src.([]byte)
	if !ok {
		return errors.New("type assertion .([]byte) failed")
	}
	return json.Unmarshal(source, ce)
}

func (ce *CronExpr) Parse(cronLine string) error {
	expr, err := cronexpr.Parse(cronLine)
	ce.expr = expr

	if err != nil {
		return err
	}

	parts := strings.Split(cronLine, " ")
	if len(parts) != 5 {
		return errors.Errorf("unexpected cron format: %s", cronLine)
	}
	ce.Minute = parts[0]
	ce.Hour = parts[1]
	ce.DayOfMonth = parts[2]
	ce.Month = parts[3]
	ce.DayOfWeek = parts[4]

	return nil
}

func (ce *CronExpr) Validate() error {
	if ce.expr != nil {
		return nil
	}

	expr, err := cronexpr.Parse(fmt.Sprintf("%s %s %s %s %s", ce.Minute, ce.Hour, ce.DayOfMonth, ce.Month, ce.DayOfWeek))
	ce.expr = expr // cache
	return err
}

func (ce *CronExpr) Next() (time.Time, error) {
	if err := ce.Validate(); err != nil {
		return time.Time{}, errors.New("invalid cron expression")
	}
	next := ce.expr.Next(time.Now().UTC())
	if next.IsZero() {
		return time.Time{}, errors.New("invalid next instance for cron expression")
	}
	return next, nil
}

func (ce *CronExpr) NextN(n uint) []time.Time {
	if err := ce.Validate(); err != nil {
		return nil
	}
	return ce.expr.NextN(time.Now().UTC(), n)
}

func (ce CronExpr) String() string {
	return fmt.Sprintf("%s %s %s %s %s", ce.Minute, ce.Hour, ce.DayOfMonth, ce.Month, ce.DayOfWeek)
}
