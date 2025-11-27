package openapi

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// Parser handles OpenAPI schema parsing
type Parser struct {
	doc *openapi3.T
}

// NewParser creates a new OpenAPI parser
func NewParser(schemaPath string) (*Parser, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI schema: %w", err)
	}

	// Validate the document
	if err := doc.Validate(loader.Context); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI schema: %w", err)
	}

	return &Parser{doc: doc}, nil
}

// GetOperation retrieves an operation by its operation ID
func (p *Parser) GetOperation(operationID string) (*openapi3.Operation, string, string, error) {
	for path, pathItem := range p.doc.Paths.Map() {
		for method, op := range pathItem.Operations() {
			if op.OperationID == operationID {
				return op, path, method, nil
			}
		}
	}
	return nil, "", "", fmt.Errorf("operation not found: %s", operationID)
}

// ValidateOperationExists checks if an operation ID exists in the schema
func (p *Parser) ValidateOperationExists(operationID string) error {
	_, _, _, err := p.GetOperation(operationID)
	return err
}

// GetSchema retrieves a schema by its name from Components.Schemas
func (p *Parser) GetSchema(name string) (*openapi3.SchemaRef, error) {
	if schemaRef, ok := p.doc.Components.Schemas[name]; ok {
		return schemaRef, nil
	}
	return nil, fmt.Errorf("schema not found: %s", name)
}

// GetOperationRequestSchema returns the request body schema for an operation
func (p *Parser) GetOperationRequestSchema(operationID string) (*openapi3.SchemaRef, error) {
	op, _, _, err := p.GetOperation(operationID)
	if err != nil {
		return nil, err
	}

	if op.RequestBody == nil || op.RequestBody.Value == nil {
		return nil, fmt.Errorf("operation %s has no request body", operationID)
	}

	// Look for application/json content
	content := op.RequestBody.Value.Content.Get("application/json")
	if content == nil {
		return nil, fmt.Errorf("operation %s has no application/json request body", operationID)
	}

	return content.Schema, nil
}

// GetOperationResponseSchema returns the success response schema for an operation
func (p *Parser) GetOperationResponseSchema(operationID string) (*openapi3.SchemaRef, error) {
	op, _, _, err := p.GetOperation(operationID)
	if err != nil {
		return nil, err
	}

	// Try 200, 201, 204 status codes
	for _, code := range []string{"200", "201", "204"} {
		resp := op.Responses.Status(StringToInt(code))
		if resp != nil && resp.Value != nil {
			content := resp.Value.Content.Get("application/json")
			if content != nil && content.Schema != nil {
				return content.Schema, nil
			}
		}
	}

	return nil, fmt.Errorf("operation %s has no success response with application/json content", operationID)
}

// StringToInt is a helper to convert string status codes to int
func StringToInt(s string) int {
	codes := map[string]int{
		"200": 200,
		"201": 201,
		"204": 204,
	}
	return codes[s]
}

// Document returns the underlying OpenAPI document
func (p *Parser) Document() *openapi3.T {
	return p.doc
}
