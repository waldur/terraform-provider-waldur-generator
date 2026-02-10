package action

// ActionTemplateData holds data for generating resource action files
type ActionTemplateData struct {
	ResourceName    string
	Service         string
	CleanName       string
	ActionName      string
	OperationID     string
	BaseOperationID string
	Description     string
	IdentifierParam string
	IdentifierDesc  string
	ProviderName    string
	Path            string
	Method          string
}
