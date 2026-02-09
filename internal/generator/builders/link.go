package builders

import (
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

// LinkBuilder implements ResourceBuilder for link resources
type LinkBuilder struct {
	BaseBuilder
}

func (b *LinkBuilder) BuildCreateFields() ([]common.FieldInfo, error) {
	schema, err := b.Parser.GetOperationRequestSchema(b.Resource.LinkOp)
	if err != nil {
		return nil, nil
	}
	fields, err := common.ExtractFields(schema, true)
	if err != nil {
		return nil, err
	}
	// Source and Target handling
	if b.Resource.Source != nil && b.Resource.Source.Param != "" {
		found := false
		for _, f := range fields {
			if f.Name == b.Resource.Source.Param {
				found = true
				break
			}
		}
		if !found {
			fields = append(fields, common.FieldInfo{
				Name: b.Resource.Source.Param, Type: "string", Description: "Source resource UUID", GoType: "types.String", Required: true,
			})
		}
	}
	if b.Resource.Target != nil && b.Resource.Target.Param != "" {
		found := false
		for _, f := range fields {
			if f.Name == b.Resource.Target.Param {
				found = true
				break
			}
		}
		if !found {
			fields = append(fields, common.FieldInfo{
				Name: b.Resource.Target.Param, Type: "string", Description: "Target resource UUID", GoType: "types.String", Required: true,
			})
		}
	}
	// LinkParams
	for _, param := range b.Resource.LinkParams {
		found := false
		for _, f := range fields {
			if f.Name == param.Name {
				found = true
				break
			}
		}
		if !found {
			goType := "types.String"
			switch param.Type {
			case "boolean":
				goType = "types.Bool"
			case "integer":
				goType = "types.Int64"
			case "number":
				goType = "types.Float64"
			}
			fields = append(fields, common.FieldInfo{
				Name: param.Name, Type: param.Type, Description: "Link parameter", GoType: goType, Required: false,
			})
		}
	}
	return fields, nil
}

func (b *LinkBuilder) BuildUpdateFields() ([]common.FieldInfo, error) {
	return nil, nil
}

func (b *LinkBuilder) BuildResponseFields() ([]common.FieldInfo, error) {
	if schema, err := b.Parser.GetOperationResponseSchema(b.Ops.Retrieve); err == nil {
		return common.ExtractFields(schema, true)
	}
	return nil, nil
}

func (b *LinkBuilder) GetAPIPaths() map[string]string {
	paths := make(map[string]string)
	if _, listPath, _, err := b.Parser.GetOperation(b.Ops.List); err == nil {
		paths["Base"] = listPath
	}
	if _, retrievePath, _, err := b.Parser.GetOperation(b.Ops.Retrieve); err == nil {
		paths["Retrieve"] = retrievePath
	}
	if _, linkPath, _, err := b.Parser.GetOperation(b.Resource.LinkOp); err == nil {
		paths["Link"] = linkPath
	}
	if _, unlinkPath, _, err := b.Parser.GetOperation(b.Resource.UnlinkOp); err == nil {
		paths["Unlink"] = unlinkPath
	}
	if b.Resource.Source != nil && b.Resource.Source.RetrieveOp != "" {
		if _, sourcePath, _, err := b.Parser.GetOperation(b.Resource.Source.RetrieveOp); err == nil {
			paths["SourceRetrieve"] = sourcePath
		}
	}
	return paths
}
