package icicibank

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	aes "github.com/eru-tech/eru/eru-crypto/aes"
	rsa "github.com/eru-tech/eru/eru-crypto/rsa"
	erusha "github.com/eru-tech/eru/eru-crypto/sha"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
)

type IciciBankTool struct {
	tools.Tool
	ApiKey           string `json:"api_key"`
	BankPublicKey    string `json:"bank_public_key"`
	ClientPrivateKey string `json:"client_private_key"`
	CorpId           string `json:"corp_id"`
	UserId           string `json:"user_id"`
	AggrId           string `json:"aggr_id"`
	URN              string `json:"urn"`
	AggrName         string `json:"aggr_name"`
	BaseUrl          string `json:"base_url"`
}

type RegistrationStatusParams struct {
	CorpId   string `json:"CORPID" eru:"required" desc:"Corporate ID"`
	UserId   string `json:"USERID" eru:"required" desc:"User ID"`
	AggrId   string `json:"AGGRID" eru:"required" desc:"Aggregator ID"`
	URN      string `json:"URN" eru:"required" desc:"Unique Reference Number"`
	AggrName string `json:"AGGRNAME" eru:"required" desc:"Aggregator Name"`
}

type BalanceInquiryParams struct {
	CorpId    string `json:"CORPID" eru:"required" desc:"Corporate ID"`
	UserId    string `json:"USERID" eru:"required" desc:"User ID"`
	AggrId    string `json:"AGGRID" eru:"required" desc:"Aggregator ID"`
	URN       string `json:"URN" eru:"required" desc:"Unique Reference Number"`
	AccountNo string `json:"ACCOUNTNO" eru:"required" desc:"Account Number"`
}

type AccountStatementParams struct {
	AggrId    string `json:"AGGRID" eru:"required" desc:"Aggregator ID"`
	CorpId    string `json:"CORPID" eru:"required" desc:"Corporate ID"`
	UserId    string `json:"USERID" eru:"required" desc:"User ID"`
	AccountNo string `json:"ACCOUNTNO" eru:"required" desc:"Account Number"`
	FromDate  string `json:"FROMDATE" eru:"required" desc:"From Date (DD-MM-YYYY)"`
	ToDate    string `json:"TODATE" eru:"required" desc:"To Date (DD-MM-YYYY)"`
	URN       string `json:"URN" eru:"required" desc:"Unique Reference Number"`
}

type TransactionInquiryParams struct {
	CorpId   string `json:"CORPID" eru:"required" desc:"Corporate ID"`
	UserId   string `json:"USERID" eru:"required" desc:"User ID"`
	AggrId   string `json:"AGGRID" eru:"required" desc:"Aggregator ID"`
	URN      string `json:"URN" eru:"required" desc:"Unique Reference Number"`
	UniqueId string `json:"UNIQUEID" eru:"required" desc:"Unique Transaction ID"`
}

type NeftStatusParams struct {
	CorpId    string `json:"CORPID" eru:"required" desc:"Corporate ID"`
	UserId    string `json:"USERID" eru:"required" desc:"User ID"`
	AggrId    string `json:"AGGRID" eru:"required" desc:"Aggregator ID"`
	URN       string `json:"URN" eru:"required" desc:"Unique Reference Number"`
	UtrNumber string `json:"UTRNUMBER" eru:"required" desc:"UTR Number"`
}

