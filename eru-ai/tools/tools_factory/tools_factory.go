package tools_factory

import (
	tools "github.com/eru-tech/eru/eru-ai/tools"
	ecomm "github.com/eru-tech/eru/eru-ai/tools/ecomm"
	emails "github.com/eru-tech/eru/eru-ai/tools/emails"
	eru "github.com/eru-tech/eru/eru-ai/tools/eru"
	messengers "github.com/eru-tech/eru/eru-ai/tools/messengers"
	repositories "github.com/eru-tech/eru/eru-ai/tools/repositories"
	sql "github.com/eru-tech/eru/eru-ai/tools/sql"
	utiltiy "github.com/eru-tech/eru/eru-ai/tools/utility"
	web_scraping "github.com/eru-tech/eru/eru-ai/tools/web_scraping"
)

func GetTool(toolType string) tools.Tooling {
	switch toolType {
	case "PLAYWRIGHT":
		return new(web_scraping.PlaywrightTool)
	case "STRUCTURED_OUTPUT":
		return new(utiltiy.StructuredOutputTool)
	case "MS_EMAIL":
		return new(emails.MsEmailTool)
	case "AMAZON":
		return new(ecomm.AmazonTool)
	/* case "MCP":
	return new(mcp.MCPToolImpl) */
	case "GITHUB":
		return new(repositories.GithubTool)
	case "WHATSAPP":
		return new(messengers.WhatsAppTool)
	case "SLACK":
		return new(messengers.SlackTool)
	case "SQL":
		return new(sql.SqlAccount)
	case "ERUQL":
		return new(eru.EruqlTool)
	default:
		return new(tools.Tool)
	}
}

func GetMcpToolNames() []string {
	tools := []string{
		"PLAYWRIGHT",
		"STRUCTURED_OUTPUT",
		"MS_EMAIL",
		"WHATSAPP",
		"SLACK",
	}
	return tools
}
