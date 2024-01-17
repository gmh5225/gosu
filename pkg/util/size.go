package util

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/shirou/gopsutil/v3/mem"
)

type ParsableSize struct {
	Value int
}

func getTotalMemory() int64 {
	v, e := mem.VirtualMemory()
	if e != nil {
		// whatever
		return 16 * 1024 * 1024 * 1024
	}
	return int64(v.Available)
}

var units = map[string]float64{
	"b":        1,
	"byte":     1,
	"k":        1024,
	"kb":       1000,
	"kib":      1024,
	"kilobyte": 1000,
	"kibibyte": 1024,
	"m":        math.Pow(1024, 2),
	"mb":       math.Pow(1000, 2),
	"mib":      math.Pow(1024, 2),
	"megabyte": math.Pow(1000, 2),
	"mebibyte": math.Pow(1024, 2),
	"g":        math.Pow(1024, 3),
	"gb":       math.Pow(1000, 3),
	"gib":      math.Pow(1024, 3),
	"gigabyte": math.Pow(1000, 3),
	"gibibyte": math.Pow(1024, 3),
}

func Bytesize(d any) ParsableSize {

	switch d := d.(type) {
	case ParsableSize:
		return d
	case int:
		return ParsableSize{d}
	case int64:
		return ParsableSize{int(d)}
	case float64:
		return ParsableSize{int(d)}
	case uint:
		return ParsableSize{int(d)}
	case uint64:
		return ParsableSize{int(d)}
	case uint32:
		return ParsableSize{int(d)}
	case uint16:
		return ParsableSize{int(d)}
	case uint8:
		return ParsableSize{int(d)}
	case int32:
		return ParsableSize{int(d)}
	case int16:
		return ParsableSize{int(d)}
	case int8:
		return ParsableSize{int(d)}
	case string:
		res := ParsableSize{}
		res.UnmarshalText([]byte(d))
		return res
	default:
		panic(fmt.Errorf("unsupported type: %T", d))
	}
}

func (d ParsableSize) IsZero() bool {
	return d.Value == 0
}
func (d ParsableSize) IsPositive() bool {
	return d.Value > 0
}
func (d ParsableSize) String() string {
	if d.Value < 0 {
		return "-" + ParsableSize{-d.Value}.String()
	} else if d.Value == 0 {
		return "0"
	} else {
		if d.Value <= 1024 {
			return fmt.Sprintf("%db", d.Value)
		}
		d.Value >>= 10
		if d.Value <= 1024 {
			return fmt.Sprintf("%dkb", d.Value)
		}
		d.Value >>= 10
		if d.Value <= 1024 {
			return fmt.Sprintf("%dmb", d.Value)
		}
		d.Value >>= 10
		return fmt.Sprintf("%dgb", d.Value)
	}
}
func (d ParsableSize) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}
func (d *ParsableSize) UnmarshalText(text []byte) (err error) {
	var val float64
	var unit string
	_, err = fmt.Sscanf(string(text), "%f%s", &val, &unit)
	if err != nil {
		return
	}
	unit, _ = strings.CutSuffix(unit, "s")
	unit = strings.ToLower(unit)
	unit = strings.TrimPrefix(unit, " ")
	if unit == "" {
		unit = "b"
	}
	if unit == "%" || unit == "percent" {
		d.Value = int(val * float64(getTotalMemory()) / 100)
		return
	}
	if mult, ok := units[unit]; ok {
		d.Value = int(val * mult)
		return
	}
	unit = string([]byte(unit)[:1])
	if mult, ok := units[unit]; ok {
		d.Value = int(val * mult)
		return
	}
	return fmt.Errorf("invalid unit: %s", unit)
}

func (d ParsableSize) MarshalJSON() ([]byte, error) {
	if d.Value == 0 {
		return []byte("null"), nil
	}
	return json.Marshal(d.String())
}
func (d *ParsableSize) UnmarshalJSON(text []byte) (err error) {
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
		err = d.UnmarshalText(text)
		return
	} else {
		var fp float64
		err = json.Unmarshal(text, &fp)
		if err != nil {
			return
		}
		d.Value = int(fp)
		return
	}
}
