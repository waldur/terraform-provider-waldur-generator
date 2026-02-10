package plugins

import (
	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
	"github.com/waldur/terraform-provider-waldur-generator/internal/openapi"
)

// ResourceBuilder defines the interface for building resource-specific data
type ResourceBuilder interface {
	BuildCreateFields() ([]common.FieldInfo, error)
	BuildUpdateFields() ([]common.FieldInfo, error)
	BuildResponseFields() ([]common.FieldInfo, error)
	BuildModelFields(createFields, responseFields []common.FieldInfo) ([]common.FieldInfo, error) // Added
	GetAPIPaths() map[string]string
	GetTemplateFiles() []string
}

// BaseBuilder provides common functionality for all builders
type BaseBuilder struct {
	Parser       *openapi.Parser
	Resource     *config.Resource
	Ops          config.OperationSet
	SchemaConfig common.SchemaConfig
}

func (b *BaseBuilder) BuildModelFields(createFields, responseFields []common.FieldInfo) ([]common.FieldInfo, error) {
	return common.MergeFields(createFields, responseFields), nil
}

func (b *BaseBuilder) GetAPIPaths() map[string]string {
	paths := make(map[string]string)
	// Get path from list operation (used as base path)
	if _, listPath, _, err := b.Parser.GetOperation(b.Ops.List); err == nil {
		paths["Base"] = listPath
	}

	// Get path from create operation
	createOp := b.Ops.Create
	if b.Resource.CreateOperation != nil && b.Resource.CreateOperation.OperationID != "" {
		createOp = b.Resource.CreateOperation.OperationID
		if _, createPath, _, err := b.Parser.GetOperation(createOp); err == nil {
			paths["Create"] = createPath
			paths["CreateOperationID"] = createOp
			for k, v := range b.Resource.CreateOperation.PathParams {
				paths["CreatePathParam_"+k] = v
			}
		}
	} else if _, createPath, _, err := b.Parser.GetOperation(createOp); err == nil {
		paths["Create"] = createPath
	}

	// Get path from retrieve operation
	if _, retrievePath, _, err := b.Parser.GetOperation(b.Ops.Retrieve); err == nil {
		paths["Retrieve"] = retrievePath
	}

	// Get path from update operation
	if _, updatePath, _, err := b.Parser.GetOperation(b.Ops.PartialUpdate); err == nil {
		paths["Update"] = updatePath
	}

	// Get path from delete operation
	if _, deletePath, _, err := b.Parser.GetOperation(b.Ops.Destroy); err == nil {
		paths["Delete"] = deletePath
	}

	return paths
}

func (b *BaseBuilder) GetTemplateFiles() []string {
	return []string{"templates/shared.tmpl", "components/resource/resource.go.tmpl"}
}
