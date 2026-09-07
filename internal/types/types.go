package types

type ChatRequest struct {
	Message string `json:"message"`
}

type StartVMArguments struct {
	VMID int64 `json:"vmid"`
}

type ChatResponse struct {
	Status         string           `json:"status"`
	Message        string           `json:"message"`
	Tool           string           `json:"tool"`
	Arguments      StartVMArguments `json:"arguments"`
	Confidence     float64          `json:"confidence"`
	UpstreamStatus int              `json:"upstreamStatus"`
	RequestID      string           `json:"requestId"`
}

type ErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

type HealthResponse struct {
	Status string `json:"status"`
}

type DependencyStatus struct {
	Needle                string `json:"needle"`
	InfrastructureControl string `json:"infrastructureControl"`
}

type ReadyResponse struct {
	Status       string           `json:"status"`
	Dependencies DependencyStatus `json:"dependencies"`
	RequestID    string           `json:"requestId"`
}
