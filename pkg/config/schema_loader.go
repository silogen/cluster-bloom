package config

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed bloom.yaml.schema.yaml
var schemaData []byte

// FieldDefinition represents a field in the YAML schema
type FieldDefinition struct {
	Type        string      `yaml:"type"`
	Default     any         `yaml:"default"`
	Desc        string      `yaml:"desc"`
	Required    string      `yaml:"required"`
	Applicable  string      `yaml:"applicable"`
	Section     string      `yaml:"section"`
	Values      []string    `yaml:"values"`
	Examples    []string    `yaml:"examples"`
	Sequence    []any       `yaml:"sequence"`
}

// YAMLSchema represents the structure of bloom.yaml.schema.yaml
type YAMLSchema struct {
	Schema struct {
		Mapping map[string]FieldDefinition `yaml:"mapping"`
	} `yaml:"schema"`
	Types map[string]struct {
		Pattern      string `yaml:"pattern"`
		ErrorMessage string `yaml:"errorMessage"`
	} `yaml:"types"`
}

// LoadSchema loads the embedded schema
func LoadSchema() ([]Argument, error) {
	var schema YAMLSchema
	if err := yaml.Unmarshal(schemaData, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	var arguments []Argument

	for key, field := range schema.Schema.Mapping {
		arg := Argument{
			Key:         key,
			Type:        mapType(field.Type),
			Default:     field.Default,
			Description: field.Desc,
			Section:     field.Section,
		}

		// Handle enum type
		if field.Type == "enum" {
			arg.Options = field.Values
		}

		// Map dependencies from required/applicable
		arg.Dependencies = mapDependencies(field.Required, field.Applicable)
		arg.Required = field.Required != ""

		// Get pattern and error message from type definition
		if typeDef, ok := schema.Types[field.Type]; ok {
			arg.Pattern = typeDef.Pattern
			arg.PatternTitle = typeDef.ErrorMessage
		}

		arguments = append(arguments, arg)
	}

	// Sort arguments to maintain consistent order
	// Order by section then by key
	sortArguments(arguments)

	return arguments, nil
}

// mapType converts YAML schema types to webui types
func mapType(yamlType string) string {
	switch yamlType {
	case "bool":
		return "bool"
	case "str":
		return "string"
	case "seq":
		return "array"
	case "enum":
		return "enum"
	default:
		// Keep custom types (domain, ipv4, url, etc.) for pattern validation
		return yamlType
	}
}

// mapDependencies converts "required: when(...)" or "applicable: when(...)" to Dependencies string
func mapDependencies(required, applicable string) string {
	condition := required
	if condition == "" {
		condition = applicable
	}
	if condition == "" {
		return ""
	}

	// Parse "when(FIRST_NODE == true)" to "FIRST_NODE=true"
	// Parse "when(USE_CERT_MANAGER == false && FIRST_NODE == true)" to "USE_CERT_MANAGER=false,FIRST_NODE=true"
	re := regexp.MustCompile(`when\((.+)\)`)
	matches := re.FindStringSubmatch(condition)
	if len(matches) < 2 {
		return ""
	}

	conditionExpr := matches[1]

	// Split by && and process each condition
	conditions := strings.Split(conditionExpr, " && ")
	var deps []string
	for _, cond := range conditions {
		// Parse "KEY == value" to "KEY=value"
		cond = strings.TrimSpace(cond)
		parts := strings.Split(cond, " == ")
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			deps = append(deps, key+"="+value)
		}
	}

	return strings.Join(deps, ",")
}

// sortArguments sorts arguments by section order and then by key
func sortArguments(args []Argument) {
	sectionOrder := map[string]int{
		"📋 Basic Configuration":          0,
		"🔗 Additional Node Configuration": 1,
		"💾 Storage Configuration":        2,
		"🔒 SSL/TLS Configuration":        3,
		"⚙️ Advanced Configuration":       4,
		"💻 Command Line Options":         5,
	}

	// Simple bubble sort (good enough for ~26 items)
	for i := 0; i < len(args); i++ {
		for j := i + 1; j < len(args); j++ {
			iOrder := sectionOrder[args[i].Section]
			jOrder := sectionOrder[args[j].Section]

			if iOrder > jOrder || (iOrder == jOrder && args[i].Key > args[j].Key) {
				args[i], args[j] = args[j], args[i]
			}
		}
	}
}

// Schema returns all bloom.yaml argument definitions
// This now loads from the YAML schema file
func Schema() []Argument {
	args, err := LoadSchema()
	if err != nil {
		// Fallback to empty array on error
		// In production, this should be logged
		return []Argument{}
	}
	return args
}
