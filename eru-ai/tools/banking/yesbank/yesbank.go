package yesbank

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"time"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	server "github.com/eru-tech/eru/eru-server/server"
	utils "github.com/eru-tech/eru/eru-utils"
)

type FundConfirmationParams struct {
	ConsentId               string `json:"consent_id" eru:"required" desc:"Consent ID"`
	Identification          string `json:"identification" eru:"required" desc:"Account identification number"`
	SecondaryIdentification string `json:"secondary_identification" eru:"required" desc:"Secondary identification"`
}

type InitiatePaymentsParams struct {
	FileIdentifier          string            `json:"file_identifier" eru:"required" desc:"File identifier"`
	NumberOfTransactions    string            `json:"number_of_transactions" eru:"required" desc:"Number of transactions"`
	ConsentId               string            `json:"consent_id" eru:"required" desc:"Consent ID"`
	ControSum               string            `json:"contro_sum" eru:"required" desc:"Control sum"`
	SecondaryIdentification string            `json:"secondary_identification" eru:"required" desc:"Secondary identification"`
	DomesticPayments        []DomesticPayment `json:"domestic_payments" eru:"required" desc:"Domestic payments array"`
}

type PaymentStatusParams struct {
	FileIdentifier          string `json:"file_identifier" eru:"required" desc:"File identifier"`
	SecondaryIdentification string `json:"secondary_identification" eru:"required" desc:"Secondary identification"`
	ConsentId               string `json:"consent_id" eru:"required" desc:"Consent ID"`
}

type InstrumentStatusParams struct {
	InstrId                 string `json:"instr_id" eru:"required" desc:"Instruction ID"`
	SecondaryIdentification string `json:"secondary_identification" eru:"required" desc:"Secondary identification"`
	ConsentId               string `json:"consent_id" eru:"required" desc:"Consent ID"`
}

type YesBankTool struct {
	tools.Tool
	BaseUrl      string `json:"base_url"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	FtxId        string `json:"ftx_id"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	ClientCert   string `json:"client_cert" desc:"PEM encoded client certificate"`
	ClientKey    string `json:"client_key" desc:"PEM encoded client private key"`
}

type FundConfirmationRequest struct {
	Data struct {
		DebtorAccount struct {
			ConsentId               string `json:"ConsentId"`
			Identification          string `json:"Identification"`
			SecondaryIdentification string `json:"SecondaryIdentification"`
		} `json:"DebtorAccount"`
	} `json:"Data"`
}

type Unstructured struct {
	ContactInformation           map[string]interface{} `json:"ContactInformation,omitempty"`
	Identities                   map[string]interface{} `json:"Identities,omitempty"`
	CreditorReferenceInformation string                 `json:"CreditorReferenceInformation,omitempty"`
	RemitterAccount              string                 `json:"RemitterAccount,omitempty"`
}

type InstructedAmount struct {
	Amount   string `json:"Amount"`
	Currency string `json:"Currency"`
}

type DebtorAccount struct {
	Identification          string        `json:"Identification"`
	Name                    string        `json:"Name,omitempty"`
	SecondaryIdentification string        `json:"SecondaryIdentification"`
	Unstructured            *Unstructured `json:"Unstructured,omitempty"`
}

type CreditorAccount struct {
	SchemeName      string        `json:"SchemeName"`
	Identification  string        `json:"Identification"`
	Name            string        `json:"Name"`
	BeneficiaryCode string        `json:"BeneficiaryCode,omitempty"`
	Unstructured    *Unstructured `json:"Unstructured,omitempty"`
}

type RemittanceInformation struct {
	Unstructured *Unstructured `json:"Unstructured,omitempty"`
}

type DeliveryAddress struct {
	AddressLine []string `json:"AddressLine,omitempty"`
}

type Risk struct {
	PaymentContextCode string           `json:"PaymentContextCode,omitempty"`
	DeliveryAddress    *DeliveryAddress `json:"DeliveryAddress,omitempty"`
}

type Initiation struct {
	InstructionIdentification    string                 `json:"InstructionIdentification"`
	ClearingSystemIdentification string                 `json:"ClearingSystemIdentification"`
	InstructedAmount             InstructedAmount       `json:"InstructedAmount"`
	DebtorAccount                DebtorAccount          `json:"DebtorAccount"`
	CreditorAccount              CreditorAccount        `json:"CreditorAccount"`
	RemittanceInformation        *RemittanceInformation `json:"RemittanceInformation,omitempty"`
}

type DomesticPayment struct {
	ConsentId  string     `json:"ConsentId"`
	Initiation Initiation `json:"Initiation"`
	Risk       *Risk      `json:"Risk,omitempty"`
}

type InitiatePaymentsRequest struct {
	Data struct {
		FileIdentifier          string            `json:"FileIdentifier"`
		NumberOfTransactions    string            `json:"NumberOfTransactions"`
		ConsentId               string            `json:"ConsentId"`
		ControSum               string            `json:"ControSum"`
		SecondaryIdentification string            `json:"SecondaryIdentification"`
		DomesticPayments        []DomesticPayment `json:"DomesticPayments"`
	} `json:"Data"`
}

type PaymentStatusRequest struct {
	Data struct {
		FileIdentifier          string `json:"FileIdentifier"`
		SecondaryIdentification string `json:"SecondaryIdentification"`
		ConsentId               string `json:"ConsentId"`
	} `json:"Data"`
}

type InstrumentStatusRequest struct {
	Data struct {
		InstrId                 string `json:"InstrId"`
		SecondaryIdentification string `json:"SecondaryIdentification"`
		ConsentId               string `json:"ConsentId"`
	} `json:"Data"`
}

