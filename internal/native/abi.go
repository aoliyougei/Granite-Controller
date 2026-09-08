package native

type ABI interface {
	Init(system, toolsJSON, toolIndexPath []byte) int
	Complete(text []byte, maxNewTokens int, output []byte) int
	Reset()
}

type NativeError struct {
	Code    string
	Message string
	Fatal   bool
	Cause   error
}

func (e *NativeError) Error() string { return e.Code + ": " + e.Message }
func (e *NativeError) Unwrap() error { return e.Cause }
