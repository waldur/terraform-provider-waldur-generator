package common

// OrderCommonFields defines the standard fields present in all order-based resources
var OrderCommonFields = []FieldInfo{
	{
		Name:        "offering",
		Type:        OpenAPITypeString,
		Description: "Offering URL",
		GoType:      TFTypeString,
		Required:    true,
		SDKType:     GoTypeString,
		IsPointer:   true,
	},
	{
		Name:        "project",
		Type:        OpenAPITypeString,
		Description: "Project URL",
		GoType:      TFTypeString,
		Required:    true,
		SDKType:     GoTypeString,
		IsPointer:   true,
	},
	{
		Name:        "plan",
		Type:        OpenAPITypeString,
		Description: "Plan URL",
		GoType:      TFTypeString,
		Required:    false,
		SDKType:     GoTypeString,
		IsPointer:   true,
	},
	{
		Name:        "limits",
		Type:        OpenAPITypeObject,
		Description: "Resource limits",
		GoType:      TFTypeMap,
		ItemType:    OpenAPITypeNumber,
		Required:    false,
		SDKType:     "map[string]float64",
		IsPointer:   false,
	},
	{
		Name:        "start_date",
		Type:        OpenAPITypeString,
		Format:      "date-time",
		Description: "Order start date",
		GoType:      TFTypeString,
		Required:    false,
		SDKType:     GoTypeString,
		IsPointer:   true,
	},
	{
		Name:        "end_date",
		Type:        OpenAPITypeString,
		Format:      "date-time",
		Description: "Order end date",
		GoType:      TFTypeString,
		Required:    false,
		SDKType:     GoTypeString,
		IsPointer:   true,
	},
}

func init() {
	for i := range OrderCommonFields {
		CalculateTypeMeta(&OrderCommonFields[i])
	}
}
