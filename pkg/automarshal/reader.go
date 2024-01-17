package automarshal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/can1357/gosu/pkg/foreign"
	"github.com/can1357/gosu/pkg/util"
)

type ArgReader struct {
	buf []rune
	pos int
}

var identifierOk = util.NewRuneSet(util.RunesAlphanum.String(), "_-:$")
var identifierOkFirst = util.RunesAlpha

func (r *ArgReader) Len() int {
	return len(r.buf) - r.pos
}
func (r *ArgReader) Finished() bool {
	return r.Len() == 0
}

func (r *ArgReader) Peek() rune {
	if r.pos >= len(r.buf) {
		return 0
	}
	return r.buf[r.pos]
}
func (r *ArgReader) Read() (res rune, e error) {
	if r.pos >= len(r.buf) {
		return 0, io.EOF
	}
	r.pos++
	return r.buf[r.pos-1], nil
}

func (r *ArgReader) TakeAnyIf(c *util.RuneSet) (rune, bool) {
	if char, err := r.Read(); err != nil {
		return 0, false
	} else if !c.TestRune(rune(char)) {
		r.Unread()
		return 0, false
	} else {
		return char, true
	}
}
func (r *ArgReader) TakeAnyN(c *util.RuneSet) string {
	res := strings.Builder{}
	for {
		if char, ok := r.TakeAnyIf(c); !ok {
			return res.String()
		} else {
			res.WriteRune(char)
		}
	}
}
func (r *ArgReader) TakeIf(c rune) bool {
	if c != ' ' {
		r.Skipspace()
	}
	if char, err := r.Read(); err != nil {
		return false
	} else if char != c {
		r.Unread()
		return false
	}
	return true
}
func (r *ArgReader) Take(c rune) (e error) {
	if !r.TakeIf(c) {
		return fmt.Errorf("expected %c", c)
	}
	return nil
}

func (r *ArgReader) Unread() error {
	r.pos--
	return nil
}
func (r *ArgReader) Remains() string {
	return string(r.buf[r.pos:])
}
func (args *ArgReader) Skipspace() {
	for {
		char, err := args.Read()
		if err != nil {
			return
		}
		if char != ' ' && char != '\t' && char != '\n' && char != '\r' {
			args.Unread()
			return
		}
	}
}
func (args *ArgReader) Significant() (char rune, err error) {
	args.Skipspace()
	return args.Read()
}

func (args *ArgReader) Quoted(q rune) (r string, e error) {
	result := strings.Builder{}
	escape := false
	for {
		char, err := args.Read()
		if err != nil {
			return "", err
		} else if escape {
			escape = false
		} else {
			if char == '\\' {
				escape = true
				continue
			} else if char == q {
				return result.String(), nil
			}
		}
		result.WriteRune(char)
	}
}
func (args *ArgReader) Text() (res string, err error) {
	result := strings.Builder{}
	first := true
	for {
		var char rune
		if first {
			char, err = args.Significant()
			if char == '"' || char == '\'' {
				return args.Quoted(char)
			}
		} else {
			char, err = args.Read()
		}
		first = false
		if err != nil {
			if err == io.EOF {
				return result.String(), nil
			}
			return "", err
		} else if char == '"' || char == '\'' {
			args.Unread()
			return result.String(), nil
		}
		result.WriteRune(char)
	}
}