type TransactionParams struct {
	CorpId      string `json:"CORPID" eru:"required" desc:"Corporate ID"`
	UserId      string `json:"USERID" eru:"required" desc:"User ID"`
	AggrId      string `json:"AGGRID" eru:"required" desc:"Aggregator ID"`
	URN         string `json:"URN" eru:"required" desc:"Unique Reference Number"`
	UniqueId    string `json:"UNIQUEID" eru:"required" desc:"Unique Transaction ID"`
	Amount      string `json:"AMOUNT" eru:"required" desc:"Transaction Amount"`
	AggrName    string `json:"AGGRNAME" eru:"required" desc:"Aggregator Name"`
	DebitAcc    string `json:"DEBITACC" eru:"required" desc:"Debit Account Number"`
	CreditAcc   string `json:"CREDITACC" eru:"required" desc:"Credit Account Number"`
	IFSC        string `json:"IFSC" eru:"required" desc:"IFSC Code"`
	Currency    string `json:"CURRENCY" eru:"required" desc:"Currency Code"`
	TxnType     string `json:"TXNTYPE" eru:"required" desc:"Transaction Type"`
	PayeeName   string `json:"PAYEENAME" eru:"required" desc:"Payee Name"`
	Remarks     string `json:"REMARKS" desc:"Transaction Remarks"`
	WorkflowReq string `json:"WORKFLOW_REQD" eru:"required" desc:"Workflow Required (Y/N)"`
}

type EncryptedRequest struct {
	EncryptedData        string `json:"encryptedData" eru:"required"`
	EncryptedKey         string `json:"encryptedKey" eru:"required"`
	Service              string `json:"service"`
	OaepHashingAlgorithm string `json:"oaepHashingAlgorithm"`
	RequestId            string `json:"requestId"`
	Iv                   string `json:"iv"`
	ClientInfo           string `json:"clientInfo"`
	OptionalParam        string `json:"optionalParam"`
}
type EncryptedResponse struct {
	EncryptedData        string `json:"encryptedData"`
	EncryptedKey         string `json:"encryptedKey"`
	Service              string `json:"service"`
	OaepHashingAlgorithm string `json:"oaepHashingAlgorithm"`
	RequestId            string `json:"requestId"`
}

const (
	RegistrationStatusAction = "registration-status"
	BalanceInquiryAction     = "balance-inquiry"
	AccountStatementAction   = "account-statement"
	TransactionInquiryAction = "transaction-inquiry"
	NeftStatusAction         = "neft-status"
	TransactionAction        = "transaction"
)

var iciciBankToolActions = []tools.ToolAction{
	{
		ActionName:   RegistrationStatusAction,
		Description:  "Check registration status with ICICI Bank",
		SystemPrompt: "Check registration status with ICICI Bank API",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(RegistrationStatusParams{}))
		},
	},
	{
		ActionName:   BalanceInquiryAction,
		Description:  "Check account balance with ICICI Bank",
		SystemPrompt: "Check account balance with ICICI Bank API",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(BalanceInquiryParams{}))
		},
	},
	{
		ActionName:   AccountStatementAction,
		Description:  "Get account statement from ICICI Bank",
		SystemPrompt: "Get account statement from ICICI Bank API",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(AccountStatementParams{}))
		},
	},
	{
		ActionName:   TransactionAction,
		Description:  "Initiate a transaction with ICICI Bank",
		SystemPrompt: "Initiate a transaction with ICICI Bank API",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(TransactionParams{}))
		},
	},
	{
		ActionName:   TransactionInquiryAction,
		Description:  "Check transaction status with ICICI Bank",
		SystemPrompt: "Check transaction status with ICICI Bank API",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(TransactionInquiryParams{}))
		},
	},
	{
		ActionName:   NeftStatusAction,
		Description:  "Get NEFT status with ICICI Bank",
		SystemPrompt: "Get NEFT status with ICICI Bank API",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(NeftStatusParams{}))
		},
	},
}

func (iciciBankTool *IciciBankTool) GetActionsList() []string {
	actions := []string{}
	for _, action := range iciciBankToolActions {
		actions = append(actions, action.ActionName)
	}
	return actions
}

func (iciciBankTool *IciciBankTool) GetSpec() tools.Tooling {
	return iciciBankTool
}

