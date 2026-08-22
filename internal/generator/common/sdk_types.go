package common

// CalculateSDKType determines the Go SDK type and pointer status for a field
func CalculateSDKType(f *FieldInfo) {
	// Default pointer status: optional fields are pointers
	f.IsPointer = !f.Required

	// 1. Types that are explicitly ignored/internal use Terraform types
	if f.JsonTag == "-" {
		f.IsPointer = false
		switch f.GoType {
		case TFTypeString:
			f.SDKType = TFTypeString
		case TFTypeInt64:
			f.SDKType = TFTypeInt64
		case TFTypeBool:
			f.SDKType = TFTypeBool
		case TFTypeFloat64:
			f.SDKType = TFTypeFloat64
		case TFTypeList:
			f.SDKType = TFTypeList
		case TFTypeSet:
			f.SDKType = TFTypeSet
		case TFTypeMap:
			f.SDKType = TFTypeMap
		default:
			f.SDKType = TFTypeString
		}
		return
	}

	// 2. Standard Go Types
	switch f.Type {
	case OpenAPITypeString:
		f.SDKType = GoTypeString
		f.IsPointer = true // Strings are almost always pointers in SDK

	case OpenAPITypeInteger:
		f.SDKType = GoTypeInt64
		f.IsPointer = true // All primitives are always pointers in SDK structs

	case OpenAPITypeBoolean:
		f.SDKType = GoTypeBool
		f.IsPointer = true

	case OpenAPITypeNumber:
		f.SDKType = GoTypeFloat64
		f.IsPointer = true

	case OpenAPITypeArray:
		f.IsPointer = !f.Required // Slices are pointers if optional in this SDK convention
		if f.ItemType == OpenAPITypeString {
			f.SDKType = "[]string"
		} else if f.ItemType == OpenAPITypeInteger {
			f.SDKType = "[]int64"
		} else if f.ItemType == OpenAPITypeNumber {
			f.SDKType = "[]float64"
		} else if f.ItemType == OpenAPITypeObject {
			// Array of objects
			// If ItemSchema has RefName, use it
			if f.ItemSchema != nil && f.ItemSchema.RefName != "" {
				f.SDKType = "[]" + f.ItemSchema.RefName
			} else {
				// Anonymous struct, templates will handle prefix naming
				f.SDKType = "[]"
			}
		}

	case OpenAPITypeObject:
		// Map detection (Terraform types.Map logic)
		if f.GoType == TFTypeMap {
			f.IsPointer = false // Maps are reference types
			valType := GoTypeAny
			switch f.ItemType {
			case OpenAPITypeNumber:
				valType = GoTypeFloat64
			case OpenAPITypeInteger:
				valType = GoTypeInt64
			case OpenAPITypeString:
				valType = GoTypeString
			}
			f.SDKType = "map[string]" + valType
			CalculateTypeMeta(f)
			return
		}

		f.IsPointer = true
		if f.RefName != "" {
			f.SDKType = f.RefName
		} else {
			f.SDKType = "" // Anonymous
		}
	}

	// Always calculate TypeMeta after SDK type is determined
	CalculateTypeMeta(f)
}

// IsAnyMap reports whether a field is an API map whose values are not primitives.
// Such maps are represented as map[string]interface{} in the SDK structs, which the
// Terraform Plugin Framework cannot reflect into the generated map(string) attribute.
// Response structs use JSONStringMap for these instead.
func IsAnyMap(f FieldInfo) bool {
	if f.GoType != TFTypeMap || f.SDKType != GoTypeMap+GoTypeAny {
		return false
	}
	// A boolean-valued map also lands on map[string]interface{} (the map switch in
	// CalculateSDKType has no boolean case), but its TypeMeta.ElemType is
	// types.BoolType. Swapping in JSONStringMap there would hand a map[string]string
	// to types.MapValueFrom(ctx, types.BoolType, ...) and fail at runtime, so only
	// claim maps whose value type is genuinely unconstrained.
	switch f.ItemType {
	case OpenAPITypeString, OpenAPITypeInteger, OpenAPITypeNumber, OpenAPITypeBoolean:
		return false
	}
	return true
}
