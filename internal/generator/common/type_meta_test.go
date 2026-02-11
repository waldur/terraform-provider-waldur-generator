package common

import (
	"testing"
)

func TestCalculateTypeMeta_String(t *testing.T) {
	f := FieldInfo{Type: OpenAPITypeString, GoType: TFTypeString}
	CalculateTypeMeta(&f)

	assertMeta(t, f.TypeMeta, TypeMeta{
		SchemaAttrType:  "schema.StringAttribute",
		AttrValueType:   "types.StringType",
		PlanModImport:   "stringplanmodifier",
		PlanModType:     "planmodifier.String",
		FromAPIFunc:     "common.StringPointerValue",
		ToAPIMethod:     "ValueStringPointer",
		ValidatorType:   "String",
		ValidatorImport: "stringvalidator",
	})
}

func TestCalculateTypeMeta_StringDateTime(t *testing.T) {
	f := FieldInfo{Type: OpenAPITypeString, GoType: TFTypeString, Format: "date-time"}
	CalculateTypeMeta(&f)

	if !f.TypeMeta.IsDateTime {
		t.Error("Expected IsDateTime=true for date-time string")
	}
	if f.TypeMeta.FromAPIFunc != "" {
		t.Errorf("Expected empty FromAPIFunc for date-time (uses timetypes), got %s", f.TypeMeta.FromAPIFunc)
	}
	if f.TypeMeta.SchemaAttrType != "schema.StringAttribute" {
		t.Errorf("Expected schema.StringAttribute, got %s", f.TypeMeta.SchemaAttrType)
	}
}

func TestCalculateTypeMeta_Int64(t *testing.T) {
	f := FieldInfo{Type: OpenAPITypeInteger, GoType: TFTypeInt64}
	CalculateTypeMeta(&f)

	assertMeta(t, f.TypeMeta, TypeMeta{
		SchemaAttrType:  "schema.Int64Attribute",
		AttrValueType:   "types.Int64Type",
		PlanModImport:   "int64planmodifier",
		PlanModType:     "planmodifier.Int64",
		FromAPIFunc:     "types.Int64PointerValue",
		ToAPIMethod:     "ValueInt64Pointer",
		ValidatorType:   "Int64",
		ValidatorImport: "int64validator",
	})
}

func TestCalculateTypeMeta_Bool(t *testing.T) {
	f := FieldInfo{Type: OpenAPITypeBoolean, GoType: TFTypeBool}
	CalculateTypeMeta(&f)

	assertMeta(t, f.TypeMeta, TypeMeta{
		SchemaAttrType: "schema.BoolAttribute",
		AttrValueType:  "types.BoolType",
		PlanModImport:  "boolplanmodifier",
		PlanModType:    "planmodifier.Bool",
		FromAPIFunc:    "types.BoolPointerValue",
		ToAPIMethod:    "ValueBoolPointer",
		ValidatorType:  "Bool",
	})
}

func TestCalculateTypeMeta_Float64(t *testing.T) {
	f := FieldInfo{Type: OpenAPITypeNumber, GoType: TFTypeFloat64}
	CalculateTypeMeta(&f)

	assertMeta(t, f.TypeMeta, TypeMeta{
		SchemaAttrType:  "schema.Float64Attribute",
		AttrValueType:   "types.Float64Type",
		PlanModImport:   "float64planmodifier",
		PlanModType:     "planmodifier.Float64",
		FromAPIFunc:     "types.Float64PointerValue",
		ToAPIMethod:     "ValueFloat64Pointer",
		ValidatorType:   "Float64",
		ValidatorImport: "float64validator",
	})
}

func TestCalculateTypeMeta_ListOfStrings(t *testing.T) {
	f := FieldInfo{Type: OpenAPITypeArray, GoType: TFTypeList, ItemType: OpenAPITypeString}
	CalculateTypeMeta(&f)

	if f.TypeMeta.SchemaAttrType != "schema.ListAttribute" {
		t.Errorf("Expected schema.ListAttribute, got %s", f.TypeMeta.SchemaAttrType)
	}
	if f.TypeMeta.ElemType != "types.StringType" {
		t.Errorf("Expected types.StringType, got %s", f.TypeMeta.ElemType)
	}
	if f.TypeMeta.IsNested {
		t.Error("List of strings should not be nested")
	}
	if !f.TypeMeta.IsComplex {
		t.Error("List should be complex")
	}
	if f.TypeMeta.PlanModImport != "listplanmodifier" {
		t.Errorf("Expected listplanmodifier, got %s", f.TypeMeta.PlanModImport)
	}
}

func TestCalculateTypeMeta_ListOfObjects(t *testing.T) {
	f := FieldInfo{Type: OpenAPITypeArray, GoType: TFTypeList, ItemType: OpenAPITypeObject}
	CalculateTypeMeta(&f)

	if f.TypeMeta.SchemaAttrType != "schema.ListNestedAttribute" {
		t.Errorf("Expected schema.ListNestedAttribute, got %s", f.TypeMeta.SchemaAttrType)
	}
	if !f.TypeMeta.IsNested {
		t.Error("List of objects should be nested")
	}
}

func TestCalculateTypeMeta_SetOfStrings(t *testing.T) {
	f := FieldInfo{Type: OpenAPITypeArray, GoType: TFTypeSet, ItemType: OpenAPITypeString}
	CalculateTypeMeta(&f)

	if f.TypeMeta.SchemaAttrType != "schema.SetAttribute" {
		t.Errorf("Expected schema.SetAttribute, got %s", f.TypeMeta.SchemaAttrType)
	}
	if f.TypeMeta.PlanModImport != "setplanmodifier" {
		t.Errorf("Expected setplanmodifier, got %s", f.TypeMeta.PlanModImport)
	}
}

