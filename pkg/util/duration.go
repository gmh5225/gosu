package util

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type ParsableDuration struct {
	time.Duration
}

func Duration(d any) ParsableDuration {
	switch d := d.(type) {
	case ParsableDuration:
		return d
	case time.Duration:
		return ParsableDuration{d}
	case string:
		dur, _ := time.ParseDuration(d)
		return ParsableDuration{dur}
	case int64:
		return ParsableDuration{time.Duration(d)}
	case int:
		return ParsableDuration{time.Duration(d)}
	case float64:
		return ParsableDuration{time.Duration(d * float64(time.Millisecond))}
	default:
		panic(fmt.Errorf("unsupported type: %T", d))
	}
}

var closed = make(chan time.Time)

func init() {
	close(closed)
}

func (d ParsableDuration) IsZero() bool {
	return d.Duration == 0
}
func (d ParsableDuration) IsPositive() bool {
	return d.Duration > 0
}

// If positive duration, returns a channel that will be given the current time after the duration has passed.
// If non-positive duration, returns a closed channel, which will be selected immediately.
func (d ParsableDuration) After() <-chan time.Time {
	if !d.IsPositive() {
		return closed
	}
	return time.After(d.Duration)
}

// If positive duration, returns a channel that will be given the current time after the duration has passed.
// If non-positive duration, returns nil, which will never be selected.
func (d ParsableDuration) AfterIf() <-chan time.Time {
	if !d.IsPositive() {
		return nil
	}
	return time.After(d.Duration)
}

// Creates a context that will be cancelled after the duration has passed.
func (d ParsableDuration) TimeoutCause(ctx context.Context, cause error) (context.Context, context.CancelFunc) {
	if !d.IsPositive() {
		return context.WithCancel(ctx)
	}
	return context.WithTimeoutCause(ctx, d.Duration, cause)
}
func (d ParsableDuration) Timeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return d.TimeoutCause(ctx, nil)
}

// Clamps the duration to the given range.
func (d ParsableDuration) Clamp(lmin time.Duration, lmax time.Duration) ParsableDuration {
	return ParsableDuration{max(min(d.Duration, lmax), lmin)}
}
func (d ParsableDuration) Min(o time.Duration) ParsableDuration {
	return ParsableDuration{min(d.Duration, o)}
}
func (d ParsableDuration) Max(o time.Duration) ParsableDuration {
	return ParsableDuration{max(d.Duration, o)}
}

// String representation of the duration.
func (d ParsableDuration) String() string {
	return d.Duration.String()
}
func (d ParsableDuration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}
func (d *ParsableDuration) UnmarshalText(text []byte) (err error) {
	d.Duration, err = time.ParseDuration(string(text))
	return
}

// JSON representation of the duration.
const microToMilli = float64(time.Microsecond) / float64(time.Millisecond)

func (d ParsableDuration) MarshalJSON() ([]byte, error) {
	if d.Duration == 0 {
		return []byte("null"), nil
	}
	return json.Marshal(d.String())
}
func (d *ParsableDuration) UnmarshalJSON(text []byte) (err error) {
	if len(text) == 0 {
		return nil
	}
	if text[0] == 'n' {
		return nil
	} else if text[0] == '"' {
		var str string
		err = json.Unmarshal(text, &str)
		if err != nil {
			return
		}
		d.Duration, err = time.ParseDuration(str)
		return
	} else {
		var fp float64
		err = json.Unmarshal(text, &fp)
		if err != nil {
			return
		}
		d.Duration = time.Duration(fp / microToMilli)
		return
	}
}
