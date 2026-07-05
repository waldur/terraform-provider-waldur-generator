package project_permission

import (
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/plugins"
)

type ProjectPermissionBuilder struct {
	plugins.BaseBuilder
}

func NewBuilder(base plugins.BaseBuilder) *ProjectPermissionBuilder {
	return &ProjectPermissionBuilder{BaseBuilder: base}
}

func (b *ProjectPermissionBuilder) BuildCreateFields() ([]common.FieldInfo, error) {
	fields := []common.FieldInfo{
		{Name: "project", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: true, ForceNew: true, Description: "UUID of the project"},
		{Name: "user", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: true, ForceNew: true, Description: "UUID of the user"},
		{Name: "role", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: true, ForceNew: true, Description: "Role name (e.g. PROJECT.ADMIN, PROJECT.MANAGER, PROJECT.MEMBER)"},
		{Name: "expiration_time", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Expiration time of the role (RFC3339 format or null)"},
	}
	for i := range fields {
		common.CalculateSDKType(&fields[i])
	}
	return fields, nil
}

func (b *ProjectPermissionBuilder) BuildUpdateFields() ([]common.FieldInfo, error) {
	return b.BuildCreateFields()
}

func (b *ProjectPermissionBuilder) BuildResponseFields() ([]common.FieldInfo, error) {
	fields := []common.FieldInfo{
		{Name: "project", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: true, ForceNew: true, Description: "UUID of the project", JsonTag: "scope_uuid"},
		{Name: "user", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: true, ForceNew: true, Description: "UUID of the user", JsonTag: "user_uuid"},
		{Name: "role", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: true, ForceNew: true, Description: "Role name (e.g. PROJECT.ADMIN, PROJECT.MANAGER, PROJECT.MEMBER)", JsonTag: "role_name"},
		{Name: "expiration_time", Type: common.OpenAPITypeString, GoType: common.TFTypeString, Required: false, Description: "Expiration time of the role (RFC3339 format or null)"},
	}
	for i := range fields {
		common.CalculateSDKType(&fields[i])
	}
	return fields, nil
}

func (b *ProjectPermissionBuilder) GetAPIPaths() map[string]string {
	paths := make(map[string]string)
	paths["Create"] = "/api/projects/"
	return paths
}

func (b *ProjectPermissionBuilder) GetTemplateFiles() []string {
	return []string{
		"templates/shared/*.tmpl",
		"components/resource/resource.go.tmpl",
		"plugins/project_permission/resource.tmpl",
	}
}
