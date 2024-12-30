package tools

type Tool struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	SystemPrompt string                 `json:"system_prompt"`
	OutputSchema map[string]interface{} `json:"output_schema"`
}