const (
	FundConfirmationAction = "fund-confirmation"
	InitiatePaymentsAction = "initiate-payments"
	PaymentStatusAction    = "payment-status"
	InstrumentStatusAction = "instrument-status"
)

var yesBankToolActions = []tools.ToolAction{
	{
		ActionName:   FundConfirmationAction,
		Description:  "Confirm fund availability in debtor account",
		SystemPrompt: "Confirm fund availability in debtor account using Yes Bank API",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(FundConfirmationParams{}), []string{})
		},
	},
	{
		ActionName:   InitiatePaymentsAction,
		Description:  "Initiate file payments with domestic payments",
		SystemPrompt: "Initiate file payments with domestic payments using Yes Bank API",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(InitiatePaymentsParams{}), []string{})
		},
	},
	{
		ActionName:   PaymentStatusAction,
		Description:  "Get payment status and details for a file payment",
		SystemPrompt: "Get payment status and details for a file payment using Yes Bank API",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(PaymentStatusParams{}), []string{})
		},
	},
	{
		ActionName:   InstrumentStatusAction,
		Description:  "Get payment status and details for a domestic payment instrument",
		SystemPrompt: "Get payment status and details for a domestic payment instrument using Yes Bank API",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(InstrumentStatusParams{}), []string{})
		},
	},
}

func (yesBankTool *YesBankTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(yesBankToolActions))
	for i, action := range yesBankToolActions {
		infos[i] = tools.ActionInfo{Name: action.ActionName, Description: action.Description}
	}
	return infos
}

func (yesBankTool *YesBankTool) GetActions() []tools.ToolAction {
	return yesBankToolActions
}

func (yesBankTool *YesBankTool) GetSpec() tools.Tooling {
	return yesBankTool
}

func (yesBankTool *YesBankTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &yesBankTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (yesBankTool *YesBankTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("YesBankTool Execute - Start")
	var toolRequest interface{}
	switch actionName {
	case FundConfirmationAction:
		toolResult, toolRequest, persistStore, err = yesBankTool.ExecuteFundConfirmation(ctx, params)
	case InitiatePaymentsAction:
		toolResult, toolRequest, persistStore, err = yesBankTool.ExecuteInitiatePayments(ctx, params)
	case PaymentStatusAction:
		toolResult, toolRequest, persistStore, err = yesBankTool.ExecutePaymentStatus(ctx, params)
	case InstrumentStatusAction:
		toolResult, toolRequest, persistStore, err = yesBankTool.ExecuteInstrumentStatus(ctx, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}

	gm := server.GetGlobalGoroutineManager(ctx)
	gm.SafeGoWithRestartBehavior("tool-post-execute-hook", func(bgCtx context.Context) {
		claims := ctx.Value("claims")
		if claims != nil {
			bgCtx = context.WithValue(bgCtx, "claims", claims)
		}
		efurl := ctx.Value(tools.EruFuncBaseUrlKey)
		if efurl == nil {
			err = errors.New("erufuncbaseurl not found in context")
			logs.WithContext(ctx).Error(err.Error())
			return
		}
		efurlString, ok := efurl.(string)
		if !ok {
			err = errors.New("erufuncbaseurl is not a string")
			logs.WithContext(ctx).Error(err.Error())
			return
		} else {
			bgCtx = context.WithValue(bgCtx, tools.EruFuncBaseUrlKey, efurlString)
		}

		body := make(map[string]interface{})
		if toolRequest != nil {
			body["request"] = toolRequest
		}
		if toolResult != nil {
			body["response"] = toolResult
		}
		body["tenant_id"] = tenantId
		body["project_id"] = projectId

		if params["metadata"] != nil {
			body["metadata"] = params["metadata"]
		}

		hookResult, err := yesBankTool.ExecuteHook(bgCtx, "poex", actionName, projectId, tenantId, body, nil)
		if err != nil {
			logs.WithContext(bgCtx).Error(err.Error())
			return
		}
		logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
	}, server.ContinueOnMaxRetries)

	return toolResult, persistStore, err
}

func (yesBankTool *YesBankTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	err := json.Unmarshal(toolObjJson, &yesBankTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return yesBankTool, nil
}

func (yesBankTool *YesBankTool) ExecuteFundConfirmation(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("YesBankTool ExecuteFundConfirmation - Start")

	if yesBankTool.BaseUrl == "" {
		return nil, nil, false, errors.New("base_url not configured")
	}

	yesBankParams := FundConfirmationParams{}
	yesBankParamsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling yesbank params: %w", err)
	}

	err = json.Unmarshal(yesBankParamsBytes, &yesBankParams)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, nil, false, err
	}

	url := fmt.Sprintf("%s/api-banking/v2.0/domestic-payments/fund-confirmation", yesBankTool.BaseUrl)

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	if yesBankTool.ClientId != "" {
		headers.Set("X-IBM-Client-Id", yesBankTool.ClientId)
	}

	if yesBankTool.ClientSecret != "" {
		headers.Set("X-IBM-Client-Secret", yesBankTool.ClientSecret)
	}

	if yesBankTool.FtxId != "" {
		headers.Set("FTXID", yesBankTool.FtxId)
	}

	if yesBankTool.Username != "" && yesBankTool.Password != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(yesBankTool.Username + ":" + yesBankTool.Password))
		headers.Set("Authorization", "Basic "+auth)
	}

	requestBody := FundConfirmationRequest{}
	requestBody.Data.DebtorAccount.ConsentId = yesBankParams.ConsentId
	requestBody.Data.DebtorAccount.Identification = yesBankParams.Identification
	requestBody.Data.DebtorAccount.SecondaryIdentification = yesBankParams.SecondaryIdentification

	res, _, _, _, err := utils.CallHttpWithTLS(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, requestBody, yesBankTool.ClientCert, yesBankTool.ClientKey, 30*time.Second)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["fund_confirmation"] = res

	return toolResult, map[string]interface{}{"body": requestBody}, true, nil
}

