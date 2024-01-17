package settings

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	atomicfile "github.com/natefinch/atomic"
)

// Home directory.
var Home = sync.OnceValue(func() string {
	if home, ok := os.LookupEnv("GOSUHOME"); ok {
		return home
	} else {
		userDir, _ := os.UserHomeDir()
		return filepath.Join(userDir, ".gosu")
	}
})

// Subdirectories.
type Subdir string

const (
	LogDir  Subdir = "log"
	DataDir Subdir = "db"
)

var pathCache = sync.Map{}

func (s Subdir) Path() string {
	if path, ok := pathCache.Load(s); ok {
		return path.(string)
	} else {
		path := filepath.Join(Home(), string(s))
		os.MkdirAll(path, 0755)
		pathCache.Store(s, path)
		return path
	}
}

// Settings interface.
type Instance[T any] interface {
	Reload() (*T, error)
	Save(T) error
	Get() *T
	Path() string
}

type storage[T any] struct {
	path     string
	defaults T
	last     *T
	mu       sync.Mutex
}

func (c *storage[T]) Path() string {
	return c.path
}
func (c *storage[T]) saveLocked(value T) error {
	res, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	err = atomicfile.WriteFile(c.path, bytes.NewReader(res))
	if err == nil {
		c.last = &value
	}
	return err
}
func (c *storage[T]) reloadLocked() (result *T, err error) {
	var data []byte
	data, err = os.ReadFile(c.path)
	if err == nil {
		err = json.Unmarshal(data, &result)
		if err == nil {
			c.last = result
			return result, nil
		}
	}
	result = &c.defaults
	c.saveLocked(c.defaults)
	return
}

func (c *storage[T]) Save(value T) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saveLocked(value)
}
func (c *storage[T]) Reload() (result *T, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reloadLocked()
}
func (c *storage[T]) Get() (res *T) {
	ptr := (*T)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&c.last))))
	if ptr != nil {
		return ptr
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.last != nil {
		return c.last
	}
	res, _ = c.reloadLocked()
	return
}

func Settings[T any](defaults T) Instance[T] {
	ty := reflect.TypeOf((*T)(nil))
	name := strings.ToLower(ty.Elem().Name())
	path := filepath.Join(Home(), name+".config.json")
	store := &storage[T]{path: path, defaults: defaults}
	return store
}
