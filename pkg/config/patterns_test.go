package config

import (
	"regexp"
	"testing"

	"gopkg.in/yaml.v3"
)

// TypeDefinition represents a type definition from the schema
type TypeDefinition struct {
	Type     string `yaml:"type"`
	Pattern  string `yaml:"pattern"`
	Desc     string `yaml:"desc"`
	Examples struct {
		Valid   []string `yaml:"valid"`
		Invalid []string `yaml:"invalid"`
	} `yaml:"examples"`
}

// SchemaFile represents the structure of bloom.yaml.schema.yaml
type SchemaFile struct {
	Types map[string]TypeDefinition `yaml:"types"`
}

var schemaFile *SchemaFile

// loadSchemaFile loads the embedded schema data
func loadSchemaFile(t *testing.T) *SchemaFile {
	if schemaFile != nil {
		return schemaFile
	}

	// Use the embedded schema data from schema_loader.go
	var schema SchemaFile
	if err := yaml.Unmarshal(schemaData, &schema); err != nil {
		t.Fatalf("Failed to parse schema file: %v", err)
	}

	schemaFile = &schema
	return schemaFile
}

// getPattern returns the compiled pattern for a type
func getPattern(t *testing.T, typeName string) *regexp.Regexp {
	schema := loadSchemaFile(t)
	typeDef, ok := schema.Types[typeName]
	if !ok {
		t.Fatalf("Type %q not found in schema", typeName)
	}

	// YAML parsing converts \\ to \ so the pattern is already properly escaped for regex
	return regexp.MustCompile(typeDef.Pattern)
}

// getExamples returns valid and invalid examples for a type
func getExamples(t *testing.T, typeName string) (valid []string, invalid []string) {
	schema := loadSchemaFile(t)
	typeDef, ok := schema.Types[typeName]
	if !ok {
		t.Fatalf("Type %q not found in schema", typeName)
	}
	return typeDef.Examples.Valid, typeDef.Examples.Invalid
}

// testPatternWithExamples is a generic test function for patterns
func testPatternWithExamples(t *testing.T, typeName string) {
	pattern := getPattern(t, typeName)
	validCases, invalidCases := getExamples(t, typeName)

	for _, tc := range validCases {
		if !pattern.MatchString(tc) {
			t.Errorf("Expected %s pattern to match %q", typeName, tc)
		}
	}

	for _, tc := range invalidCases {
		if pattern.MatchString(tc) {
			t.Errorf("Expected %s pattern to reject %q", typeName, tc)
		}
	}
}

func TestDomainPattern(t *testing.T) {
	testPatternWithExamples(t, "domain")
}

func TestDomainListPattern(t *testing.T) {
	testPatternWithExamples(t, "domainList")
}

func TestIPv4Pattern(t *testing.T) {
	testPatternWithExamples(t, "ipv4")
}

func TestURLPattern(t *testing.T) {
	testPatternWithExamples(t, "url")
}

func TestDevicePathPattern(t *testing.T) {
	testPatternWithExamples(t, "devicePath")
}

func TestDiskListPattern(t *testing.T) {
	testPatternWithExamples(t, "diskList")
}

func TestCertFilePathPattern(t *testing.T) {
	testPatternWithExamples(t, "certFilePath")
}

func TestKeyFilePathPattern(t *testing.T) {
	testPatternWithExamples(t, "keyFilePath")
}

func TestRKE2VersionPattern(t *testing.T) {
	testPatternWithExamples(t, "rke2Version")
}

// TestAllTypesHaveExamples ensures every type in the schema has both valid and invalid examples
func TestAllTypesHaveExamples(t *testing.T) {
	schemaFile := loadSchemaFile(t)

	for typeName, typeDef := range schemaFile.Types {
		if len(typeDef.Examples.Valid) == 0 {
			t.Errorf("Type %s has no valid examples", typeName)
		}
		if len(typeDef.Examples.Invalid) == 0 {
			t.Errorf("Type %s has no invalid examples", typeName)
		}
	}
}
