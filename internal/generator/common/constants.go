package common

// OrderCommonFields defines the standard fields present in all order-based resources
var OrderCommonFields = []FieldInfo{
	{
		Name:        "offering",
		Type:        "string",
		Description: "Offering URL",
		GoType:      "types.String",
		Required:    true,
	},
	{
		Name:        "project",
		Type:        "string",
		Description: "Project URL",
		GoType:      "types.String",
		Required:    true,
	},
	{
		Name:        "plan",
		Type:        "string",
		Description: "Plan URL",
		GoType:      "types.String",
		Required:    false,
	},
	{
		Name:        "limits",
		Type:        "object",
		Description: "Resource limits",
		GoType:      "types.Map",
		ItemType:    "number",
		Required:    false,
	},
	{
		Name:        "start_date",
		Type:        "string",
		Format:      "date-time",
		Description: "Order start date",
		GoType:      "types.String",
		Required:    false,
	},
	{
		Name:        "end_date",
		Type:        "string",
		Format:      "date-time",
		Description: "Order end date",
		GoType:      "types.String",
		Required:    false,
	},
}
