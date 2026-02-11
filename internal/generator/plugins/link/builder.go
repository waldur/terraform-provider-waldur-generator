package link

import (
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/plugins"
)

// LinkBuilder implements ResourceBuilder for link resources
type LinkBuilder struct {
	plugins.BaseBuilder
}

func (b *LinkBuilder) BuildCreateFields() ([]common.FieldInfo, error) {
	schema, err := b.Parser.GetOperationRequestSchema(b.Resource.LinkOp)
	if err != nil {
		return nil, nil
	}
	fields, err := common.ExtractFields(b.SchemaConfig, schema, true)
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
			f := common.FieldInfo{
				Name: b.Resource.Source.Param, Type: common.OpenAPITypeString, Description: "Source resource UUID", GoType: common.TFTypeString, Required: true,
			}
			common.CalculateSDKType(&f)
			fields = append(fields, f)
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
			f := common.FieldInfo{
				Name: b.Resource.Target.Param, Type: common.OpenAPITypeString, Description: "Target resource UUID", GoType: common.TFTypeString, Required: true,
			}
			common.CalculateSDKType(&f)
			fields = append(fields, f)
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
			f := common.FieldInfo{
				Name: param.Name, Type: param.Type, Description: "Link parameter", GoType: common.GetGoType(param.Type), Required: false,
			}
			common.CalculateSDKType(&f)
			fields = append(fields, f)
		}
	}
	return fields, nil
}

func (b *LinkBuilder) BuildUpdateFields() ([]common.FieldInfo, error) {
	return nil, nil
}

func (b *LinkBuilder) BuildResponseFields() ([]common.FieldInfo, error) {
	fields, err := func() ([]common.FieldInfo, error) {
		if schema, err := b.Parser.GetOperationResponseSchema(b.Ops.Retrieve); err == nil {
			return common.ExtractFields(b.SchemaConfig, schema, true)
		}
		return nil, nil
	}()
	if err != nil {
		return nil, err
	}

	// Update Source and Target fields to be Required and ForceNew
	for i := range fields {
		f := &fields[i]
		isSource := b.Resource.Source != nil && f.Name == b.Resource.Source.Param
		isTarget := b.Resource.Target != nil && f.Name == b.Resource.Target.Param

		if isSource || isTarget {
			f.Required = true
			f.ReadOnly = false
			f.ForceNew = true
		}
	}

	return fields, nil
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

func (b *LinkBuilder) GetTemplateFiles() []string {
	return append(b.BaseBuilder.GetTemplateFiles(), "plugins/link/resource.tmpl")
}
