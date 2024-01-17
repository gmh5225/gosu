package util

import (
	"fmt"
	"time"
)

type TimerRate struct {
	Count  uint32
	Period time.Duration
}

func Rate(v ...any) TimerRate {
	if len(v) == 0 {
		return TimerRate{}
	} else if len(v) == 2 {
		return TimerRate{
			Count:  uint32(v[0].(int)),
			Period: Duration(v[1]).Duration,
		}
	} else if len(v) == 1 {
		v := v[0]
		switch v := v.(type) {
		case TimerRate:
			return v
		case string:
			var r TimerRate
			r.UnmarshalText([]byte(v))
			return r
		case float64:
			return TimerRate{
				Count:  uint32(v),
				Period: time.Second,
			}
		case float32:
			return TimerRate{
				Count:  uint32(v),
				Period: time.Second,
			}
		default:
		}
	}
	panic(fmt.Errorf("unsupported type: %T", v))
}

func (d TimerRate) IsZero() bool {
	return d.Period == 0
}
func (d TimerRate) String() string {
	return fmt.Sprintf("%d/%s", d.Count, d.Period)
}
func (d TimerRate) IsPositive() bool {
	return d.Count > 0 && d.Period > 0
}
func (d TimerRate) Compare(o TimerRate) int {
	o = o.Rescale(d.Period)
	return int(d.Count) - int(o.Count)
}
func (d TimerRate) Faster(o TimerRate) bool {
	return d.Compare(o) < 0
}
func (d TimerRate) Slower(o TimerRate) bool {
	return d.Compare(o) > 0
}
func (d TimerRate) Rescale(period time.Duration) TimerRate {
	count := float64(d.Count) * float64(d.Period.Milliseconds()) / float64(period.Milliseconds())
	return TimerRate{
		Count:  uint32(count),
		Period: period,
	}
}
func (d TimerRate) Clamp(rateMin TimerRate, rateMax TimerRate) TimerRate {
	cmin := rateMin.Rescale(d.Period).Count
	cmax := rateMax.Rescale(d.Period).Count
	return TimerRate{
		Count:  max(min(d.Count, cmax), cmin),
		Period: d.Period,
	}
}
func (d TimerRate) ClampPeriod(pmin time.Duration, pmax time.Duration) TimerRate {
	return TimerRate{
		Count:  d.Count,
		Period: max(min(d.Period, pmax), pmin),
	}
}
func (d TimerRate) ToTicks(t time.Time) uint32 {
	return uint32(int64(t.UnixNano()) / int64(d.Period.Nanoseconds()))
}
func (d TimerRate) Ticks() uint32 {
	return d.ToTicks(time.Now())
}

func (d TimerRate) MarshalText() ([]byte, error) {
	if d.IsZero() {
		return nil, nil
	}
	return []byte(d.String()), nil
}
func (d *TimerRate) UnmarshalText(text []byte) (err error) {
	if len(text) == 0 {
		*d = TimerRate{}
		return nil
	}
	var period string
	_, err = fmt.Sscanf(string(text), "%d/%s", &d.Count, &period)
	if err != nil {
		return
	}
	d.Period, err = time.ParseDuration(period)
	return
}
