package common

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

	fields, err := ExtractFields(SchemaConfig{}, schema, false)
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
	fields, err := ExtractFields(SchemaConfig{}, nil, false)
	if err != nil {
		t.Fatalf("ExtractFields(nil, false) failed: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("Expected 0 fields for nil schema, got %d", len(fields))
	}
}

func TestExtractFields_Enum(t *testing.T) {
	schema := &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
			Properties: openapi3.Schemas{
				"status": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Enum:        []interface{}{"active", "pending", "archived"},
						Description: "Status field with enum values",
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
	if f.Name != "status" {
		t.Errorf("Expected field name 'status', got %s", f.Name)
	}
	if f.Type != "string" {
		t.Errorf("Expected type 'string', got %s", f.Type)
	}
	if len(f.Enum) != 3 {
		t.Fatalf("Expected 3 enum values, got %d", len(f.Enum))
	}
	expectedEnums := map[string]bool{"active": true, "pending": true, "archived": true}
	for _, e := range f.Enum {
		if !expectedEnums[e] {
			t.Errorf("Unexpected enum value: %s", e)
		}
	}
}

func TestExtractFields_ListOfStrings(t *testing.T) {
	schema := &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
			Properties: openapi3.Schemas{
				"tags": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"array"},
						Description: "List of tags",
						Items: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"string"},
							},
						},
					},
				},
			},
		},
	}

	cfg := SchemaConfig{
		SetFields: map[string]bool{"tags": true},
	}
	fields, err := ExtractFields(cfg, schema, false)
	if err != nil {
		t.Fatalf("ExtractFields failed: %v", err)
	}

	if len(fields) != 1 {
		t.Fatalf("Expected 1 field, got %d", len(fields))
	}

	f := fields[0]
	if f.Name != "tags" {
		t.Errorf("Expected field name 'tags', got %s", f.Name)
	}
	if f.Type != "array" {
		t.Errorf("Expected type 'array', got %s", f.Type)
	}
	if f.ItemType != "string" {
		t.Errorf("Expected ItemType 'string', got %s", f.ItemType)
	}
	if f.GoType != "types.Set" {
		t.Errorf("Expected GoType 'types.Set', got %s", f.GoType)
	}
}

func TestExtractFields_ListOfObjects(t *testing.T) {
	schema := &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
			Properties: openapi3.Schemas{
				"members": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"array"},
						Description: "List of members",
						Items: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"object"},
								Properties: openapi3.Schemas{
									"username": &openapi3.SchemaRef{
										Value: &openapi3.Schema{
											Type: &openapi3.Types{"string"},
										},
									},
									"role": &openapi3.SchemaRef{
										Value: &openapi3.Schema{
											Type: &openapi3.Types{"string"},
										},
									},
								},
							},
						},
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
	if f.Name != "members" {
		t.Errorf("Expected field name 'members', got %s", f.Name)
	}
	if f.Type != "array" {
		t.Errorf("Expected type 'array', got %s", f.Type)
	}
	if f.ItemType != "object" {
		t.Errorf("Expected ItemType 'object', got %s", f.ItemType)
	}
	if f.ItemSchema == nil {
		t.Fatal("ItemSchema should not be nil for array of objects")
	}
	if len(f.ItemSchema.Properties) != 2 {
		t.Errorf("Expected 2 properties in ItemSchema, got %d", len(f.ItemSchema.Properties))
	}
}

func TestExtractFields_NestedObject(t *testing.T) {
	schema := &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
			Properties: openapi3.Schemas{
				"settings": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"object"},
						Description: "Settings object",
						Properties: openapi3.Schemas{
							"theme": &openapi3.SchemaRef{
								Value: &openapi3.Schema{
									Type: &openapi3.Types{"string"},
								},
							},
							"notifications": &openapi3.SchemaRef{
								Value: &openapi3.Schema{
									Type: &openapi3.Types{"boolean"},
								},
							},
						},
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
	if f.Name != "settings" {
		t.Errorf("Expected field name 'settings', got %s", f.Name)
	}
	if f.Type != "object" {
		t.Errorf("Expected type 'object', got %s", f.Type)
	}
	if f.GoType != "types.Object" {
		t.Errorf("Expected GoType 'types.Object', got %s", f.GoType)
	}
	if len(f.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(f.Properties))
	}

	// Verify nested properties
	propMap := make(map[string]FieldInfo)
	for _, p := range f.Properties {
		propMap[p.Name] = p
	}

	if theme, ok := propMap["theme"]; !ok {
		t.Error("theme property not found")
	} else if theme.Type != "string" {
		t.Errorf("theme type: expected string, got %s", theme.Type)
	}

	if notif, ok := propMap["notifications"]; !ok {
		t.Error("notifications property not found")
	} else if notif.Type != "boolean" {
		t.Errorf("notifications type: expected boolean, got %s", notif.Type)
	}
}
