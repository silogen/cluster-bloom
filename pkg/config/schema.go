package config

// Argument represents a configuration field in bloom.yaml
type Argument struct {
	Key          string   `json:"key"`
	Type         string   `json:"type"`
	Default      any      `json:"default"`
	Description  string   `json:"description"`
	Options      []string `json:"options,omitempty"`
	Dependencies string   `json:"dependencies,omitempty"`
	Required     bool     `json:"required"`
	Section      string   `json:"section,omitempty"`
	Pattern      string   `json:"pattern,omitempty"`      // HTML5 validation pattern
	PatternTitle string   `json:"patternTitle,omitempty"` // Custom validation error message
	Sequence     []SequenceItem `json:"sequence,omitempty"` // Sequence validation rules
}

// SequenceItem represents validation rules for sequence items
type SequenceItem struct {
	Type         string `json:"type"`
	Pattern      string `json:"pattern"`
	PatternTitle string `json:"patternTitle"`
}
