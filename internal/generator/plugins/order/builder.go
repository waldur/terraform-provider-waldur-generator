package order

import (
	"fmt"
	"strings"

	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/plugins"
)

// OrderBuilder implements ResourceBuilder for marketplace order resources
type OrderBuilder struct {
	plugins.BaseBuilder
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
	// Add required offering and project fields
	fields = append(fields, common.OrderCommonFields...)
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
	// Add Termination Attributes
	for _, term := range b.Resource.TerminationAttributes {
		f := common.FieldInfo{
			Name: term.Name, Type: term.Type, Description: "Termination attribute", GoType: common.GetGoType(term.Type),
		}
		common.CalculateSDKType(&f)
		modelFields = append(modelFields, f)
	}
	return modelFields, nil
}

func (b *OrderBuilder) GetAPIPaths() map[string]string {
	paths := b.BaseBuilder.GetAPIPaths()
	return paths
}

func (b *OrderBuilder) GetTemplateFiles() []string {
	return append(b.BaseBuilder.GetTemplateFiles(), "plugins/order/resource.tmpl")
}
