package common

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// An offering's `options.options` is `additionalProperties: {$ref: OptionField}` --
// a map keyed by field name whose values are order-form field descriptors.
func TestExtractFields_MapOfObjects(t *testing.T) {
	optionField := &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
			Properties: openapi3.Schemas{
				"type":  {Value: &openapi3.Schema{Type: &openapi3.Types{"string"}}},
				"label": {Value: &openapi3.Schema{Type: &openapi3.Types{"string"}}},
			},
		},
	}

	schema := &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
			Properties: openapi3.Schemas{
				"options": {
					Value: &openapi3.Schema{
						Type:                 &openapi3.Types{"object"},
						Description:          "Order form fields",
						AdditionalProperties: openapi3.AdditionalProperties{Schema: optionField},
					},
				},
			},
		},
	}

	fields, err := ExtractFields(SchemaConfig{}, schema, false)
	if err != nil {
		t.Fatalf("ExtractFields failed: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("Expected 1 field, got %d", len(fields))
	}

	f := fields[0]
	if f.GoType != TFTypeMap {
		t.Errorf("Expected GoType %q, got %q", TFTypeMap, f.GoType)
	}
	if f.SDKType != "map[string]interface{}" {
		t.Errorf("Expected SDKType 'map[string]interface{}', got %q", f.SDKType)
	}
	if !IsAnyMap(f) {
		t.Errorf("Expected IsAnyMap to be true for a map of objects")
	}
}

func TestIsAnyMap(t *testing.T) {
	tests := []struct {
		name     string
		field    FieldInfo
		expected bool
	}{
		{
			name:     "map of objects",
			field:    FieldInfo{Type: OpenAPITypeObject, GoType: TFTypeMap, ItemType: OpenAPITypeObject},
			expected: true,
		},
		{
			name:     "map of arrays",
			field:    FieldInfo{Type: OpenAPITypeObject, GoType: TFTypeMap, ItemType: OpenAPITypeArray},
			expected: true,
		},
		{
			name:     "generic map with no declared value type",
			field:    FieldInfo{Type: OpenAPITypeObject, GoType: TFTypeMap, ItemType: ""},
			expected: true,
		},
		{
			name:     "map of strings",
			field:    FieldInfo{Type: OpenAPITypeObject, GoType: TFTypeMap, ItemType: OpenAPITypeString},
			expected: false,
		},
		{
			name:     "map of numbers",
			field:    FieldInfo{Type: OpenAPITypeObject, GoType: TFTypeMap, ItemType: OpenAPITypeNumber},
			expected: false,
		},
		{
			name:     "map of booleans",
			field:    FieldInfo{Type: OpenAPITypeObject, GoType: TFTypeMap, ItemType: OpenAPITypeBoolean},
			expected: false,
		},
		{
			name:     "map of integers",
			field:    FieldInfo{Type: OpenAPITypeObject, GoType: TFTypeMap, ItemType: OpenAPITypeInteger},
			expected: false,
		},
		{
			name:     "not a map",
			field:    FieldInfo{Type: OpenAPITypeString},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.field
			CalculateSDKType(&f)
			if got := IsAnyMap(f); got != tt.expected {
				t.Errorf("IsAnyMap(%s) = %v, want %v (SDKType %q)", tt.name, got, tt.expected, f.SDKType)
			}
		})
	}
}
