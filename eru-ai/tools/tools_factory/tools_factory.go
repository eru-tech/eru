package tools_factory

import (
	tools "github.com/eru-tech/eru/eru-ai/tools"
	repositories "github.com/eru-tech/eru/eru-ai/tools/repositories"
	utiltiy "github.com/eru-tech/eru/eru-ai/tools/utility"
	web_scraping "github.com/eru-tech/eru/eru-ai/tools/web_scraping"
)

func GetTool(toolType string) tools.Tooling {
	switch toolType {
	case "PLAYWRIGHT":
		return new(web_scraping.PlaywrightTool)
	case "STRUCTURED_OUTPUT":
		return new(utiltiy.StructuredOutputTool)
	/* case "MCP":
	return new(mcp.MCPToolImpl) */
	case "GITHUB":
		return new(repositories.GithubTool)
	default:
		return new(tools.Tool)
	}
}
