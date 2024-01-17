package settings

type service struct {
	Ephemeral bool `json:"ephemeral"`
}

var Service = Settings(service{
	Ephemeral: false,
})
