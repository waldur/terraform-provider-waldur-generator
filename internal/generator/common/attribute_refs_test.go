package common

import "testing"

func TestCollectUniqueStructs(t *testing.T) {
	fields := []FieldInfo{
		{
			Name:        "nested",
			GoType:      TFTypeObject,
			AttrTypeRef: "NestedStruct",
			Properties: []FieldInfo{
				{Name: "field1", Type: "string"},
			},
		},
		{
			Name:   "list",
			GoType: TFTypeList,
			ItemSchema: &FieldInfo{
				GoType:      TFTypeObject,
				AttrTypeRef: "ItemStruct",
				Properties: []FieldInfo{
					{Name: "field2", Type: "integer"},
				},
			},
		},
	}

	structs := CollectUniqueStructs(fields)
	if len(structs) != 2 {
		t.Errorf("Expected 2 unique structs, got %d", len(structs))
	}

	foundNested := false
	foundItem := false
	for _, s := range structs {
		if s.AttrTypeRef == "NestedStruct" {
			foundNested = true
		}
		if s.AttrTypeRef == "ItemStruct" {
			foundItem = true
		}
	}

	if !foundNested || !foundItem {
		t.Error("Missing expected structs in CollectUniqueStructs output")
	}
}

func TestAssignMissingAttrTypeRefs(t *testing.T) {
	cfg := SchemaConfig{}
	fields := []FieldInfo{
		{
			Name:   "user",
			GoType: TFTypeObject,
			Properties: []FieldInfo{
				{Name: "username", Type: "string"},
			},
		},
	}
	seenHashes := make(map[string]string)
	seenNames := make(map[string]string)

	AssignMissingAttrTypeRefs(cfg, fields, "Base", seenHashes, seenNames)

	if fields[0].AttrTypeRef == "" {
		t.Error("AttrTypeRef was not assigned to object field")
	}

	expectedPrefix := "BaseUser"
	if fields[0].AttrTypeRef != expectedPrefix {
		t.Errorf("Expected AttrTypeRef %q, got %q", expectedPrefix, fields[0].AttrTypeRef)
	}

	// Test deduplication by hash
	fields2 := []FieldInfo{
		{
			Name:   "profile",
			GoType: TFTypeObject,
			Properties: []FieldInfo{
				{Name: "username", Type: "string"}, // Same structure as 'user'
			},
		},
	}
	AssignMissingAttrTypeRefs(cfg, fields2, "Other", seenHashes, seenNames)
	if fields2[0].AttrTypeRef != fields[0].AttrTypeRef {
		t.Errorf("Expected duplicate structure to have same AttrTypeRef %q, got %q", fields[0].AttrTypeRef, fields2[0].AttrTypeRef)
	}
}
