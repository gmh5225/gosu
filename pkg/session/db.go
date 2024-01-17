package session

import (
	"bytes"
	"encoding/json"
	"log"

	"github.com/dgraph-io/badger/v4"
	"github.com/samber/lo"
)

// Serialization.
func marshal(into *bytes.Buffer, v any) {
	switch v := v.(type) {
	case string:
		lo.Must(into.Write([]byte(v)))
		return
	case []byte:
		lo.Must(into.Write(v))
		return
	default:
		enc := json.NewEncoder(into)
		if err := enc.Encode(v); err != nil {
			log.Fatal("Error encoding struct:", err)
		}
	}
}
func unmarshal(v any, b []byte) error {
	switch v := v.(type) {
	case *string:
		*v = string(b)
		return nil
	case *[]byte:
		*v = b
		return nil
	}
	return json.Unmarshal(b, v)
}

// Collections.
type Collection[K any, V any] struct {
	DB     *badger.DB
	Prefix []byte
}

func (c *Collection[K, V]) MarshalKey(k K) []byte {
	buf := bytes.NewBuffer(c.Prefix)
	marshal(buf, k)
	return buf.Bytes()
}
func (c *Collection[K, V]) MarshalValue(v V) []byte {
	var buf bytes.Buffer
	marshal(&buf, v)
	return buf.Bytes()
}
func (c *Collection[K, V]) UnmarshalKey(k *K, b []byte) error {
	if len(b) < len(c.Prefix) {
		return badger.ErrKeyNotFound
	}
	return unmarshal(k, b[len(c.Prefix):])
}
func (c *Collection[K, V]) UnmarshalValue(v *V, b []byte) error {
	return unmarshal(v, b)
}

func (c *Collection[K, V]) Count() (count int) {
	c.DB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(c.Prefix); it.ValidForPrefix(c.Prefix); it.Next() {
			count++
		}
		return nil
	})
	return
}
func (c *Collection[K, V]) Range(f func(key K, item V) bool) {
	c.DB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(c.Prefix); it.ValidForPrefix(c.Prefix); it.Next() {
			item := it.Item()
			var key K
			if err := c.UnmarshalKey(&key, item.Key()); err != nil {
				continue
			}

			var value V
			var marshalError error
			err := item.Value(func(val []byte) error {
				marshalError = c.UnmarshalValue(&value, val)
				return nil
			})
			if err != nil {
				return err
			}
			if marshalError == nil && !f(key, value) {
				break
			}
		}
		return nil
	})
}
func (c *Collection[K, V]) Insert(key K, value V) (inserted bool) {
	k, v := c.MarshalKey(key), c.MarshalValue(value)
	lo.Must0(c.DB.Update(func(txn *badger.Txn) error {
		_, err := txn.Get(k)
		if err == badger.ErrKeyNotFound {
			inserted = true
			return txn.Set(k, v)
		}
		return err
	}))
	return
}
func (c *Collection[K, V]) Upsert(key K, cb func(value *V) V) {
	k := c.MarshalKey(key)
	lo.Must0(c.DB.Update(func(txn *badger.Txn) error {
		item, err := txn.Get(k)
		if err == nil {
			var value V
			found := nil == item.Value(func(val []byte) error {
				return c.UnmarshalValue(&value, val)
			})
			if found {
				return txn.Set(k, c.MarshalValue(cb(&value)))
			}
		} else if err == badger.ErrKeyNotFound {
			return txn.Set(k, c.MarshalValue(cb(nil)))
		}
		return err
	}))
}
func (c *Collection[K, V]) Replace(key K, value V) (prev V, existed bool) {
	k, v := c.MarshalKey(key), c.MarshalValue(value)
	lo.Must0(c.DB.Update(func(txn *badger.Txn) error {
		item, err := txn.Get(k)
		if err == nil {
			existed = nil == item.Value(func(val []byte) error {
				return c.UnmarshalValue(&prev, val)
			})
		} else if err != badger.ErrKeyNotFound {
			return err
		}
		return txn.Set(k, v)
	}))
	return
}

func (c *Collection[K, V]) Delete(key K) {
	k := c.MarshalKey(key)
	c.DB.Update(func(txn *badger.Txn) error {
		return txn.Delete(k)
	})
}
func (c *Collection[K, V]) Get(key K) (value V, ok bool) {
	k := c.MarshalKey(key)
	c.DB.View(func(txn *badger.Txn) error {
		item, err := txn.Get(k)
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error {
			ok = c.UnmarshalValue(&value, v) == nil
			return nil
		})
	})
	return
}
func (c *Collection[K, V]) Has(key K) bool {
	k := c.MarshalKey(key)
	err := c.DB.View(func(txn *badger.Txn) error {
		_, err := txn.Get(k)
		return err
	})
	return err == nil
}
func (c *Collection[K, V]) Clear() {
	c.DB.Update(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(c.Prefix); it.ValidForPrefix(c.Prefix); it.Next() {
			err := txn.Delete(it.Item().Key())
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *Collection[K, V]) Open(db *badger.DB, prefix string) {
	c.DB = db
	c.Prefix = []byte(prefix + ":")
}
