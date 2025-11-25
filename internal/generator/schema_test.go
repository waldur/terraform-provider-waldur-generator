package generator

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestExtractFields(t *testing.T) {
	// Create a test schema
	schema := &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Type:     &openapi3.Types{"object"},
			Required: []string{"name"},
			Properties: openapi3.Schemas{
				"name": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "Resource name",
					},
				},
				"description": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "Resource description",
					},
				},
				"count": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"integer"},
						Description: "Item count",
					},
				},
				"enabled": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"boolean"},
						Description: "Whether enabled",
					},
				},
			},
		},
	}

	fields, err := ExtractFields(schema)
	if err != nil {
		t.Fatalf("ExtractFields failed: %v", err)
	}

	if len(fields) != 4 {
		t.Fatalf("Expected 4 fields, got %d", len(fields))
	}

	// Check field mapping
	fieldMap := make(map[string]FieldInfo)
	for _, f := range fields {
		fieldMap[f.Name] = f
	}

	// Verify name field
	if f, ok := fieldMap["name"]; !ok {
		t.Error("name field not found")
	} else {
		if f.Type != "string" {
			t.Errorf("name type: expected string, got %s", f.Type)
		}
		if !f.Required {
			t.Error("name should be required")
		}
		if f.GoType != "types.String" {
			t.Errorf("name GoType: expected types.String, got %s", f.GoType)
		}
	}

	// Verify count field
	if f, ok := fieldMap["count"]; !ok {
		t.Error("count field not found")
	} else {
		if f.Type != "integer" {
			t.Errorf("count type: expected integer, got %s", f.Type)
		}
		if f.Required {
			t.Error("count should not be required")
		}
		if f.GoType != "types.Int64" {
			t.Errorf("count GoType: expected types.Int64, got %s", f.GoType)
		}
	}

	// Verify enabled field
	if f, ok := fieldMap["enabled"]; !ok {
		t.Error("enabled field not found")
	} else {
		if f.Type != "boolean" {
			t.Errorf("enabled type: expected boolean, got %s", f.Type)
		}
		if f.GoType != "types.Bool" {
			t.Errorf("enabled GoType: expected types.Bool, got %s", f.GoType)
		}
	}
}

func TestExtractFields_EmptySchema(t *testing.T) {
	fields, err := ExtractFields(nil)
	if err != nil {
		t.Fatalf("ExtractFields(nil) failed: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("Expected 0 fields for nil schema, got %d", len(fields))
	}
}