func (args *ArgReader) ID(tile bool) (res string, err error) {
	result := strings.Builder{}
	began := false
	for {
		var char rune
		if began {
			char, err = args.Read()
		} else {
			char, err = args.Significant()
		}
		if err == nil {
			if began {
				if !identifierOk.TestRune(rune(char)) {
					args.Unread()
					err = io.EOF
				}
				if tile {
					if char == '-' {
						char = '_'
					}
				}
			} else {
				if !identifierOkFirst.TestRune(rune(char)) {
					args.Unread()
					err = io.EOF
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				if began {
					return result.String(), nil
				} else {
					return "", io.EOF
				}
			}
			return "", err
		}
		if char == '"' || char == '\'' {
			if began {
				args.Unread()
				return result.String(), nil
			} else {
				res, err = args.Quoted(char)
				return res, err
			}
		}
		began = true
		result.WriteRune(char)
	}
}
func (args *ArgReader) Assign(k rune) (r any, e error) {
	args.Skipspace()
	err := args.Take(k)
	if err != nil {
		if err == io.EOF {
			return true, nil
		}
		return nil, err
	}
	return args.Any()
}
func (args *ArgReader) TakePath() (r string, e error) {
	args.Skipspace()
	if r := args.Peek(); r == '"' || r == '\'' {
		return args.Quoted(r)
	}
	result := strings.Builder{}
	for {
		char, err := args.Read()
		if err != nil {
			return "", err
		}
		if char == ' ' {
			args.Unread()
			return result.String(), nil
		}
		result.WriteRune(char)
	}
}
func (args *ArgReader) TakeFile() (res map[string]any, e error) {
	file, err := args.TakePath()
	if err != nil {
		return nil, err
	}
	e = foreign.Unmarshal(file, nil, &res)
	return
}
func (args *ArgReader) Set() (res map[string]any, e error) {
	if args.TakeIf('@') {
		return args.TakeFile()
	} else if args.TakeIf('{') {
		return args.Object()
	}
	res = map[string]any{}
	for {
		if args.TakeIf('-') {
			args.TakeIf('-')
		}
		key, err := args.ID(true)
		if err != nil {
			if err == io.EOF {
				return res, nil
			}
			return nil, err
		}
		val, err := args.Assign('=')
		if err != nil {
			if err == io.EOF {
				res[key] = true
				if res["args"] == nil {
					res["args"] = []string{key}
				}
				return res, nil
			}
			return nil, err
		}
		res[key] = val
	}
}

func (args *ArgReader) Unmarshal(arg any) (e error) {
	defer func() {
		if e == io.EOF {
			e = nil
		}
	}()

	switch arg := arg.(type) {
	case *struct{}:
		return
	case **ArgReader:
		*arg = args
		return
	case *string:
		*arg = args.Remains()
		return
	case *[]byte:
		*arg = []byte(args.Remains())
		return
	case *[]string:
		*arg = []string{}
		for {
			if args.Finished() {
				return nil
			}
			val, err := args.Text()
			if err != nil {
				return err
			}
			*arg = append(*arg, val)
		}
	default:
	}

	common, er := args.Set()
	if er != nil {
		return er
	}
	js, er := json.Marshal(common)
	if er != nil {
		return er
	}
	return json.Unmarshal(js, arg)
}
func (args *ArgReader) Object() (res map[string]any, e error) {
	if args.TakeIf('@') {
		res, e = args.TakeFile()
		if e != nil {
			return nil, e
		}
	} else {
		res = map[string]any{}
	}
	for {
		if args.TakeIf('}') {
			return res, nil
		}
		key, err := args.ID(true)
		if err != nil {
			return nil, err
		}
		val, err := args.Assign(':')
		if err != nil {
			return nil, err
		}
		res[key] = val
	}
}
func (args *ArgReader) Array() (res []any, e error) {
	res = []any{}
	for {
		ch, err := args.Significant()
		if err != nil {
			return nil, err
		}
		if ch == ']' {
			return res, nil
		}
		args.Unread()
		val, err := args.Any()
		if err != nil {
			return nil, err
		}
		res = append(res, val)
	}
}

var numericRunes = util.NewRuneSet("0123456789-+.eExX")

func (args *ArgReader) Number() (res float64, e error) {
	result := args.TakeAnyN(&numericRunes)
	if result == "" {
		return 0, fmt.Errorf("expected number")
	}
	return strconv.ParseFloat(result, 64)
}
func (args *ArgReader) Any() (res any, e error) {
	char, err := args.Significant()
	if err != nil {
		return nil, err
	}
	switch char {
	case '{':
		return args.Object()
	case '[':
		return args.Array()
	case '"', '\'':
		return args.Quoted(char)
	case '-', '+', '.', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		args.Unread()
		return args.Number()
	default:
		if identifierOkFirst.TestRune(rune(char)) {
			args.Unread()
			return args.ID(false)
		}
		return nil, fmt.Errorf("unexpected character %c", char)
	}
}

func NewArgReader(args string) *ArgReader {
	return &ArgReader{buf: []rune(args)}
}
