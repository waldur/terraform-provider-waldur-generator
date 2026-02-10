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
	if f.GoType == "types.Object" && f.AttrTypeRef != "" {
		return f.AttrTypeRef + "Type()"
	}
	// If it's a list/set of objects with a AttrTypeRef, return collection with helper function
	if (f.GoType == "types.List" || f.GoType == "types.Set") && f.ItemType == "object" && f.ItemSchema != nil && f.ItemSchema.AttrTypeRef != "" {
		var collectionType string
		if f.GoType == "types.List" {
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
	switch f.GoType {
	case "types.String":
		return "types.StringType"
	case "types.Int64":
		return "types.Int64Type"
	case "types.Bool":
		return "types.BoolType"
	case "types.Float64":
		return "types.Float64Type"
	case "types.List":
		if f.ItemType == "object" && f.ItemSchema != nil {
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
		elemType := "types.StringType"
		if f.ItemType == "integer" {
			elemType = "types.Int64Type"
		} else if f.ItemType == "boolean" {
			elemType = "types.BoolType"
		} else if f.ItemType == "number" {
			elemType = "types.Float64Type"
		}
		return "types.ListType{ElemType: " + elemType + "}"
	case "types.Set":
		if f.ItemType == "object" && f.ItemSchema != nil {
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
		elemType := "types.StringType"
		if f.ItemType == "integer" {
			elemType = "types.Int64Type"
		} else if f.ItemType == "boolean" {
			elemType = "types.BoolType"
		} else if f.ItemType == "number" {
			elemType = "types.Float64Type"
		}
		return "types.SetType{ElemType: " + elemType + "}"
	case "types.Object":
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
	if goType == "types.Int64" {
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
			typeName := ""
			isPointer := true
			if f.JsonTag == "-" {
				switch f.GoType {
				case "types.String":
					typeName = "types.String"
					isPointer = false
				case "types.Int64":
					typeName = "types.Int64"
					isPointer = false
				case "types.Bool":
					typeName = "types.Bool"
					isPointer = false
				case "types.Float64":
					typeName = "types.Float64"
					isPointer = false
				case "types.List":
					typeName = "types.List"
					isPointer = false
				case "types.Set":
					typeName = "types.Set"
					isPointer = false
				}
			}
			if typeName == "" {
				if f.Type == "string" {
					typeName = "string"
				} else if f.Type == "integer" {
					typeName = "int64"
				} else if f.Type == "boolean" {
					typeName = "bool"
				} else if f.Type == "number" {
					typeName = "float64"
					if suffix == "Response" {
						if pkgName != "common" {
							typeName = "common.FlexibleNumber"
						} else {
							typeName = "FlexibleNumber"
						}
					}
				}
			}
			if typeName != "" {
			} else if f.Type == "array" {
				isPointer = !f.Required
				if f.ItemType == "string" {
					typeName = "[]string"
				} else if f.ItemType == "integer" {
					typeName = "[]int64"
				} else {
					elemType := ""
					if f.ItemSchema != nil && f.ItemSchema.RefName != "" {
						if pkgName != "common" {
							elemType = "common." + f.ItemSchema.RefName
						} else {
							elemType = f.ItemSchema.RefName
						}
					} else {
						elemType = prefix + common.ToTitle(f.Name) + suffix
					}
					typeName = "[]" + elemType
				}
			} else if f.GoType == "types.Map" {
				typeName = "map[string]interface{}"
				isPointer = false
			} else if f.Type == "object" {
				if f.RefName != "" {
					if pkgName != "common" {
						typeName = "common." + f.RefName
					} else {
						typeName = f.RefName
					}
				} else {
					typeName = prefix + common.ToTitle(f.Name) + suffix
				}
			}
			if typeName == "" {
				typeName = "string"
			}
			if isPointer {
				return "*" + typeName
			}
			return typeName
		},
	}
}
