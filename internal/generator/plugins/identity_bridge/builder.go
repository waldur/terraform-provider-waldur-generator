package identity_bridge

import (
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/plugins"
)

type IdentityBridgeBuilder struct {
	plugins.BaseBuilder
}

func (b *IdentityBridgeBuilder) BuildCreateFields() ([]common.FieldInfo, error) {
	fields := []common.FieldInfo{
		{Name: "username", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: true, ForceNew: true, Description: "CUID / username of the federated user"},
		{Name: "source", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: true, ForceNew: true, Description: "ISD source identifier (must match `^[a-z]+:[a-zA-Z0-9._-]+$`)"},
		{Name: "first_name", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "First name"},
		{Name: "last_name", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Last name"},
		{Name: "email", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Email address"},
		{Name: "organization", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Organization name"},
		{Name: "phone_number", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Phone number"},
		{Name: "civil_number", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Civil number"},
		{Name: "identity_source", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Identity source"},
		{Name: "gender", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Gender"},
		{Name: "personal_title", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Personal title"},
		{Name: "birth_date", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Birth date"},
		{Name: "place_of_birth", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Place of birth"},
		{Name: "address", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Address"},
		{Name: "country_of_residence", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Country of residence"},
		{Name: "nationality", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Nationality"},
		{Name: "organization_country", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Organization country"},
		{Name: "organization_type", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Organization type"},
		{Name: "affiliations", Type: common.OpenAPITypeArray, ItemType: common.OpenAPITypeString, GoType: common.TFTypeList, Required: false, Description: "List of affiliations"},
		{Name: "nationalities", Type: common.OpenAPITypeArray, ItemType: common.OpenAPITypeString, GoType: common.TFTypeList, Required: false, Description: "List of nationalities"},
		{Name: "eduperson_assurance", Type: common.OpenAPITypeArray, ItemType: common.OpenAPITypeString, GoType: common.TFTypeList, Required: false, Description: "List of eduperson assurances"},
	}
	for i := range fields {
		common.CalculateSDKType(&fields[i])
	}
	return fields, nil
}

func (b *IdentityBridgeBuilder) BuildUpdateFields() ([]common.FieldInfo, error) {
	return b.BuildCreateFields()
}

func (b *IdentityBridgeBuilder) BuildResponseFields() ([]common.FieldInfo, error) {
	fields := []common.FieldInfo{
		{Name: "user_uuid", JsonTag: "", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, ServerComputed: true, ReadOnly: true, Description: "UUID of the created or updated user"},
		{Name: "is_created", JsonTag: "created", Type: common.OpenAPITypeBoolean, GoType: common.TFTypeBool, Required: false, ServerComputed: true, ReadOnly: true, Description: "True if the user was created, false if updated"},
		{Name: "updated_fields", Type: common.OpenAPITypeArray, ItemType: common.OpenAPITypeString, GoType: common.TFTypeList, Required: false, ServerComputed: true, ReadOnly: true, Description: "List of fields that were updated"},
	}
	for i := range fields {
		common.CalculateSDKType(&fields[i])
		common.CalculateTypeMeta(&fields[i])
	}
	return fields, nil
}

func (b *IdentityBridgeBuilder) GetAPIPaths() map[string]string {
	paths := make(map[string]string)
	paths["Create"] = "/api/identity-bridge/"
	return paths
}

func (b *IdentityBridgeBuilder) GetTemplateFiles() []string {
	return []string{
		"templates/shared/*.tmpl",
		"components/resource/resource.go.tmpl",
		"plugins/identity_bridge/resource.tmpl",
	}
}
