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
			return utils.StructToJSONSchema(reflect.TypeOf(FundConfirmationParams{}))
		},
	},
	{
		ActionName:   InitiatePaymentsAction,
		Description:  "Initiate file payments with domestic payments",
		SystemPrompt: "Initiate file payments with domestic payments using Yes Bank API",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(InitiatePaymentsParams{}))
		},
	},
	{
		ActionName:   PaymentStatusAction,
		Description:  "Get payment status and details for a file payment",
		SystemPrompt: "Get payment status and details for a file payment using Yes Bank API",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(PaymentStatusParams{}))
		},
	},
	{
		ActionName:   InstrumentStatusAction,
		Description:  "Get payment status and details for a domestic payment instrument",
		SystemPrompt: "Get payment status and details for a domestic payment instrument using Yes Bank API",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(InstrumentStatusParams{}))
		},
	},
}

func (yesBankTool *YesBankTool) GetActionsList() []string {
	actions := []string{}
	for _, action := range yesBankToolActions {
		actions = append(actions, action.ActionName)
	}
	return actions
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