func TestCalculateTypeMeta_SetOfObjects(t *testing.T) {
	f := FieldInfo{Type: OpenAPITypeArray, GoType: TFTypeSet, ItemType: OpenAPITypeObject}
	CalculateTypeMeta(&f)

	if f.TypeMeta.SchemaAttrType != "schema.SetNestedAttribute" {
		t.Errorf("Expected schema.SetNestedAttribute, got %s", f.TypeMeta.SchemaAttrType)
	}
	if !f.TypeMeta.IsNested {
		t.Error("Set of objects should be nested")
	}
	if f.TypeMeta.PlanModType != "planmodifier.Set" {
		t.Errorf("Expected planmodifier.Set, got %s", f.TypeMeta.PlanModType)
	}
}

func TestCalculateTypeMeta_Map(t *testing.T) {
	f := FieldInfo{Type: OpenAPITypeObject, GoType: TFTypeMap, ItemType: OpenAPITypeNumber}
	CalculateTypeMeta(&f)

	if f.TypeMeta.SchemaAttrType != "schema.MapAttribute" {
		t.Errorf("Expected schema.MapAttribute, got %s", f.TypeMeta.SchemaAttrType)
	}
	if f.TypeMeta.ElemType != "types.Float64Type" {
		t.Errorf("Expected types.Float64Type, got %s", f.TypeMeta.ElemType)
	}
	if f.TypeMeta.PlanModImport != "mapplanmodifier" {
		t.Errorf("Expected mapplanmodifier, got %s", f.TypeMeta.PlanModImport)
	}
}

func TestCalculateTypeMeta_Object(t *testing.T) {
	f := FieldInfo{Type: OpenAPITypeObject, GoType: TFTypeObject}
	CalculateTypeMeta(&f)

	if f.TypeMeta.SchemaAttrType != "schema.SingleNestedAttribute" {
		t.Errorf("Expected schema.SingleNestedAttribute, got %s", f.TypeMeta.SchemaAttrType)
	}
	if !f.TypeMeta.IsNested {
		t.Error("Object should be nested")
	}
	if f.TypeMeta.PlanModImport != "objectplanmodifier" {
		t.Errorf("Expected objectplanmodifier, got %s", f.TypeMeta.PlanModImport)
	}
}

func TestCalculateTypeMeta_ListElemTypes(t *testing.T) {
	tests := []struct {
		itemType     string
		expectedElem string
	}{
		{OpenAPITypeString, "types.StringType"},
		{OpenAPITypeInteger, "types.Int64Type"},
		{OpenAPITypeBoolean, "types.BoolType"},
		{OpenAPITypeNumber, "types.Float64Type"},
		{"", "types.StringType"}, // default
	}

	for _, tt := range tests {
		f := FieldInfo{Type: OpenAPITypeArray, GoType: TFTypeList, ItemType: tt.itemType}
		CalculateTypeMeta(&f)
		if f.TypeMeta.ElemType != tt.expectedElem {
			t.Errorf("ItemType=%q: expected ElemType=%q, got %q", tt.itemType, tt.expectedElem, f.TypeMeta.ElemType)
		}
	}
}

func TestCalculateTypeMeta_OrderCommonFields(t *testing.T) {
	// OrderCommonFields should have TypeMeta populated via init()
	for _, f := range OrderCommonFields {
		if f.TypeMeta.SchemaAttrType == "" {
			t.Errorf("OrderCommonField %q has empty SchemaAttrType", f.Name)
		}
		if f.TypeMeta.PlanModImport == "" {
			t.Errorf("OrderCommonField %q has empty PlanModImport", f.Name)
		}
	}
}

// assertMeta compares key TypeMeta fields
func assertMeta(t *testing.T, got, want TypeMeta) {
	t.Helper()
	if got.SchemaAttrType != want.SchemaAttrType {
		t.Errorf("SchemaAttrType: want %q, got %q", want.SchemaAttrType, got.SchemaAttrType)
	}
	if got.AttrValueType != want.AttrValueType {
		t.Errorf("AttrValueType: want %q, got %q", want.AttrValueType, got.AttrValueType)
	}
	if got.PlanModImport != want.PlanModImport {
		t.Errorf("PlanModImport: want %q, got %q", want.PlanModImport, got.PlanModImport)
	}
	if got.PlanModType != want.PlanModType {
		t.Errorf("PlanModType: want %q, got %q", want.PlanModType, got.PlanModType)
	}
	if got.FromAPIFunc != want.FromAPIFunc {
		t.Errorf("FromAPIFunc: want %q, got %q", want.FromAPIFunc, got.FromAPIFunc)
	}
	if got.ToAPIMethod != want.ToAPIMethod {
		t.Errorf("ToAPIMethod: want %q, got %q", want.ToAPIMethod, got.ToAPIMethod)
	}
	if got.ValidatorType != want.ValidatorType {
		t.Errorf("ValidatorType: want %q, got %q", want.ValidatorType, got.ValidatorType)
	}
	if got.ValidatorImport != want.ValidatorImport {
		t.Errorf("ValidatorImport: want %q, got %q", want.ValidatorImport, got.ValidatorImport)
	}
}
