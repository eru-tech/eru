package tools_factory

import (
	tools "github.com/eru-tech/eru/eru-ai/tools"
	utiltiy "github.com/eru-tech/eru/eru-ai/tools/utility"
	web_scraping "github.com/eru-tech/eru/eru-ai/tools/web_scraping"
)

func GetTool(toolType string) tools.Tooling {
	switch toolType {
	case "PLAYWRIGHT":
		return new(web_scraping.PlaywrightTool)
	case "STRUCTURED_OUTPUT":
		return new(utiltiy.StructuredOutputTool)
	default:
		return new(tools.Tool)
	}
}