func (iciciBankTool *IciciBankTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &iciciBankTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (iciciBankTool *IciciBankTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("IciciBankTool Execute - Start")
	switch actionName {
	case RegistrationStatusAction:
		return iciciBankTool.ExecuteRegistrationStatus(ctx, params)
	case BalanceInquiryAction:
		return iciciBankTool.ExecuteBalanceInquiry(ctx, params)
	case AccountStatementAction:
		return iciciBankTool.ExecuteAccountStatement(ctx, params)
	case TransactionAction:
		return iciciBankTool.ExecuteTransaction(ctx, params)
	case TransactionInquiryAction:
		return iciciBankTool.ExecuteTransactionInquiry(ctx, params)
	case NeftStatusAction:
		return iciciBankTool.ExecuteNeftStatus(ctx, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
}

func (iciciBankTool *IciciBankTool) generateFingerprint(ctx context.Context, payload string, apiKey string, publicKey string) (string, error) {
	sha512Hash := hex.EncodeToString(erusha.NewSHA512([]byte(payload)))
	id_fingerprint := strings.Builder{}
	id_fingerprint.WriteString(apiKey)
	id_fingerprint.WriteString("|")
	id_fingerprint.WriteString(sha512Hash)

	encryptedFingerprintBytes, err := rsa.EncryptWithCert(ctx, []byte(id_fingerprint.String()), publicKey)
	if err != nil {
		return "", fmt.Errorf("error encrypting fingerprint: %w", err)
	}
	encryptedFingerprintB64 := base64.StdEncoding.EncodeToString(encryptedFingerprintBytes)

	return encryptedFingerprintB64, nil
}

func (iciciBankTool *IciciBankTool) encryptRequestPayload(ctx context.Context, requestPayload interface{}) (EncryptedRequest, string, error) {
	requestPayloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return EncryptedRequest{}, "", err
	}
	bankPublicKeyBytes, err := base64.StdEncoding.DecodeString(iciciBankTool.BankPublicKey)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return EncryptedRequest{}, "", err
	}
	bankPublicKey := string(bankPublicKeyBytes)

	randomKey, err := aes.GenerateKey(ctx, 16)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return EncryptedRequest{}, "", err
	}

	requestPayloadBytesPadded := aes.Pad(requestPayloadBytes, 16)
	encryptedRequestPayloadBytes, err := aes.EncryptCBC(ctx, requestPayloadBytesPadded, randomKey.Key, randomKey.Vector)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return EncryptedRequest{}, "", err
	}
	encryptedWithIV := append(randomKey.Vector, encryptedRequestPayloadBytes...)
	encryptedRequestPayloadB64 := base64.StdEncoding.EncodeToString(encryptedWithIV)

	encryptedRandomKeyBytes, err := rsa.EncryptWithCert(ctx, randomKey.Key, bankPublicKey)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return EncryptedRequest{}, "", err
	}
	encryptedRandomKeyB64 := base64.StdEncoding.EncodeToString(encryptedRandomKeyBytes)

	encryptedRequest := EncryptedRequest{
		EncryptedData:        encryptedRequestPayloadB64,
		EncryptedKey:         encryptedRandomKeyB64,
		Service:              "proxyPathSuffix",
		OaepHashingAlgorithm: "NONE",
		RequestId:            "",
		Iv:                   "",
		ClientInfo:           "",
		OptionalParam:        "",
	}
	requestPayloadStr := string(requestPayloadBytes)
	encryptedFingerprintB64, err := iciciBankTool.generateFingerprint(ctx, requestPayloadStr, iciciBankTool.ApiKey, bankPublicKey)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return EncryptedRequest{}, "", err
	}
	return encryptedRequest, encryptedFingerprintB64, nil
}

