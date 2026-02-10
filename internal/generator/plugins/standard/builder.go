package standard

import (
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/plugins"
)

// StandardBuilder implements ResourceBuilder for standard resources
type StandardBuilder struct {
	plugins.BaseBuilder
}

func (b *StandardBuilder) BuildCreateFields() ([]common.FieldInfo, error) {
	createOp := b.Ops.Create
	if b.Resource.CreateOperation != nil && b.Resource.CreateOperation.OperationID != "" {
		createOp = b.Resource.CreateOperation.OperationID
	}
	schema, err := b.Parser.GetOperationRequestSchema(createOp)
	if err != nil {
		return nil, nil // Some resources might not have a create schema
	}
	return common.ExtractFields(b.SchemaConfig, schema, true)
}

func (b *StandardBuilder) BuildUpdateFields() ([]common.FieldInfo, error) {
	schema, err := b.Parser.GetOperationRequestSchema(b.Ops.PartialUpdate)
	if err != nil {
		return nil, nil
	}
	return common.ExtractFields(b.SchemaConfig, schema, true)
}

func (b *StandardBuilder) BuildResponseFields() ([]common.FieldInfo, error) {
	schema, err := b.Parser.GetOperationResponseSchema(b.Ops.Retrieve)
	if err != nil {
		return nil, err
	}
	return common.ExtractFields(b.SchemaConfig, schema, true)
}

func (b *StandardBuilder) GetTemplateFiles() []string {
	return []string{"templates/shared.tmpl", "components/resource/resource.go.tmpl", "plugins/standard/resource.tmpl"}
}
