package foreign

import (
	"bytes"
	"context"
	"encoding"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/pelletier/go-toml"
)

type Unmarshaler func(input []byte, out any) (err error)

func (u Unmarshaler) Unmarshal(ctx context.Context, path string, params any, out any) (err error) {
	var data []byte
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		req, err := http.NewRequestWithContext(ctx, "GET", path, nil)
		if err != nil {
			return err
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return errors.New("http error: " + res.Status)
		}
		buf := bytes.Buffer{}
		_, err = buf.ReadFrom(res.Body)
		if err != nil {
			return err
		}
		data = buf.Bytes()
	} else {
		data, err = os.ReadFile(path)
		if err != nil {
			return
		}
	}
	return u(data, out)
}
func (u Unmarshaler) Run(ctx context.Context, script string, args ...string) (cmd *exec.Cmd, err error) {
	return nil, errors.New("run not implemented for unmarshaler")
}

var Unmarshalers = map[string]Unmarshaler{}

func (u Unmarshaler) Register(ext ...string) {
	for _, l := range ext {
		Unmarshalers[l] = u
	}
	Register(u, nil, ext)
}

type InlineUnmarshaler interface {
	UnmarshalInline(text string) (err error)
}

func UnmarshalText(input []byte, out any) (err error) {
	textUnmarshal, ok := out.(encoding.TextUnmarshaler)
	if !ok {
		inlineUnmarshal, ok := out.(InlineUnmarshaler)
		if ok {
			err = inlineUnmarshal.UnmarshalInline(string(input))
			return
		}
		err = errors.New("cannot unmarshal text into " + reflect.TypeOf(out).String())
	} else {
		err = textUnmarshal.UnmarshalText(input)
	}
	return
}

func init() {
	Unmarshaler(json.Unmarshal).Register(".json")
	Unmarshaler(yaml.Unmarshal).Register(".yaml", ".yml")
	Unmarshaler(toml.Unmarshal).Register(".toml")
	Unmarshaler(UnmarshalText).Register(".txt")
}