func (yesBankTool *YesBankTool) ExecuteInitiatePayments(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("YesBankTool ExecuteInitiatePayments - Start")

	if yesBankTool.BaseUrl == "" {
		return nil, nil, false, errors.New("base_url not configured")
	}

	initiateParams := InitiatePaymentsParams{}
	initiateParamsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling yesbank params: %w", err)
	}

	err = json.Unmarshal(initiateParamsBytes, &initiateParams)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, nil, false, err
	}

	url := fmt.Sprintf("%s/api-banking/v2.0/file-payments", yesBankTool.BaseUrl)

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	if yesBankTool.ClientId != "" {
		headers.Set("X-IBM-Client-Id", yesBankTool.ClientId)
	}

	if yesBankTool.ClientSecret != "" {
		headers.Set("X-IBM-Client-Secret", yesBankTool.ClientSecret)
	}

	if yesBankTool.FtxId != "" {
		headers.Set("FTXID", yesBankTool.FtxId)
	}

	requestBody := InitiatePaymentsRequest{}
	requestBody.Data.FileIdentifier = initiateParams.FileIdentifier
	requestBody.Data.NumberOfTransactions = initiateParams.NumberOfTransactions
	requestBody.Data.ConsentId = initiateParams.ConsentId
	requestBody.Data.ControSum = initiateParams.ControSum
	requestBody.Data.SecondaryIdentification = initiateParams.SecondaryIdentification
	requestBody.Data.DomesticPayments = initiateParams.DomesticPayments

	res, _, _, _, err := utils.CallHttpWithTLS(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, requestBody, yesBankTool.ClientCert, yesBankTool.ClientKey, 30*time.Second)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["initiate_payments"] = res

	return toolResult, map[string]interface{}{"body": requestBody}, true, nil
}

func (yesBankTool *YesBankTool) ExecutePaymentStatus(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("YesBankTool ExecutePaymentStatus - Start")

	if yesBankTool.BaseUrl == "" {
		return nil, nil, false, errors.New("base_url not configured")
	}

	paymentStatusParams := PaymentStatusParams{}
	paymentStatusParamsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling yesbank params: %w", err)
	}

	err = json.Unmarshal(paymentStatusParamsBytes, &paymentStatusParams)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, nil, false, err
	}

	url := fmt.Sprintf("%s/api-banking/v2.0/file-payments/payment-details", yesBankTool.BaseUrl)

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	if yesBankTool.ClientId != "" {
		headers.Set("X-IBM-Client-Id", yesBankTool.ClientId)
	}

	if yesBankTool.ClientSecret != "" {
		headers.Set("X-IBM-Client-Secret", yesBankTool.ClientSecret)
	}

	if yesBankTool.FtxId != "" {
		headers.Set("FTXID", yesBankTool.FtxId)
	}

	requestBody := PaymentStatusRequest{}
	requestBody.Data.FileIdentifier = paymentStatusParams.FileIdentifier
	requestBody.Data.SecondaryIdentification = paymentStatusParams.SecondaryIdentification
	requestBody.Data.ConsentId = paymentStatusParams.ConsentId

	res, _, _, _, err := utils.CallHttpWithTLS(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, requestBody, yesBankTool.ClientCert, yesBankTool.ClientKey, 30*time.Second)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["payment_status"] = res

	return toolResult, map[string]interface{}{"body": requestBody}, true, nil
}

func (yesBankTool *YesBankTool) ExecuteInstrumentStatus(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("YesBankTool ExecuteInstrumentStatus - Start")

	if yesBankTool.BaseUrl == "" {
		return nil, nil, false, errors.New("base_url not configured")
	}

	instrumentStatusParams := InstrumentStatusParams{}
	instrumentStatusParamsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling yesbank params: %w", err)
	}

	err = json.Unmarshal(instrumentStatusParamsBytes, &instrumentStatusParams)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, nil, false, err
	}

	url := fmt.Sprintf("%s/api-banking/v2.0/domestic-payments/payment-details", yesBankTool.BaseUrl)

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	if yesBankTool.ClientId != "" {
		headers.Set("X-IBM-Client-Id", yesBankTool.ClientId)
	}

	if yesBankTool.ClientSecret != "" {
		headers.Set("X-IBM-Client-Secret", yesBankTool.ClientSecret)
	}

	if yesBankTool.FtxId != "" {
		headers.Set("FTXID", yesBankTool.FtxId)
	}

	requestBody := InstrumentStatusRequest{}
	requestBody.Data.InstrId = instrumentStatusParams.InstrId
	requestBody.Data.SecondaryIdentification = instrumentStatusParams.SecondaryIdentification
	requestBody.Data.ConsentId = instrumentStatusParams.ConsentId

	res, _, _, _, err := utils.CallHttpWithTLS(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, requestBody, yesBankTool.ClientCert, yesBankTool.ClientKey, 30*time.Second)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["instrument_status"] = res

	return toolResult, map[string]interface{}{"body": requestBody}, true, nil
}

