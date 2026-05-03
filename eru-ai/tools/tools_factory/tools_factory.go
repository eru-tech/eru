package tools_factory

import (
	tools "github.com/eru-tech/eru/eru-ai/tools"
	aggregators "github.com/eru-tech/eru/eru-ai/tools/aggregators"
	analytics "github.com/eru-tech/eru/eru-ai/tools/analytics"
	icicibank "github.com/eru-tech/eru/eru-ai/tools/banking/icicibank"
	yesbank "github.com/eru-tech/eru/eru-ai/tools/banking/yesbank"
	ckyc "github.com/eru-tech/eru/eru-ai/tools/ckyc"
	ecomm "github.com/eru-tech/eru/eru-ai/tools/ecomm"
	emails "github.com/eru-tech/eru/eru-ai/tools/emails"
	eru "github.com/eru-tech/eru/eru-ai/tools/eru"
	esign "github.com/eru-tech/eru/eru-ai/tools/esign"
	messengers "github.com/eru-tech/eru/eru-ai/tools/messengers"
	ndml_kyc "github.com/eru-tech/eru/eru-ai/tools/ndml_kyc"
	repositories "github.com/eru-tech/eru/eru-ai/tools/repositories"
	saas "github.com/eru-tech/eru/eru-ai/tools/saas"
	stocks "github.com/eru-tech/eru/eru-ai/tools/stocks"
	telecom "github.com/eru-tech/eru/eru-ai/tools/telecom"
	utiltiy "github.com/eru-tech/eru/eru-ai/tools/utility"
	vectorstore "github.com/eru-tech/eru/eru-ai/tools/vectors"
	web_scraping "github.com/eru-tech/eru/eru-ai/tools/web_scraping"
)

func GetTool(toolType string) tools.Tooling {
	switch toolType {
	case "CYGNET":
		return new(aggregators.CygnetTool)
	case "PERFIOS":
		return new(aggregators.PerfiosTool)
	case "PLAYWRIGHT":
		return new(web_scraping.PlaywrightTool)
	case "STRUCTURED_OUTPUT":
		return new(utiltiy.StructuredOutputTool)
	case "MS_EMAIL":
		return new(emails.MsEmailTool)
	case "GL_EMAIL":
		return new(emails.GlEmailTool)
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
	case "FIREBASE":
		return new(messengers.FirebaseTool)
	case "VECTORSTORE":
		return new(vectorstore.VectorstoreAccount)
	case "ERUQL":
		return new(eru.EruqlTool)
	case "ERUFUNCTIONS":
		return new(eru.ErufunctionsTool)
	case "ERUSTUDIO":
		return new(eru.EruStudioTool)
	case "NDMLKYC":
		return new(ndml_kyc.NdmlTool)
	case "CKYC":
		return new(ckyc.CkycTool)
	case "YESBANK":
		return new(yesbank.YesBankTool)
	case "ICICIBANK":
		return new(icicibank.IciciBankTool)
	case "ZOHODESK":
		return new(saas.ZohoDeskTool)
	case "MASSIVE":
		return new(stocks.MassiveTool)
	case "OZONE":
		return new(telecom.OzoneTool)
	case "PICHAIN":
		return new(esign.PichainTool)
	case "CLEVERTAP":
		return new(analytics.ClevertapTool)
	default:
		return new(tools.Tool)
	}
}
