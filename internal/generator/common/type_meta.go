package common

// TypeMeta holds all pre-calculated, type-specific strings needed by templates.
// This eliminates type-branching in templates by providing ready-to-use fragments.
type TypeMeta struct {
	// Schema generation
	SchemaAttrType string // e.g., "schema.StringAttribute", "schema.ListNestedAttribute"
	AttrValueType  string // e.g., "types.StringType", "types.Int64Type"
	ElemType       string // For collections: element type expression (e.g., "types.StringType")

	// Plan modifiers
	PlanModImport string // e.g., "stringplanmodifier", "listplanmodifier"
	PlanModType   string // e.g., "planmodifier.String", "planmodifier.List"

	// Value conversion (API response → TF model)
	FromAPIFunc string // e.g., "types.StringPointerValue", "types.Int64PointerValue"
	// Value conversion (TF model → API request)
	ToAPIMethod string // e.g., "ValueStringPointer", "ValueInt64Pointer"

	// Validators
	ValidatorType   string // e.g., "String", "Int64", "Float64"
	ValidatorImport string // e.g., "stringvalidator", "int64validator", "float64validator"

	// Classification flags
	IsNested   bool // true if needs NestedAttribute (objects in list/set, or single object)
	IsComplex  bool // true if list/set/map/object (not a simple scalar)
	IsDateTime bool // true if string with format="date-time" (needs timetypes)
}

// CalculateTypeMeta populates TypeMeta on a FieldInfo based on its Type, GoType, ItemType, and Format.
// This should be called after GoType and SDKType are already set.
func CalculateTypeMeta(f *FieldInfo) {
	m := &f.TypeMeta

	switch f.GoType {
	case TFTypeString:
		if f.Format == "date-time" {
			m.IsDateTime = true
			m.SchemaAttrType = "schema.StringAttribute"
			m.AttrValueType = "types.StringType"
			m.PlanModImport = "stringplanmodifier"
			m.PlanModType = "planmodifier.String"
			m.FromAPIFunc = "" // Special: uses timetypes.NewRFC3339PointerValue
			m.ToAPIMethod = "ValueStringPointer"
			m.ValidatorImport = "stringvalidator"
		} else {
			m.SchemaAttrType = "schema.StringAttribute"
			m.AttrValueType = "types.StringType"
			m.PlanModImport = "stringplanmodifier"
			m.PlanModType = "planmodifier.String"
			m.FromAPIFunc = "common.StringPointerValue"
			m.ToAPIMethod = "ValueStringPointer"
			m.ValidatorImport = "stringvalidator"
		}

	case TFTypeInt64:
		m.SchemaAttrType = "schema.Int64Attribute"
		m.AttrValueType = "types.Int64Type"
		m.PlanModImport = "int64planmodifier"
		m.PlanModType = "planmodifier.Int64"
		m.FromAPIFunc = "types.Int64PointerValue"
		m.ToAPIMethod = "ValueInt64Pointer"
		m.ValidatorImport = "int64validator"

	case TFTypeBool:
		m.SchemaAttrType = "schema.BoolAttribute"
		m.AttrValueType = "types.BoolType"
		m.PlanModImport = "boolplanmodifier"
		m.PlanModType = "planmodifier.Bool"
		m.FromAPIFunc = "types.BoolPointerValue"
		m.ToAPIMethod = "ValueBoolPointer"
		m.ValidatorImport = "" // No dedicated bool validator import typically

	case TFTypeFloat64:
		m.SchemaAttrType = "schema.Float64Attribute"
		m.AttrValueType = "types.Float64Type"
		m.PlanModImport = "float64planmodifier"
		m.PlanModType = "planmodifier.Float64"
		m.FromAPIFunc = "types.Float64PointerValue"
		m.ToAPIMethod = "ValueFloat64Pointer"
		m.ValidatorImport = "float64validator"

	case TFTypeList:
		m.IsComplex = true
		m.PlanModImport = "listplanmodifier"
		m.PlanModType = "planmodifier.List"
		if f.ItemType == OpenAPITypeObject {
			m.IsNested = true
			m.SchemaAttrType = "schema.ListNestedAttribute"
		} else {
			m.SchemaAttrType = "schema.ListAttribute"
			m.ElemType = itemTypeToAttrType(f.ItemType)
		}

	case TFTypeSet:
		m.IsComplex = true
		m.PlanModImport = "setplanmodifier"
		m.PlanModType = "planmodifier.Set"
		if f.ItemType == OpenAPITypeObject {
			m.IsNested = true
			m.SchemaAttrType = "schema.SetNestedAttribute"
		} else {
			m.SchemaAttrType = "schema.SetAttribute"
			m.ElemType = itemTypeToAttrType(f.ItemType)
		}

	case TFTypeMap:
		m.IsComplex = true
		m.SchemaAttrType = "schema.MapAttribute"
		m.PlanModImport = "mapplanmodifier"
		m.PlanModType = "planmodifier.Map"
		m.ElemType = itemTypeToAttrType(f.ItemType)

	case TFTypeObject:
		m.IsComplex = true
		m.IsNested = true
		m.SchemaAttrType = "schema.SingleNestedAttribute"
		m.PlanModImport = "objectplanmodifier"
		m.PlanModType = "planmodifier.Object"
	default:
		m.SchemaAttrType = "schema.StringAttribute"
	}

	m.ValidatorType = GoTypeToValidatorType(f.GoType)
}

// GoTypeToValidatorType maps a Terraform framework type to the corresponding validator type name.
func GoTypeToValidatorType(goType string) string {
	switch goType {
	case TFTypeInt64:
		return "Int64"
	case TFTypeBool:
		return "Bool"
	case TFTypeFloat64:
		return "Float64"
	default:
		return "String"
	}
}

// itemTypeToAttrType converts an OpenAPI item type to a Terraform attr.Type expression string.
func itemTypeToAttrType(itemType string) string {
	switch itemType {
	case OpenAPITypeInteger:
		return "types.Int64Type"
	case OpenAPITypeBoolean:
		return "types.BoolType"
	case OpenAPITypeNumber:
		return "types.Float64Type"
	default:
		return "types.StringType"
	}
}
