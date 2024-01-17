package task

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"sync"
)

type Whiteboard struct {
	data   *sync.Map // string -> json.RawMessage
	prefix string
}

func (w Whiteboard) Get(key string, out any) error {
	if w.data == nil {
		return errors.New("whiteboard not found")
	}
	msg, ok := w.data.Load(w.prefix + key)
	if !ok {
		return errors.New("field not found")
	}
	return json.Unmarshal(msg.([]byte), out)

}
func (w Whiteboard) Set(key string, value any) {
	if w.data == nil {
		return
	}
	msg, err := json.Marshal(value)
	if err != nil {
		log.Fatal(err)
	}
	w.data.Store(w.prefix+key, msg)
}
func (w Whiteboard) Clear() {
	if w.data == nil {
		return
	}
	w.data.Range(func(key, value any) bool {
		if k, ok := key.(string); ok {
			if strings.HasPrefix(k, w.prefix) {
				w.data.Delete(key)
			}
		}
		return true
	})
}
func (w Whiteboard) Fork(key string) Whiteboard {
	if key == "" || w.data == nil {
		return w
	}
	return Whiteboard{data: w.data, prefix: w.prefix + key + "."}
}
func (w Whiteboard) WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, whiteboardKey{}, w)
}

func NewWhiteboard() Whiteboard {
	return Whiteboard{data: new(sync.Map)}
}

type whiteboardKey struct{}

func WhiteboardFromContext(ctx context.Context) Whiteboard {
	if ctx == nil {
		return NewWhiteboard()
	}
	if w, ok := ctx.Value(whiteboardKey{}).(Whiteboard); ok {
		return w
	}
	return NewWhiteboard()
}
