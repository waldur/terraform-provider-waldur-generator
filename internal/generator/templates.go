package generator

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

// ToAttrType returns the type definition or a function call to a shared type helper
func ToAttrType(f common.FieldInfo) string {
	// If it's an object with a AttrTypeRef, return the helper function call
	if f.GoType == common.TFTypeObject && f.AttrTypeRef != "" {
		return f.AttrTypeRef + "Type()"
	}
	// If it's a list/set of objects with a AttrTypeRef, return collection with helper function
	if (f.GoType == common.TFTypeList || f.GoType == common.TFTypeSet) && f.ItemType == common.OpenAPITypeObject && f.ItemSchema != nil && f.ItemSchema.AttrTypeRef != "" {
		var collectionType string
		if f.GoType == common.TFTypeList {
			collectionType = "types.ListType"
		} else {
			collectionType = "types.SetType"
		}
		return collectionType + "{ElemType: " + f.ItemSchema.AttrTypeRef + "Type()}"
	}

	return ToAttrTypeDefinition(f)
}

// ToAttrTypeDefinition converts FieldInfo to proper attr.Type expression used in Terraform schema
func ToAttrTypeDefinition(f common.FieldInfo) string {
	if !f.TypeMeta.IsComplex {
		return f.TypeMeta.AttrValueType
	}

	switch f.GoType {
	case common.TFTypeList:
		if f.ItemType == common.OpenAPITypeObject && f.ItemSchema != nil {
			var attrTypes []string
			sortedProps := f.ItemSchema.Properties
			for _, prop := range sortedProps {
				attrTypes = append(attrTypes, "\""+prop.Name+"\": "+ToAttrType(prop))
			}
			content := strings.Join(attrTypes, ",\n")
			if len(attrTypes) > 0 {
				content += ","
			}
			objType := "types.ObjectType{AttrTypes: map[string]attr.Type{\n" + content + "\n}}"
			return "types.ListType{ElemType: " + objType + "}"
		}
		return "types.ListType{ElemType: " + f.TypeMeta.ElemType + "}"

	case common.TFTypeSet:
		if f.ItemType == common.OpenAPITypeObject && f.ItemSchema != nil {
			var attrTypes []string
			sortedProps := f.ItemSchema.Properties
			for _, prop := range sortedProps {
				attrTypes = append(attrTypes, "\""+prop.Name+"\": "+ToAttrType(prop))
			}
			content := strings.Join(attrTypes, ",\n")
			if len(attrTypes) > 0 {
				content += ","
			}
			objType := "types.ObjectType{AttrTypes: map[string]attr.Type{\n" + content + "\n}}"
			return "types.SetType{ElemType: " + objType + "}"
		}
		return "types.SetType{ElemType: " + f.TypeMeta.ElemType + "}"

	case common.TFTypeMap:
		return "types.MapType{ElemType: " + f.TypeMeta.ElemType + "}"

	case common.TFTypeObject:
		var attrTypes []string
		for _, prop := range f.Properties {
			attrTypes = append(attrTypes, "\""+prop.Name+"\": "+ToAttrType(prop))
		}
		content := strings.Join(attrTypes, ",\n")
		if len(attrTypes) > 0 {
			content += ","
		}
		return "types.ObjectType{AttrTypes: map[string]attr.Type{\n" + content + "\n}}"

	default:
		return "types.StringType"
	}
}

func formatValidatorValue(v float64, goType string) string {
	if goType == common.TFTypeInt64 {
		if v > 9e18 || v < -9e18 {
			return ""
		}
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%g", v)
}

// GetFuncMap returns the common template functions
func GetFuncMap() template.FuncMap {
	return template.FuncMap{
		"title":                common.ToTitle,
		"humanize":             common.Humanize,
		"displayName":          common.DisplayName,
		"toAttrType":           ToAttrType,
		"toAttrTypeDefinition": ToAttrTypeDefinition,
		"formatValidator":      formatValidatorValue,
		"replace": func(old, new, s string) string {
			return strings.ReplaceAll(s, old, new)
		},
		"contains": strings.Contains,
		"isPathParam": func(op *config.CreateOperationConfig, fieldName string) bool {
			if op == nil {
				return false
			}
			for _, val := range op.PathParams {
				if val == fieldName {
					return true
				}
			}
			return false
		},
		"isOrderAttribute": func(name string) bool {
			for _, field := range common.OrderCommonFields {
				if field.Name == name {
					return false
				}
			}
			return true
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, nil
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, nil
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"makeSlice": func(items ...interface{}) interface{} {
			return items
		},
		"renderGoType": func(f common.FieldInfo, pkgName string, prefix string, suffix string) string {
			sdkType := f.SDKType
			isPointer := f.IsPointer

			// Handle context-dependent anonymous types
			if f.Type == common.OpenAPITypeObject && sdkType == "" {
				sdkType = prefix + common.ToTitle(f.Name) + suffix
			} else if f.Type == common.OpenAPITypeArray && f.ItemType == common.OpenAPITypeObject && sdkType == "[]" {
				elemType := prefix + common.ToTitle(f.Name) + suffix
				sdkType = "[]" + elemType
			}

			// Handle package prefixes for references
			if pkgName != "common" {
				if f.Type == common.OpenAPITypeObject && f.RefName != "" {
					sdkType = "common." + f.RefName
				} else if f.Type == common.OpenAPITypeArray && f.ItemRefName != "" {
					sdkType = "[]common." + f.ItemRefName
				}
			}

			// Handle FlexibleNumber for responses
			if f.Type == common.OpenAPITypeNumber && suffix == "Response" {
				if pkgName != "common" {
					sdkType = "common.FlexibleNumber"
				} else {
					sdkType = "FlexibleNumber"
				}
				isPointer = false
			}

			if isPointer {
				return "*" + sdkType
			}
			return sdkType
		},
	}
}
