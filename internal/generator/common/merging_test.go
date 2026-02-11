package common

import (
	"testing"
)

func TestMergeFields(t *testing.T) {
	primary := []FieldInfo{
		{Name: "field1", Type: "string", Required: true},
		{Name: "field2", Type: "integer", Required: false},
	}
	secondary := []FieldInfo{
		{Name: "field2", Type: "integer", Required: true, ReadOnly: true},
		{Name: "field3", Type: "boolean"},
	}

	merged := MergeFields(primary, secondary)

	if len(merged) != 3 {
		t.Fatalf("Expected 3 fields, got %d", len(merged))
	}

	fieldMap := make(map[string]FieldInfo)
	for _, f := range merged {
		fieldMap[f.Name] = f
	}

	// field1 from primary should be present
	if f, ok := fieldMap["field1"]; !ok || !f.Required {
		t.Error("field1 missing or incorrect")
	}

	// field2 should be merged
	if f, ok := fieldMap["field2"]; !ok {
		t.Error("field2 missing")
	} else {
		if !f.ReadOnly {
			t.Error("field2 should be ReadOnly from secondary")
		}
		if f.Required {
			t.Error("field2 should preserve its Required status from primary (merged logic: primary takes precedence for properties)")
		}
	}

	// field3 from secondary should be present
	if _, ok := fieldMap["field3"]; !ok {
		t.Error("field3 missing")
	}
}

func TestMergeFields_Recursive(t *testing.T) {
	primary := []FieldInfo{
		{
			Name:   "nested",
			GoType: TFTypeObject,
			Properties: []FieldInfo{
				{Name: "sub1", Type: "string"},
			},
		},
	}
	secondary := []FieldInfo{
		{
			Name:   "nested",
			GoType: TFTypeObject,
			Properties: []FieldInfo{
				{Name: "sub2", Type: "integer"},
			},
		},
	}

	merged := MergeFields(primary, secondary)
	if len(merged) != 1 {
		t.Fatal("Expected 1 merged field")
	}

	nested := merged[0]
	if len(nested.Properties) != 2 {
		t.Errorf("Expected 2 nested properties, got %d", len(nested.Properties))
	}
}

func TestMergeOrderFields(t *testing.T) {
	input := []FieldInfo{
		{Name: "plan", Type: "string"},
	}
	output := []FieldInfo{
		{Name: "status", Type: "string"},
	}

	merged := MergeOrderFields(input, output)

	fieldMap := make(map[string]FieldInfo)
	for _, f := range merged {
		fieldMap[f.Name] = f
	}

	// Check for project and offering (ensured by MergeOrderFields)
	if _, ok := fieldMap["project"]; !ok {
		t.Error("project field missing")
	}
	if _, ok := fieldMap["offering"]; !ok {
		t.Error("offering field missing")
	}

	// plan should be present from input
	if _, ok := fieldMap["plan"]; !ok {
		t.Error("plan field missing")
	}

	// status should be present from output and marked ReadOnly
	if f, ok := fieldMap["status"]; !ok {
		t.Error("status field missing")
	} else if !f.ReadOnly {
		t.Error("status field should be ReadOnly")
	}
}
