package native

type State string

const (
	StateLoading State = "loading"
	StateReady   State = "ready"
	StateFailed  State = "failed"
)
