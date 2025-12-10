package config

// Config represents the bloom.yaml configuration
type Config map[string]any

// SchemaResponse is the JSON response for /api/schema
type SchemaResponse struct {
	Arguments []Argument `json:"arguments"`
}

// ValidateRequest is the JSON request for /api/validate
type ValidateRequest struct {
	Config Config `json:"config"`
}

// ValidateResponse is the JSON response for /api/validate
type ValidateResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// GenerateRequest is the JSON request for /api/generate
type GenerateRequest struct {
	Config Config `json:"config"`
}

// GenerateResponse is the JSON response for /api/generate
type GenerateResponse struct {
	YAML string `json:"yaml"`
}

// SaveRequest is the JSON request for /api/save
type SaveRequest struct {
	Config   Config `json:"config"`
	Filename string `json:"filename"`
}
