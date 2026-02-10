package builders

import (
	"fmt"
	"strings"

	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

// OrderBuilder implements ResourceBuilder for marketplace order resources
type OrderBuilder struct {
	BaseBuilder
}

func (b *OrderBuilder) BuildCreateFields() ([]common.FieldInfo, error) {
	schemaName := strings.ReplaceAll(b.Resource.OfferingType, ".", "") + "CreateOrderAttributes"
	offeringSchema, err := b.Parser.GetSchema(schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to find offering schema %s: %w", schemaName, err)
	}
	fields, err := common.ExtractFields(b.SchemaConfig, offeringSchema, true)
	if err != nil {
		return nil, err
	}
	for i := range fields {
		fields[i].Required = false
	}
	// Add required offering and project fields
	fields = append(fields, common.FieldInfo{
		Name: "offering", Type: "string", Description: "Offering URL", GoType: "types.String", Required: true,
	}, common.FieldInfo{
		Name: "project", Type: "string", Description: "Project URL", GoType: "types.String", Required: true,
	})
	return fields, nil
}

func (b *OrderBuilder) BuildUpdateFields() ([]common.FieldInfo, error) {
	schema, err := b.Parser.GetOperationRequestSchema(b.Ops.PartialUpdate)
	if err != nil {
		return nil, nil
	}
	return common.ExtractFields(b.SchemaConfig, schema, true)
}

func (b *OrderBuilder) BuildResponseFields() ([]common.FieldInfo, error) {
	schema, err := b.Parser.GetOperationResponseSchema(b.Ops.Retrieve)
	if err != nil {
		return nil, err
	}
	return common.ExtractFields(b.SchemaConfig, schema, true)
}

func (b *OrderBuilder) BuildModelFields(createFields, responseFields []common.FieldInfo) ([]common.FieldInfo, error) {
	modelFields := common.MergeOrderFields(createFields, responseFields)
	// Add Plan and Limits fields manually to ModelFields for Order resources
	modelFields = common.MergeFields(modelFields, []common.FieldInfo{
		{Name: "plan", Type: "string", Description: "Plan URL", GoType: "types.String", Required: false},
		{Name: "limits", Type: "object", Description: "Resource limits", GoType: "types.Map", ItemType: "number", Required: false},
	})
	// Add Termination Attributes
	for _, term := range b.Resource.TerminationAttributes {
		goType := "types.String"
		switch term.Type {
		case "boolean":
			goType = "types.Bool"
		case "integer":
			goType = "types.Int64"
		case "number":
			goType = "types.Float64"
		}
		modelFields = append(modelFields, common.FieldInfo{
			Name: term.Name, Type: term.Type, Description: "Termination attribute", GoType: goType,
		})
	}
	return modelFields, nil
}

func (b *OrderBuilder) GetAPIPaths() map[string]string {
	paths := b.BaseBuilder.GetAPIPaths()
	return paths
}

func (b *OrderBuilder) GetTemplateFiles() []string {
	return []string{"templates/shared.tmpl", "templates/resource.go.tmpl", "templates/resource_order.tmpl"}
}
