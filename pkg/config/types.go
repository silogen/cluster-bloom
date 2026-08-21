package config

// Config represents the bloom.yaml configuration
type Config map[string]any

// SchemaResponse is the JSON response for /api/schema
type SchemaResponse struct {
	Arguments   []Argument      `json:"arguments"`
	Constraints []constraintDef `json:"constraints"`
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

// DetectHardwareResponse is the JSON response for /api/detect-hardware
type DetectHardwareResponse struct {
	AIMHardwareFamily string   `json:"aim_hardware_family"`
	GPUFamilies       []string `json:"gpu_families,omitempty"`
	EPYCModel         string   `json:"epyc_model,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
}
