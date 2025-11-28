package tools_factory

import (
	tools "github.com/eru-tech/eru/eru-ai/tools"
	icicibank "github.com/eru-tech/eru/eru-ai/tools/banking/icicibank"
	yesbank "github.com/eru-tech/eru/eru-ai/tools/banking/yesbank"
	ecomm "github.com/eru-tech/eru/eru-ai/tools/ecomm"
	emails "github.com/eru-tech/eru/eru-ai/tools/emails"
	eru "github.com/eru-tech/eru/eru-ai/tools/eru"
	messengers "github.com/eru-tech/eru/eru-ai/tools/messengers"
	ndml_kyc "github.com/eru-tech/eru/eru-ai/tools/ndml_kyc"
	repositories "github.com/eru-tech/eru/eru-ai/tools/repositories"
	saas "github.com/eru-tech/eru/eru-ai/tools/saas"
	stocks "github.com/eru-tech/eru/eru-ai/tools/stocks"
	utiltiy "github.com/eru-tech/eru/eru-ai/tools/utility"
	vectorstore "github.com/eru-tech/eru/eru-ai/tools/vectors"
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
	case "VECTORSTORE":
		return new(vectorstore.VectorstoreAccount)
	case "ERUQL":
		return new(eru.EruqlTool)
	case "NDMLKYC":
		return new(ndml_kyc.NdmlTool)
	case "YESBANK":
		return new(yesbank.YesBankTool)
	case "ICICIBANK":
		return new(icicibank.IciciBankTool)
	case "ZOHODESK":
		return new(saas.ZohoDeskTool)
	case "MASSIVE":
		return new(stocks.MassiveTool)
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
		"ZOHODESK",
	}
	return tools
}
