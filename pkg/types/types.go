package types

type Model struct {
    ID     string `json:"id"`
    Name   string `json:"name"`
    Path   string `json:"path"`
    Quant  string `json:"quant"`
    Family string `json:"family,omitempty"`
}

// InferRequest represents an inference request payload.
type InferRequest struct {
    Model  string `json:"model,omitempty"`
    Prompt string `json:"prompt"`
    Stream bool   `json:"stream,omitempty"`
}