func (iciciBankTool *IciciBankTool) decryptResponsePayload(ctx context.Context, responsePayload interface{}) ([]byte, error) {
	var encryptedResp EncryptedResponse
	resBytes, err := json.Marshal(responsePayload)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	if err := json.Unmarshal(resBytes, &encryptedResp); err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	err = utils.ValidateStruct(ctx, encryptedResp, "")
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	if encryptedResp.EncryptedData == "" || encryptedResp.EncryptedKey == "" {
		err = logs.Err(ctx, errors.New("response is not a valid EncryptedResponse"), "")
		return nil, err
	}

	encryptedResponseKeyBytes, err := base64.StdEncoding.DecodeString(encryptedResp.EncryptedKey)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	encryptedResponseDataBytes, err := base64.StdEncoding.DecodeString(encryptedResp.EncryptedData)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	clientPrivateKeyBytes, err := base64.StdEncoding.DecodeString(iciciBankTool.ClientPrivateKey)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	clientPrivateKey := string(clientPrivateKeyBytes)
	decryptedResponseKeyBytes, err := rsa.DecryptPKCS1v15(ctx, encryptedResponseKeyBytes, clientPrivateKey)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}

	iv := encryptedResponseDataBytes[:16]
	data := encryptedResponseDataBytes[16:]

	decryptedResponseDataBytes, err := aes.DecryptCBC(ctx, data, decryptedResponseKeyBytes, iv)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	decryptedResponseDataBytesUnpadded, err := aes.Unpad(decryptedResponseDataBytes)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return decryptedResponseDataBytesUnpadded, nil
}

func (iciciBankTool *IciciBankTool) ExecuteRegistrationStatus(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("IciciBankTool ExecuteRegistrationStatus - Start")

	regStatusParams := RegistrationStatusParams{
		CorpId:   iciciBankTool.CorpId,
		UserId:   iciciBankTool.UserId,
		AggrId:   iciciBankTool.AggrId,
		URN:      iciciBankTool.URN,
		AggrName: iciciBankTool.AggrName,
	}

	requestPayload, encryptedFingerprintB64, err := iciciBankTool.encryptRequestPayload(ctx, regStatusParams)
	if err != nil {
		return nil, false, err
	}

	url := fmt.Sprintf("%s/v1/RegistrationStatus", iciciBankTool.BaseUrl)

	headers := http.Header{}
	headers.Set("Content-Type", "application/json; charset=utf-8")
	headers.Set("X-SDK-Originated", "true")
	headers.Set("X-SDK-Version", "1.0")
	headers.Set("X-ID-Fingerprint", encryptedFingerprintB64)

	if iciciBankTool.ApiKey != "" {
		headers.Set("apikey", iciciBankTool.ApiKey)
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, requestPayload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	decryptedResponseDataBytesUnpadded, err := iciciBankTool.decryptResponsePayload(ctx, res)
	if err != nil {
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	decryptedResponse := make(map[string]interface{})
	if err := json.Unmarshal(decryptedResponseDataBytesUnpadded, &decryptedResponse); err != nil {
		toolResult["registration_status"] = string(decryptedResponseDataBytesUnpadded)
	} else {
		toolResult["registration_status"] = decryptedResponse
	}
	return toolResult, true, nil
}

func (iciciBankTool *IciciBankTool) ExecuteBalanceInquiry(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("IciciBankTool ExecuteBalanceInquiry - Start")

	accountNo := ""
	ok := false
	if accountNo, ok = params["account_no"].(string); !ok {
		err = logs.Err(ctx, errors.New("account_no is required"), "")
		return nil, false, err
	}

	balanceInquiryParams := BalanceInquiryParams{
		CorpId:    iciciBankTool.CorpId,
		UserId:    iciciBankTool.UserId,
		AggrId:    iciciBankTool.AggrId,
		URN:       iciciBankTool.URN,
		AccountNo: accountNo,
	}
	x, _ := json.Marshal(balanceInquiryParams)
	logs.WithContext(ctx).Info(fmt.Sprintf("IciciBankTool ExecuteBalanceInquiry - balanceInquiryParams: %s", string(x)))
	requestPayload, encryptedFingerprintB64, err := iciciBankTool.encryptRequestPayload(ctx, balanceInquiryParams)
	if err != nil {
		return nil, false, err
	}
	y, _ := json.Marshal(requestPayload)
	logs.WithContext(ctx).Info(fmt.Sprintf("IciciBankTool ExecuteBalanceInquiry - requestPayload: %s", string(y)))
	logs.WithContext(ctx).Info(fmt.Sprintf("IciciBankTool ExecuteBalanceInquiry - encryptedFingerprintB64: %s", encryptedFingerprintB64))

	url := fmt.Sprintf("%s/v1/BalanceInquiry", iciciBankTool.BaseUrl)

	headers := http.Header{}
	headers.Set("Content-Type", "application/json; charset=utf-8")
	headers.Set("X-SDK-Originated", "true")
	headers.Set("X-SDK-Version", "1.0")
	headers.Set("X-ID-Fingerprint", encryptedFingerprintB64)

	if iciciBankTool.ApiKey != "" {
		headers.Set("apikey", iciciBankTool.ApiKey)
	}

	res, _, _, sc, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, requestPayload)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf(err.Error(), " :status code = %d", sc))
		return nil, false, err
	}

	decryptedResponseDataBytesUnpadded, err := iciciBankTool.decryptResponsePayload(ctx, res)
	if err != nil {
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	decryptedResponse := make(map[string]interface{})
	if err := json.Unmarshal(decryptedResponseDataBytesUnpadded, &decryptedResponse); err != nil {
		toolResult["balance_inquiry"] = string(decryptedResponseDataBytesUnpadded)
	} else {
		toolResult["balance_inquiry"] = decryptedResponse
	}
	return toolResult, true, nil
}

