package job

type TriggerBoot struct {
}

func (t TriggerBoot) Listen(callback func()) (remove func()) {
	go callback()
	return func() {}
}
func (h *TriggerBoot) UnmarshalInline(text string) (err error) {
	return
}
func init() {
	TriggerRegistry.Define("boot", TriggerBoot{})
}
