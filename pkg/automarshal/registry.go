package automarshal

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/can1357/gosu/pkg/foreign"
)

/*
	"id:kind"
	"id:kind": {
		details
	}
	"id:kind arg1="arg2" arg3=[arg4]
*/

// Parses a=name b=5.0 id=string id="string 2" etc into a json msg
// Must handle quoted strings with spaces, so cant use split

func ArglistToJson(args string) (json.RawMessage, error) {
	r, err := NewArgReader(args).Set()
	if err != nil {
		return nil, err
	}
	return json.Marshal(r)
}

type ID struct {
	ID      string
	Kind    string
	pending *ArgReader
}

func (id ID) Label() string {
	if id.ID == "" {
		return id.Kind
	}
	return id.ID
}

type WithDefaults interface {
	WithDefaults()
}
type WithDefaultsID interface {
	WithDefaults(ID)
}

func (id ID) String() (r string) {
	if id.ID == "" {
		return id.Kind
	}
	return fmt.Sprintf("%s:%s", id.Kind, id.ID)
}
func (id ID) MarshalText() ([]byte, error) {
	return []byte(id.String()), nil
}
func (id *ID) UnmarshalText(data []byte) (err error) {
	if len(data) == 0 {
		return nil
	}
	ids, args, _ := strings.Cut(string(data), " ")
	if before, after, found := strings.Cut(ids, ":"); found {
		id.Kind = before
		id.ID = after
	} else {
		id.Kind = ids
	}
	as := NewArgReader(strings.TrimSpace(args))
	if !as.Finished() {
		id.pending = as
	}
	return nil
}

type Registry[W any, I any] struct {
	ifaceField      reflect.StructField
	idField         reflect.StructField
	kindToUnmarshal map[string]func(id ID, msg []byte) (any, error)
	nonObject       map[byte]func(*W, []byte) error
}

var idType = reflect.TypeOf(ID{})

func getFieldWithType(t reflect.Type, typ reflect.Type) (field reflect.StructField) {
	for i := 0; i < t.NumField(); i++ {
		field = t.Field(i)
		if field.Type == typ {
			return field
		}
	}
	panic(fmt.Sprintf("type %v does not have a field of type %s", t, typ.Name()))
}
func (r *Registry[W, I]) RegisterNonObject(kind byte, factory func(*W, []byte) error) {
	r.nonObject[kind] = factory
}
func (r *Registry[W, I]) Define(kind string, defaults any) {
	ty := reflect.TypeOf(defaults)
	if ty.Kind() != reflect.Struct {
		panic("defaults must be a struct type")
	}
	ptr := reflect.PointerTo(ty)
	if !ptr.Implements(r.ifaceField.Type) {
		panic("defaults must implement the interface")
	}
	r.kindToUnmarshal[kind] = func(id ID, msg []byte) (res any, err error) {
		result := reflect.New(ty)
		result.Elem().Set(reflect.ValueOf(defaults))
		if id.pending != nil && !id.pending.Finished() {
			if result, ok := result.Interface().(foreign.InlineUnmarshaler); ok {
				if err := result.UnmarshalInline(id.pending.Remains()); err == nil {
					id.pending = nil
				}
			}
			if id.pending != nil && !id.pending.Finished() {
				set, err := id.pending.Set()
				if err != nil {
					return nil, err
				}
				if err := json.Unmarshal(msg, &set); err != nil {
					return nil, err
				} else {
					msg, err = json.Marshal(set)
					if err != nil {
						return nil, err
					}
				}
			}
		}

		if err := json.Unmarshal(msg, result.Interface()); err != nil {
			return nil, err
		}
		if result, ok := result.Interface().(WithDefaultsID); ok {
			result.WithDefaults(id)
		}
		if result, ok := result.Interface().(WithDefaults); ok {
			result.WithDefaults()
		}
		return result.Interface(), nil
	}
}

func (r *Registry[W, I]) Unmarshal(value *W, data []byte) (err error) {
	for {
		if len(data) == 0 {
			return errors.New("empty data")
		}
		if data[0] == ' ' || data[0] == '\t' || data[0] == '\n' {
			data = data[1:]
		} else {
			break
		}
	}
	if data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		if after, found := strings.CutPrefix(text, "@"); found {
			return foreign.Unmarshal(after, nil, value)
		}
		id := ID{}
		if err := id.UnmarshalText([]byte(text)); err != nil {
			return err
		}
		if factory, ok := r.kindToUnmarshal[id.Kind]; ok {
			if result, err := factory(id, []byte("{}")); err != nil {
				return err
			} else {
				reflect.ValueOf(value).Elem().FieldByIndex(r.ifaceField.Index).Set(reflect.ValueOf(result))
				reflect.ValueOf(value).Elem().FieldByIndex(r.idField.Index).Set(reflect.ValueOf(id))
				return nil
			}
		}
	}
	if data[0] != '{' {
		if factory, ok := r.nonObject[data[0]]; ok {
			return factory(value, data)
		}
		return fmt.Errorf("no unmarshaler found for %s", data)
	}

	ids := make(map[string]json.RawMessage, 1)
	if err := json.Unmarshal(data, &ids); err != nil {
		return err
	} else {
		for idstr, data := range ids {
			id := ID{}
			if id.UnmarshalText([]byte(idstr)) == nil {
				if factory, ok := r.kindToUnmarshal[id.Kind]; ok {
					if result, err := factory(id, data); err != nil {
						return err
					} else {
						reflect.ValueOf(value).Elem().FieldByIndex(r.ifaceField.Index).Set(reflect.ValueOf(result))
						reflect.ValueOf(value).Elem().FieldByIndex(r.idField.Index).Set(reflect.ValueOf(id))
						return nil
					}
				}
			}
		}
		return fmt.Errorf("no unmarshaler found for %s", data)
	}
}
func (r *Registry[W, I]) Marshal(value W) ([]byte, error) {
	iface := reflect.ValueOf(value).FieldByIndex(r.ifaceField.Index).Interface()
	id := reflect.ValueOf(value).FieldByIndex(r.idField.Index).Interface().(ID)
	return json.Marshal(map[string]any{id.String(): iface})
}

func NewRegistry[Wrapper any, IFace any]() *Registry[Wrapper, IFace] {
	reg := &Registry[Wrapper, IFace]{
		kindToUnmarshal: map[string]func(id ID, msg []byte) (any, error){},
		nonObject:       map[byte]func(*Wrapper, []byte) error{},
		ifaceField:      getFieldWithType(reflect.TypeOf((*Wrapper)(nil)).Elem(), reflect.TypeOf((*IFace)(nil)).Elem()),
		idField:         getFieldWithType(reflect.TypeOf((*Wrapper)(nil)).Elem(), idType),
	}
	return reg
}