func (iciciBankTool *IciciBankTool) ExecuteAccountStatement(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("IciciBankTool ExecuteAccountStatement - Start")

	var accountNo, fromDate, toDate string
	var ok bool

	if accountNo, ok = params["account_no"].(string); !ok || accountNo == "" {
		err = logs.Err(ctx, errors.New("account_no is required"), "")
		return nil, false, err
	}
	if fromDate, ok = params["from_date"].(string); !ok || fromDate == "" {
		err = logs.Err(ctx, errors.New("from_date is required (DD-MM-YYYY)"), "")
		return nil, false, err
	}
	if toDate, ok = params["to_date"].(string); !ok || toDate == "" {
		err = logs.Err(ctx, errors.New("to_date is required (DD-MM-YYYY)"), "")
		return nil, false, err
	}

	accountStatementParams := AccountStatementParams{
		AggrId:    iciciBankTool.AggrId,
		CorpId:    iciciBankTool.CorpId,
		UserId:    iciciBankTool.UserId,
		AccountNo: accountNo,
		FromDate:  fromDate,
		ToDate:    toDate,
		URN:       iciciBankTool.URN,
	}

	requestPayload, encryptedFingerprintB64, err := iciciBankTool.encryptRequestPayload(ctx, accountStatementParams)
	if err != nil {
		return nil, false, err
	}

	url := fmt.Sprintf("%s/v1/AccountStatement", iciciBankTool.BaseUrl)

	headers := http.Header{}
	headers.Set("Content-Type", "application/json; charset=utf-8")
	headers.Set("X-SDK-Originated", "true")
	headers.Set("X-SDK-Version", "1.0")
	headers.Set("X-ID-Fingerprint", encryptedFingerprintB64)

	if iciciBankTool.ApiKey != "" {
		headers.Set("apikey", iciciBankTool.ApiKey)
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, requestPayload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	decryptedResponseDataBytesUnpadded, err := iciciBankTool.decryptResponsePayload(ctx, res)
	if err != nil {
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	decryptedResponse := make(map[string]interface{})
	if err := json.Unmarshal(decryptedResponseDataBytesUnpadded, &decryptedResponse); err != nil {
		toolResult["account_statement"] = string(decryptedResponseDataBytesUnpadded)
	} else {
		toolResult["account_statement"] = decryptedResponse
	}
	return toolResult, true, nil
}

func (iciciBankTool *IciciBankTool) ExecuteTransaction(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("IciciBankTool ExecuteTransaction - Start")

	var uniqueId, amount, debitAcc, creditAcc, ifsc, currency, txnType, payeeName, remarks, workflowReq string
	var ok bool

	if uniqueId, ok = params["unique_id"].(string); !ok || uniqueId == "" {
		err = logs.Err(ctx, errors.New("unique_id is required"), "")
		return nil, false, err
	}
	if amount, ok = params["amount"].(string); !ok || amount == "" {
		err = logs.Err(ctx, errors.New("amount is required"), "")
		return nil, false, err
	}
	if debitAcc, ok = params["debit_acc"].(string); !ok || debitAcc == "" {
		err = logs.Err(ctx, errors.New("debit_acc is required"), "")
		return nil, false, err
	}
	if creditAcc, ok = params["credit_acc"].(string); !ok || creditAcc == "" {
		err = logs.Err(ctx, errors.New("credit_acc is required"), "")
		return nil, false, err
	}
	if ifsc, ok = params["ifsc"].(string); !ok || ifsc == "" {
		err = logs.Err(ctx, errors.New("ifsc is required"), "")
		return nil, false, err
	}
	if currency, ok = params["currency"].(string); !ok || currency == "" {
		currency = "INR"
	}
	if txnType, ok = params["txn_type"].(string); !ok || txnType == "" {
		err = logs.Err(ctx, errors.New("txn_type is required"), "")
		return nil, false, err
	}
	if payeeName, ok = params["payee_name"].(string); !ok || payeeName == "" {
		err = logs.Err(ctx, errors.New("payee_name is required"), "")
		return nil, false, err
	}
	if remarks, ok = params["remarks"].(string); !ok {
		remarks = ""
	}
	if workflowReq, ok = params["workflow_reqd"].(string); !ok || workflowReq == "" {
		workflowReq = "N"
	}

	transactionParams := TransactionParams{
		CorpId:      iciciBankTool.CorpId,
		UserId:      iciciBankTool.UserId,
		AggrId:      iciciBankTool.AggrId,
		URN:         iciciBankTool.URN,
		UniqueId:    uniqueId,
		Amount:      amount,
		AggrName:    iciciBankTool.AggrName,
		DebitAcc:    debitAcc,
		CreditAcc:   creditAcc,
		IFSC:        ifsc,
		Currency:    currency,
		TxnType:     txnType,
		PayeeName:   payeeName,
		Remarks:     remarks,
		WorkflowReq: workflowReq,
	}

	requestPayload, encryptedFingerprintB64, err := iciciBankTool.encryptRequestPayload(ctx, transactionParams)
	if err != nil {
		return nil, false, err
	}

	url := fmt.Sprintf("%s/v1/Transaction", iciciBankTool.BaseUrl)

	headers := http.Header{}
	headers.Set("Content-Type", "application/json; charset=utf-8")
	headers.Set("X-SDK-Originated", "true")
	headers.Set("X-SDK-Version", "1.0")
	headers.Set("X-ID-Fingerprint", encryptedFingerprintB64)

	if iciciBankTool.ApiKey != "" {
		headers.Set("apikey", iciciBankTool.ApiKey)
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, requestPayload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	decryptedResponseDataBytesUnpadded, err := iciciBankTool.decryptResponsePayload(ctx, res)
	if err != nil {
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	decryptedResponse := make(map[string]interface{})
	if err := json.Unmarshal(decryptedResponseDataBytesUnpadded, &decryptedResponse); err != nil {
		toolResult["transaction"] = string(decryptedResponseDataBytesUnpadded)
	} else {
		toolResult["transaction"] = decryptedResponse
	}
	return toolResult, true, nil
}

func (iciciBankTool *IciciBankTool) ExecuteTransactionInquiry(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("IciciBankTool ExecuteTransactionInquiry - Start")

	var uniqueId string
	var ok bool
	if uniqueId, ok = params["unique_id"].(string); !ok || uniqueId == "" {
		err = logs.Err(ctx, errors.New("unique_id is required"), "")
		return nil, false, err
	}

	transactionInquiryParams := TransactionInquiryParams{
		CorpId:   iciciBankTool.CorpId,
		UserId:   iciciBankTool.UserId,
		AggrId:   iciciBankTool.AggrId,
		URN:      iciciBankTool.URN,
		UniqueId: uniqueId,
	}

	requestPayload, encryptedFingerprintB64, err := iciciBankTool.encryptRequestPayload(ctx, transactionInquiryParams)
	if err != nil {
		return nil, false, err
	}

	url := fmt.Sprintf("%s/v1/TransactionInquiry", iciciBankTool.BaseUrl)

	headers := http.Header{}
	headers.Set("Content-Type", "application/json; charset=utf-8")
	headers.Set("X-SDK-Originated", "true")
	headers.Set("X-SDK-Version", "1.0")
	headers.Set("X-ID-Fingerprint", encryptedFingerprintB64)

	if iciciBankTool.ApiKey != "" {
		headers.Set("apikey", iciciBankTool.ApiKey)
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, requestPayload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	decryptedResponseDataBytesUnpadded, err := iciciBankTool.decryptResponsePayload(ctx, res)
	if err != nil {
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	decryptedResponse := make(map[string]interface{})
	if err := json.Unmarshal(decryptedResponseDataBytesUnpadded, &decryptedResponse); err != nil {
		toolResult["transaction_inquiry"] = string(decryptedResponseDataBytesUnpadded)
	} else {
		toolResult["transaction_inquiry"] = decryptedResponse
	}
	return toolResult, true, nil
}

func (iciciBankTool *IciciBankTool) ExecuteNeftStatus(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("IciciBankTool ExecuteNeftStatus - Start")

	var utrNumber string
	var ok bool
	if utrNumber, ok = params["utr_number"].(string); !ok || utrNumber == "" {
		err = logs.Err(ctx, errors.New("utr_number is required"), "")
		return nil, false, err
	}

	neftStatusParams := NeftStatusParams{
		CorpId:    iciciBankTool.CorpId,
		UserId:    iciciBankTool.UserId,
		AggrId:    iciciBankTool.AggrId,
		URN:       iciciBankTool.URN,
		UtrNumber: utrNumber,
	}

	requestPayload, encryptedFingerprintB64, err := iciciBankTool.encryptRequestPayload(ctx, neftStatusParams)
	if err != nil {
		return nil, false, err
	}

	url := strings.Replace(iciciBankTool.BaseUrl, "/Corporate/CIB", "/v1/CIBNEFTStatus", -1)

	headers := http.Header{}
	headers.Set("Content-Type", "application/json; charset=utf-8")
	headers.Set("X-SDK-Originated", "true")
	headers.Set("X-SDK-Version", "1.0")
	headers.Set("X-ID-Fingerprint", encryptedFingerprintB64)

	if iciciBankTool.ApiKey != "" {
		headers.Set("apikey", iciciBankTool.ApiKey)
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, requestPayload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	decryptedResponseDataBytesUnpadded, err := iciciBankTool.decryptResponsePayload(ctx, res)
	if err != nil {
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	decryptedResponse := make(map[string]interface{})
	if err := json.Unmarshal(decryptedResponseDataBytesUnpadded, &decryptedResponse); err != nil {
		toolResult["neft_status"] = string(decryptedResponseDataBytesUnpadded)
	} else {
		toolResult["neft_status"] = decryptedResponse
	}
	return toolResult, true, nil
}

func (iciciBankTool *IciciBankTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	err := json.Unmarshal(toolObjJson, &iciciBankTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return iciciBankTool, nil
}

func (iciciBankTool *IciciBankTool) GetBytes(ctx context.Context) ([]byte, error) {
	toolJson, err := json.Marshal(iciciBankTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}
