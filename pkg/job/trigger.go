package job

import (
	"encoding/json"

	"github.com/can1357/gosu/pkg/automarshal"
)

type ITrigger interface {
	Listen(callback func()) (remove func())
}

type Trigger struct {
	ITrigger
	automarshal.ID
}

var TriggerRegistry = automarshal.NewRegistry[Trigger, ITrigger]()

func (t *Trigger) UnmarshalJSON(data []byte) error { return TriggerRegistry.Unmarshal(t, data) }
func (t Trigger) MarshalJSON() ([]byte, error)     { return TriggerRegistry.Marshal(t) }

type TriggerAny struct {
	List []Trigger `json:"list"`
}

func (t TriggerAny) Listen(callback func()) (remove func()) {
	var removes []func()
	for _, t := range t.List {
		removes = append(removes, t.Listen(callback))
	}
	return func() {
		for _, r := range removes {
			r()
		}
	}
}

func init() {

	TriggerRegistry.Define("any", TriggerAny{})
	TriggerRegistry.RegisterNonObject('[', func(w *Trigger, b []byte) error {
		var t TriggerAny
		if err := json.Unmarshal(b, &t.List); err != nil {
			return err
		}
		w.ITrigger = t
		w.Kind = "many"
		return nil
	})
}
