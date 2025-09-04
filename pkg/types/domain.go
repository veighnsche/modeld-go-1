package types

// Model represents a discoverable or loadable LLM model on disk.
type Model struct {
    ID     string `json:"id"`
    Name   string `json:"name"`
    Path   string `json:"path"`
    Quant  string `json:"quant"`
    Family string `json:"family,omitempty"`
}
