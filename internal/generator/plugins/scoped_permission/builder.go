package scoped_permission

import (
	"fmt"

	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/plugins"
)

type ScopedPermissionBuilder struct {
	plugins.BaseBuilder
}

func NewBuilder(base plugins.BaseBuilder) *ScopedPermissionBuilder {
	return &ScopedPermissionBuilder{BaseBuilder: base}
}

func (b *ScopedPermissionBuilder) scopeField() string {
	if b.Resource.ScopeField != "" {
		return b.Resource.ScopeField
	}
	return "scope"
}

func (b *ScopedPermissionBuilder) BuildCreateFields() ([]common.FieldInfo, error) {
	scopeField := b.scopeField()
	fields := []common.FieldInfo{
		{Name: scopeField, Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: true, ForceNew: true, Description: fmt.Sprintf("UUID of the %s", scopeField)},
		{Name: "user", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: true, ForceNew: true, Description: "UUID of the user"},
		{Name: "role", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: true, ForceNew: true, Description: "Role name (e.g. PROJECT.ADMIN, CUSTOMER.OWNER)"},
		{Name: "expiration_time", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Expiration time of the role (RFC3339 format or null)"},
	}
	for i := range fields {
		common.CalculateSDKType(&fields[i])
	}
	return fields, nil
}

func (b *ScopedPermissionBuilder) BuildUpdateFields() ([]common.FieldInfo, error) {
	return b.BuildCreateFields()
}

func (b *ScopedPermissionBuilder) BuildResponseFields() ([]common.FieldInfo, error) {
	scopeField := b.scopeField()
	fields := []common.FieldInfo{
		{Name: scopeField, Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: true, ForceNew: true, Description: fmt.Sprintf("UUID of the %s", scopeField), JsonTag: "scope_uuid"},
		{Name: "user", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: true, ForceNew: true, Description: "UUID of the user", JsonTag: "user_uuid"},
		{Name: "role", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: true, ForceNew: true, Description: "Role name (e.g. PROJECT.ADMIN, CUSTOMER.OWNER)", JsonTag: "role_name"},
		{Name: "expiration_time", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Expiration time of the role (RFC3339 format or null)"},
	}
	for i := range fields {
		common.CalculateSDKType(&fields[i])
	}
	return fields, nil
}

func (b *ScopedPermissionBuilder) GetAPIPaths() map[string]string {
	paths := make(map[string]string)
	scopeType := b.Resource.ScopeType
	if scopeType == "" {
		scopeType = "projects"
	}
	paths["Create"] = fmt.Sprintf("/api/%s/", scopeType)
	return paths
}

func (b *ScopedPermissionBuilder) GetTemplateFiles() []string {
	return []string{
		"templates/shared/*.tmpl",
		"components/resource/resource.go.tmpl",
		"plugins/scoped_permission/resource.tmpl",
	}
}
