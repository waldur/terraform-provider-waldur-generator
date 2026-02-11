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
