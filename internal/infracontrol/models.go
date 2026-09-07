package infracontrol

type StartResult struct {
	UpstreamStatus    int
	UpstreamRequestID string
}

type upstreamError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}