func (yesBankTool *YesBankTool) GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "tool_name":
		return yesBankTool.ToolName, nil
	case "tool_type":
		return yesBankTool.ToolType, nil
	case "system_prompt":
		return yesBankTool.SystemPrompt, nil
	case "output_schema":
		return yesBankTool.OutputSchema, nil
	case "parameters":
		return yesBankTool.Parameters, nil
	case "description":
		return yesBankTool.Description, nil
	case "base_url":
		return yesBankTool.BaseUrl, nil
	case "client_id":
		return yesBankTool.ClientId, nil
	case "client_secret":
		return yesBankTool.ClientSecret, nil
	case "ftx_id":
		return yesBankTool.FtxId, nil
	case "username":
		return yesBankTool.Username, nil
	case "password":
		return yesBankTool.Password, nil
	case "client_cert":
		return yesBankTool.ClientCert, nil
	case "client_key":
		return yesBankTool.ClientKey, nil
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func (yesBankTool *YesBankTool) SetAttribute(ctx context.Context, attributeName string, attributeValue interface{}) (err error) {
	switch attributeName {
	case "tool_name":
		yesBankTool.ToolName = attributeValue.(string)
	case "tool_type":
		yesBankTool.ToolType = attributeValue.(string)
	case "system_prompt":
		yesBankTool.SystemPrompt = attributeValue.(string)
	case "output_schema":
		yesBankTool.OutputSchema = attributeValue.(eru_models.JSONSchema)
	case "parameters":
		yesBankTool.Parameters = attributeValue.(eru_models.JSONSchema)
	case "description":
		yesBankTool.Description = attributeValue.(string)
	case "base_url":
		yesBankTool.BaseUrl = attributeValue.(string)
	case "client_id":
		yesBankTool.ClientId = attributeValue.(string)
	case "client_secret":
		yesBankTool.ClientSecret = attributeValue.(string)
	case "ftx_id":
		yesBankTool.FtxId = attributeValue.(string)
	case "username":
		yesBankTool.Username = attributeValue.(string)
	case "password":
		yesBankTool.Password = attributeValue.(string)
	case "client_cert":
		yesBankTool.ClientCert = attributeValue.(string)
	case "client_key":
		yesBankTool.ClientKey = attributeValue.(string)
	default:
		err = errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (yesBankTool *YesBankTool) GetBytes(ctx context.Context) ([]byte, error) {
	toolJson, err := json.Marshal(yesBankTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}

func (yesBankTool *YesBankTool) SetToolAction(actionName string) {
	for _, action := range yesBankToolActions {
		if action.ActionName == actionName {
			yesBankTool.ToolAction = action
			return
		}
	}
	yesBankTool.ToolAction = tools.ToolAction{}
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		Public:      false,
		ToolType:    "YesBank",
		Category:    "Banking",
		Description: "Yes Bank payment and banking operations including fund confirmation, NEFT, and account inquiries",
		Actions: func() []tools.ActionInfo {
			infos := make([]tools.ActionInfo, len(yesBankToolActions))
			for i, a := range yesBankToolActions {
				infos[i] = tools.ActionInfo{Name: a.ActionName, Description: a.Description}
			}
			return infos
		}(),
		OAuthEnabled: false,
		Icon:         "PHN2ZyB4bWxuczpkYz0iaHR0cDovL3B1cmwub3JnL2RjL2VsZW1lbnRzLzEuMS8iIHhtbG5zOmNjPSJodHRwOi8vY3JlYXRpdmVjb21tb25zLm9yZy9ucyMiIHhtbG5zOnJkZj0iaHR0cDovL3d3dy53My5vcmcvMTk5OS8wMi8yMi1yZGYtc3ludGF4LW5zIyIgeG1sbnM6c3ZnPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIiB2ZXJzaW9uPSIxLjEiIHdpZHRoPSIzMDAiIGhlaWdodD0iMTEyIiB2aWV3Qm94PSIwIDAgNjk4IDI2MCIgaWQ9InN2ZzIiPjxkZWZzIGlkPSJkZWZzMzQiIC8+PGcgaWQ9ImcyOTk4Ij48ZyBpZD0iZ2ZmZmZmZmZmIj48cGF0aCBkPSJtIDUwOS40OSw5NS4xNCBjIDEuNSwwLjUgMi41NiwxLjcgMy42MSwyLjgyIDE0LjgyLDE1LjYgMjkuNzQsMzEuMDkgNDQuNTMsNDYuNzEgMC4wMywtMTUuNjMgLTAuMDQsLTMxLjI2IDAuMDMsLTQ2Ljg5IDQuMDEsLTAuMDEgOC4wMSwwLjA2IDEyLjAyLC0wLjA3IDAuMzgsMS40MSAwLjU0LDIuODUgMC41LDQuMzEgLTAuMDgsMjMuNzcgMC4wMyw0Ny41NCAtMC4wNSw3MS4zMSAtMC4zLDAuMTQgLTAuODksMC40MSAtMS4xOCwwLjU1IC0xNS45MywtMTYuMiAtMzEuMzUsLTMyLjkxIC00Ny4yNSwtNDkuMTMgLTAuMTUsMTUuNTggMC4xOCwzMS4xOCAtMC4xNiw0Ni43NyAtNC4wNCwtMC4xOCAtOC4wOCwtMC4wOSAtMTIuMTEsLTAuMDcgMC4wMSwtMjUuNDQgLTAuMSwtNTAuODggMC4wNiwtNzYuMzEgeiIgaWQ9InBhdGg1IiBzdHlsZT0iZmlsbDojZmZmZmZmIiAvPjxwYXRoIGQ9Im0gMTkxLjksOTcuMDUgYyA5LjMzLC0zLjQgMTkuMzcsLTAuOTYgMjguNDYsMS45NyAwLjExLDUuNzkgMC4wNSwxMS41NyAwLjA1LDE3LjM2IC01LjU4LC0zLjQgLTExLjYyLC02LjggLTE4LjMxLC02Ljk5IC0zLjM4LC0wLjIyIC03LjUzLDEuODUgLTcuNTksNS42NCAtMC4yMywyLjc3IDEuOTgsNC44OSA0LjA4LDYuMzEgOC4xLDUuMDcgMTcuNTIsOC42NCAyMy43NSwxNi4yIDcuNTgsMTAuMzggMy41NiwyNi45NyAtNy45LDMyLjgxIC0xMC41Nyw1LjYxIC0yMy4xNiwzLjMyIC0zNC4wNiwtMC4wNyAtMC42MywtNi41MiAtMS4wNCwtMTMuMDUgLTEuMywtMTkuNTkgNi4zMSwzLjk0IDEzLjMsNy42MiAyMC45NCw3LjQ5IDMuMjUsLTAuMjIgNy4wOCwtMS4yMSA4LjY4LC00LjM2IDEuNjcsLTIuODMgMC40LC02LjU3IC0yLjA5LC04LjQ3IC03LjE0LC01LjcxIC0xNi4zOCwtOC40OSAtMjIuNjYsLTE1LjMxIC00LjA3LC00LjI1IC02LjEzLC0xMC4yOCAtNS41NiwtMTYuMTMgMC40MiwtNy43NSA2LjI2LC0xNC40OCAxMy41MSwtMTYuODYgeiIgaWQ9InBhdGg3IiBzdHlsZT0iZmlsbDojZmZmZmZmIiAvPjxwYXRoIGQ9Im0gNDU3LjUxLDEwMy42MyBjIDEuNDcsLTIuOTEgMi41MSwtNi4wNSA0LjQ4LC04LjY4IDExLjg1LDI1LjM4IDIzLjI3LDUwLjk2IDM0Ljg4LDc2LjQ2IC00LjQzLDAuMDMgLTguODcsLTAuMDQgLTEzLjMsMC4wNyAtMi4wOCwtNC4zMyAtNCwtOC43MyAtNS45MSwtMTMuMTMgLTExLjA4LC0wLjA2IC0yMi4xNiwtMC4wMiAtMzMuMjQsLTAuMDIgLTEuOTYsNC4zOSAtMy44OCw4LjggLTYuMDEsMTMuMTIgLTQuMzEsLTAuMDYgLTguNjEsLTAuMDIgLTEyLjkxLC0wLjA0IDEwLjc4LC0yMi41MyAyMS4zLC00NS4yIDMyLjAxLC02Ny43OCBtIDIuOTcsMTguNDUgYyAtMy43OCw4LjUxIC03LjM3LDE3LjExIC0xMS4yOSwyNS41NSA3LjkzLDAuMzYgMTUuODksMC4wOCAyMy44MywwLjEzIC0zLjgxLC05IC03Ljc3LC0xNy45NCAtMTEuNTcsLTI2Ljk1IC0wLjI0LDAuMzIgLTAuNzMsMC45NSAtMC45NywxLjI3IHoiIGlkPSJwYXRoOSIgc3R5bGU9ImZpbGw6I2ZmZmZmZiIgLz48cGF0aCBkPSJtIDM4Ljc0LDk3Ljc5IGMgNi44OSwtMC4wNCAxMy43NywwLjA3IDIwLjY1LC0wLjA2IDUuMzcsNy42OCAxMC4yMiwxNS43IDE1LjUyLDIzLjQzIDUuNTQsLTcuODUgMTEuMTMsLTE1LjY4IDE2Ljg2LC0yMy4zOSA2LjA2LDAuMDYgMTIuMTIsLTAuMDMgMTguMTgsMC4wNSAtOS4xOCwxMi44OCAtMTkuMDcsMjUuMjkgLTI4LjAyLDM4LjMgLTAuMTYsMTEuNzUgLTAuMDQsMjMuNTIgLTAuMDUsMzUuMjggLTUuNzUsMC4wNCAtMTEuNSwwLjAyIC0xNy4yNSwwLjAxIEMgNjQuNTQsMTU5Ljc0IDY0LjgyLDE0OC4wNCA2NC41LDEzNi4zOCA1Ni4wOCwxMjMuNDIgNDcuMTMsMTEwLjc5IDM4Ljc0LDk3Ljc5IHoiIGlkPSJwYXRoMTEiIHN0eWxlPSJmaWxsOiNmZmZmZmYiIC8+PHBhdGggZD0ibSAxMTcuNDksMTcxLjQzIGMgMC4wNCwtMjQuNTUgLTAuMDUsLTQ5LjExIDAuMDUsLTczLjY2IDE0LjkzLDAuMDQgMjkuODYsMC4wMSA0NC43OSwwLjAyIDAuMDQsNS4wMSAtMC4wNSwxMC4wMyAwLjA3LDE1LjA0IC05LjMzLDAuMzcgLTE4LjcyLC0wLjMzIC0yOC4wNywwLjMyIDAuMDIsNC40MiAtMC4wMSw4Ljg0IDAuMDQsMTMuMjUgOCwtMC4wMyAxNi4wMSwtMC4xIDI0LjAxLDAuMDUgLTAuMSw0Ljk3IC0wLjAzLDkuOTUgLTAuMDQsMTQuOTQgLTguMDEsLTAuMDEgLTE2LjAxLDAgLTI0LjAxLC0wLjAyIDAuMDMsNC44NCAwLjAxLDkuNjkgMC4wMSwxNC41MyA5LjQ0LDAuMDEgMTguODgsLTAuMDggMjguMzIsMC4wMyAwLjAyLDUuMTcgMC4wNiwxMC4zNCAtMC4wMiwxNS41MiAtMTUuMDUsLTAuMDUgLTMwLjEsLTAuMDMgLTQ1LjE1LC0wLjAyIHoiIGlkPSJwYXRoMTMiIHN0eWxlPSJmaWxsOiNmZmZmZmYiIC8+PHBhdGggZD0ibSAzNjkuNTMsOTcuNzggYyA5LjMxLDAuMjEgMTguNjUsLTAuNDcgMjcuOTIsMC41MiA2LDAuNyAxMi41MywzLjA5IDE1LjY3LDguNjIgMy4zNSw2LjExIDMuNCwxNC41IC0xLjIxLDIwIC0yLjAxLDIuNDkgLTQuOTksMy44MiAtNy43OCw1LjIzIDUuMzYsMS40NyAxMC4xMyw1LjM2IDExLjg1LDEwLjc1IDIuMDMsNi42MyAxLjQ3LDE0LjQ5IC0yLjgxLDIwLjE0IC00Ljc2LDYuMDggLTEyLjg3LDcuOCAtMjAuMTcsOC4yNyAtNy44NCwwLjIyIC0xNS42NywwLjA2IC0yMy41LDAuMTIgMC4wNiwtMjQuNTUgLTAuMDIsLTQ5LjEgMC4wMywtNzMuNjUgbSAxMi4yNCw5Ljg1IGMgMC4wNCw2LjkyIDAuMDIsMTMuODUgMC4wMiwyMC43OCA1LjQ4LC0wLjE1IDExLjU0LDAuODQgMTYuNDYsLTIuMTkgNS44NSwtMy4yOCA2LjU0LC0xMy41IDAuMzUsLTE2Ljc3IC01LjE5LC0yLjYxIC0xMS4yMywtMS42NiAtMTYuODMsLTEuODIgbSAwLjAxLDMwLjYxIGMgMC4wMyw3LjY0IDAuMDEsMTUuMjkgMC4wMiwyMi45MyA1LjU5LC0wLjIgMTEuNTcsMC44MiAxNi44MywtMS41OSA3LjAyLC0zLjIxIDcuOTYsLTE0LjQxIDEuNzcsLTE4Ljg2IC01LjQ1LC0zLjc3IC0xMi40MywtMi4xNCAtMTguNjIsLTIuNDggeiIgaWQ9InBhdGgxNSIgc3R5bGU9ImZpbGw6I2ZmZmZmZiIgLz48cGF0aCBkPSJtIDU5My4zNSwxNzEuNDQgYyAwLjAzLC0yNC41NiAtMC4xOCwtNDkuMTMgMC4xLC03My42OSA0LjEyLDAuMDYgOC4yNCwwLjA0IDEyLjM2LDAuMDIgMC4xNywxMSAwLDIyIDAuMDYsMzMgMTAuMzcsLTExIDIwLjcxLC0yMi4wMiAzMS4xMywtMzIuOTcgNC44OSwtMC4wMyA5Ljc5LC0wLjAxIDE0LjY5LC0wLjAyIC0xMSwxMS44NiAtMjIuMTUsMjMuNTkgLTMzLjI0LDM1LjM4IDEyLjI4LDEyLjcgMjQuNDIsMjUuNTQgMzYuNzIsMzguMjEgLTUuNDcsMC4xMSAtMTAuOTMsMC4wMSAtMTYuNCwwLjA4IC0xMC45MywtMTEuNTEgLTIxLjksLTIyLjk4IC0zMi44OCwtMzQuNDMgLTAuMDYsMTEuNDYgMC4wMiwyMi45MiAtMC4wMywzNC4zOSAtNC4xNywwLjAzIC04LjM0LC0wLjAxIC0xMi41MSwwLjAzIHoiIGlkPSJwYXRoMTciIHN0eWxlPSJmaWxsOiNmZmZmZmYiIC8+PC9nPjxnIGlkPSJnYzQyNjFiZmYiPjxwYXRoIGQ9Im0gMzcyLjIxLDIwLjE5IGMgMy41OCwtNC43IDYuODcsLTkuNjQgMTAuNzEsLTE0LjE1IC0zNy40OCw2Ni4yNCAtNzQuNjksMTMyLjY1IC0xMTIuMDksMTk4Ljk0IC05LjMyLDE2LjcyIC0xOC44NywzMy4zMiAtMjguMDksNTAuMDkgLTAuODEsLTAuNDMgLTEuNTQsLTAuOTcgLTIuMTgsLTEuNjIgLTEzLjE2LC0xMy40MSAtMjYuMzksLTI2Ljc0IC0zOS41NywtNDAuMTMgOS4zNiwtMC4xIDE4LjcxLC0wLjAxIDI4LjA3LC0wLjA1IEMgMjc2Ljc4LDE0OC45MiAzMjQuNSw4NC41NiAzNzIuMjEsMjAuMTkgeiIgaWQ9InBhdGgyMCIgc3R5bGU9ImZpbGw6I2M0MjYxYiIgLz48L2c+PGcgaWQ9ImcwMDUxOTJmZiI+PHBhdGggZD0ibSAyLjMyLDYwLjg3IGMgMTExLjg1LC0wLjE2IDIyMy42OSwwLjAxIDMzNS41NCwtMC4wOCAtMzcuMiw0OS4xMyAtNzQuMTQsOTguNDYgLTExMS41MSwxNDcuNDYgQyAxNTEuNywyMDguMTkgNzcuMDUsMjA4LjIxIDIuNCwyMDguMjggMi4yMywxNTkuMTQgMi4zOSwxMTAuMDEgMi4zMiw2MC44NyBNIDE5MS45LDk3LjA1IGMgLTcuMjUsMi4zOCAtMTMuMDksOS4xMSAtMTMuNTEsMTYuODYgLTAuNTcsNS44NSAxLjQ5LDExLjg4IDUuNTYsMTYuMTMgNi4yOCw2LjgyIDE1LjUyLDkuNiAyMi42NiwxNS4zMSAyLjQ5LDEuOSAzLjc2LDUuNjQgMi4wOSw4LjQ3IC0xLjYsMy4xNSAtNS40Myw0LjE0IC04LjY4LDQuMzYgLTcuNjQsMC4xMyAtMTQuNjMsLTMuNTUgLTIwLjk0LC03LjQ5IDAuMjYsNi41NCAwLjY3LDEzLjA3IDEuMywxOS41OSAxMC45LDMuMzkgMjMuNDksNS42OCAzNC4wNiwwLjA3IDExLjQ2LC01Ljg0IDE1LjQ4LC0yMi40MyA3LjksLTMyLjgxIC02LjIzLC03LjU2IC0xNS42NSwtMTEuMTMgLTIzLjc1LC0xNi4yIC0yLjEsLTEuNDIgLTQuMzEsLTMuNTQgLTQuMDgsLTYuMzEgMC4wNiwtMy43OSA0LjIxLC01Ljg2IDcuNTksLTUuNjQgNi42OSwwLjE5IDEyLjczLDMuNTkgMTguMzEsNi45OSAwLC01Ljc5IDAuMDYsLTExLjU3IC0wLjA1LC0xNy4zNiAtOS4wOSwtMi45MyAtMTkuMTMsLTUuMzcgLTI4LjQ2LC0xLjk3IE0gMzguNzQsOTcuNzkgYyA4LjM5LDEzIDE3LjM0LDI1LjYzIDI1Ljc2LDM4LjU5IDAuMzIsMTEuNjYgMC4wNCwyMy4zNiAwLjEzLDM1LjAzIDUuNzUsMC4wMSAxMS41LDAuMDMgMTcuMjUsLTAuMDEgMC4wMSwtMTEuNzYgLTAuMTEsLTIzLjUzIDAuMDUsLTM1LjI4IDguOTUsLTEzLjAxIDE4Ljg0LC0yNS40MiAyOC4wMiwtMzguMyAtNi4wNiwtMC4wOCAtMTIuMTIsMC4wMSAtMTguMTgsLTAuMDUgLTUuNzMsNy43MSAtMTEuMzIsMTUuNTQgLTE2Ljg2LDIzLjM5IC01LjMsLTcuNzMgLTEwLjE1LC0xNS43NSAtMTUuNTIsLTIzLjQzIC02Ljg4LDAuMTMgLTEzLjc2LDAuMDIgLTIwLjY1LDAuMDYgbSA3OC43NSw3My42NCBjIDE1LjA1LC0wLjAxIDMwLjEsLTAuMDMgNDUuMTUsMC4wMiAwLjA4LC01LjE4IDAuMDQsLTEwLjM1IDAuMDIsLTE1LjUyIC05LjQ0LC0wLjExIC0xOC44OCwtMC4wMiAtMjguMzIsLTAuMDMgMCwtNC44NCAwLjAyLC05LjY5IC0wLjAxLC0xNC41MyA4LDAuMDIgMTYsMC4wMSAyNC4wMSwwLjAyIDAuMDEsLTQuOTkgLTAuMDYsLTkuOTcgMC4wNCwtMTQuOTQgLTgsLTAuMTUgLTE2LjAxLC0wLjA4IC0yNC4wMSwtMC4wNSAtMC4wNSwtNC40MSAtMC4wMiwtOC44MyAtMC4wNCwtMTMuMjUgOS4zNSwtMC42NSAxOC43NCwwLjA1IDI4LjA3LC0wLjMyIC0wLjEyLC01LjAxIC0wLjAzLC0xMC4wMyAtMC4wNywtMTUuMDQgLTE0LjkzLC0wLjAxIC0yOS44NiwwLjAyIC00NC43OSwtMC4wMiAtMC4xLDI0LjU1IC0wLjAxLDQ5LjExIC0wLjA1LDczLjY2IHoiIGlkPSJwYXRoMjMiIHN0eWxlPSJmaWxsOiMwMDUxOTIiIC8+PHBhdGggZD0ibSAzMzEuNzYsMTAzLjY3IGMgOC4wNywtMTQuMjUgMTUuNiwtMjguODIgMjMuOTEsLTQyLjkxIDExMi45NCwwLjA3IDIyNS44OSwwLjEyIDMzOC44MywtMC4wMyAwLjM2LDQ5LjE3IDAuMSw5OC4zNiAwLjEzLDE0Ny41NCAtMTQwLjE2LC0wLjA5IC0yODAuMzEsLTAuMDIgLTQyMC40NywtMC4wMyAxOS4wNywtMzQuOTMgMzguNDQsLTY5LjY5IDU3LjYsLTEwNC41NyBtIDE3Ny43MywtOC41MyBjIC0wLjE2LDI1LjQzIC0wLjA1LDUwLjg3IC0wLjA2LDc2LjMxIDQuMDMsLTAuMDIgOC4wNywtMC4xMSAxMi4xMSwwLjA3IDAuMzQsLTE1LjU5IDAuMDEsLTMxLjE5IDAuMTYsLTQ2Ljc3IDE1LjksMTYuMjIgMzEuMzIsMzIuOTMgNDcuMjUsNDkuMTMgMC4yOSwtMC4xNCAwLjg4LC0wLjQxIDEuMTgsLTAuNTUgMC4wOCwtMjMuNzcgLTAuMDMsLTQ3LjU0IDAuMDUsLTcxLjMxIDAuMDQsLTEuNDYgLTAuMTIsLTIuOSAtMC41LC00LjMxIC00LjAxLDAuMTMgLTguMDEsMC4wNiAtMTIuMDIsMC4wNyAtMC4wNywxNS42MyAwLDMxLjI2IC0wLjAzLDQ2Ljg5IC0xNC43OSwtMTUuNjIgLTI5LjcxLC0zMS4xMSAtNDQuNTMsLTQ2LjcxIC0xLjA1LC0xLjEyIC0yLjExLC0yLjMyIC0zLjYxLC0yLjgyIG0gLTUxLjk4LDguNDkgYyAtMTAuNzEsMjIuNTggLTIxLjIzLDQ1LjI1IC0zMi4wMSw2Ny43OCA0LjMsMC4wMiA4LjYsLTAuMDIgMTIuOTEsMC4wNCAyLjEzLC00LjMyIDQuMDUsLTguNzMgNi4wMSwtMTMuMTIgMTEuMDgsMCAyMi4xNiwtMC4wNCAzMy4yNCwwLjAyIDEuOTEsNC40IDMuODMsOC44IDUuOTEsMTMuMTMgNC40MywtMC4xMSA4Ljg3LC0wLjA0IDEzLjMsLTAuMDcgLTExLjYxLC0yNS41IC0yMy4wMywtNTEuMDggLTM0Ljg4LC03Ni40NiAtMS45NywyLjYzIC0zLjAxLDUuNzcgLTQuNDgsOC42OCBtIC04Ny45OCwtNS44NSBjIC0wLjA1LDI0LjU1IDAuMDMsNDkuMSAtMC4wMyw3My42NSA3LjgzLC0wLjA2IDE1LjY2LDAuMSAyMy41LC0wLjEyIDcuMywtMC40NyAxNS40MSwtMi4xOSAyMC4xNywtOC4yNyA0LjI4LC01LjY1IDQuODQsLTEzLjUxIDIuODEsLTIwLjE0IC0xLjcyLC01LjM5IC02LjQ5LC05LjI4IC0xMS44NSwtMTAuNzUgMi43OSwtMS40MSA1Ljc3LC0yLjc0IDcuNzgsLTUuMjMgNC42MSwtNS41IDQuNTYsLTEzLjg5IDEuMjEsLTIwIC0zLjE0LC01LjUzIC05LjY3LC03LjkyIC0xNS42NywtOC42MiAtOS4yNywtMC45OSAtMTguNjEsLTAuMzEgLTI3LjkyLC0wLjUyIG0gMjIzLjgyLDczLjY2IGMgNC4xNywtMC4wNCA4LjM0LDAgMTIuNTEsLTAuMDMgMC4wNSwtMTEuNDcgLTAuMDMsLTIyLjkzIDAuMDMsLTM0LjM5IDEwLjk4LDExLjQ1IDIxLjk1LDIyLjkyIDMyLjg4LDM0LjQzIDUuNDcsLTAuMDcgMTAuOTMsMC4wMyAxNi40LC0wLjA4IC0xMi4zLC0xMi42NyAtMjQuNDQsLTI1LjUxIC0zNi43MiwtMzguMjEgMTEuMDksLTExLjc5IDIyLjI0LC0yMy41MiAzMy4yNCwtMzUuMzggLTQuOSwwLjAxIC05LjgsLTAuMDEgLTE0LjY5LDAuMDIgLTEwLjQyLDEwLjk1IC0yMC43NiwyMS45NyAtMzEuMTMsMzIuOTcgLTAuMDYsLTExIDAuMTEsLTIyIC0wLjA2LC0zMyAtNC4xMiwwLjAyIC04LjI0LDAuMDQgLTEyLjM2LC0wLjAyIC0wLjI4LDI0LjU2IC0wLjA3LDQ5LjEzIC0wLjEsNzMuNjkgeiIgaWQ9InBhdGgyNSIgc3R5bGU9ImZpbGw6IzAwNTE5MiIgLz48cGF0aCBkPSJtIDM4MS43NywxMDcuNjMgYyA1LjYsMC4xNiAxMS42NCwtMC43OSAxNi44MywxLjgyIDYuMTksMy4yNyA1LjUsMTMuNDkgLTAuMzUsMTYuNzcgLTQuOTIsMy4wMyAtMTAuOTgsMi4wNCAtMTYuNDYsMi4xOSAwLC02LjkzIDAuMDIsLTEzLjg2IC0wLjAyLC0yMC43OCB6IiBpZD0icGF0aDI3IiBzdHlsZT0iZmlsbDojMDA1MTkyIiAvPjxwYXRoIGQ9Im0gNDYwLjQ4LDEyMi4wOCBjIDAuMjQsLTAuMzIgMC43MywtMC45NSAwLjk3LC0xLjI3IDMuOCw5LjAxIDcuNzYsMTcuOTUgMTEuNTcsMjYuOTUgLTcuOTQsLTAuMDUgLTE1LjksMC4yMyAtMjMuODMsLTAuMTMgMy45MiwtOC40NCA3LjUxLC0xNy4wNCAxMS4yOSwtMjUuNTUgeiIgaWQ9InBhdGgyOSIgc3R5bGU9ImZpbGw6IzAwNTE5MiIgLz48cGF0aCBkPSJtIDM4MS43OCwxMzguMjQgYyA2LjE5LDAuMzQgMTMuMTcsLTEuMjkgMTguNjIsMi40OCA2LjE5LDQuNDUgNS4yNSwxNS42NSAtMS43NywxOC44NiAtNS4yNiwyLjQxIC0xMS4yNCwxLjM5IC0xNi44MywxLjU5IC0wLjAxLC03LjY0IDAuMDEsLTE1LjI5IC0wLjAyLC0yMi45MyB6IiBpZD0icGF0aDMxIiBzdHlsZT0iZmlsbDojMDA1MTkyIiAvPjwvZz48L2c+PC9zdmc+",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(YesBankTool{}), []string{}),
	})
}
