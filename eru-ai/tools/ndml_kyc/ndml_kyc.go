package ndml_kyc

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
)

// NDML Tool Parameters
type NdmlInquiryParams struct {
	PanNo     string `json:"pan_no" eru:"required" desc:"PAN number for inquiry"`
	PanDob    string `json:"pan_dob" eru:"required" desc:"Date of birth in DDMMYYYY format"`
	MobileNo  string `json:"mobile_no" eru:"required" desc:"Mobile number"`
	RequestNo string `json:"request_no" eru:"required" desc:"Request number"`
}

type NdmlDownloadParams struct {
	PanNo     string `json:"pan_no" eru:"required" desc:"PAN number for document download"`
	PanDob    string `json:"pan_dob" eru:"required" desc:"Date of birth in DDMMYYYY format"`
	MobileNo  string `json:"mobile_no" eru:"required" desc:"Mobile number"`
	RequestNo string `json:"request_no" desc:"Request number (optional)"`
}

// SOAP Request/Response structures
type AppPanInq struct {
	PanNo     string `xml:"APP_PAN_NO"`
	PanDob    string `xml:"APP_PAN_DOB"`
	MobileNo  string `xml:"APP_MOBILE_NO"`
	RequestNo string `xml:"APP_REQ_NO"`
}

type AppPanDown struct {
	PanNo     string `xml:"APP_PAN_NO"`
	PanDob    string `xml:"APP_PAN_DOB"`
	MobileNo  string `xml:"APP_MOBILE_NO"`
	RequestNo string `xml:"APP_REQ_NO,omitempty"`
}

type InquiryRequest struct {
	XMLName xml.Name  `xml:"APP_REQ_ROOT"`
	PanInq  AppPanInq `xml:"APP_PAN_INQ"`
}

type DownloadRequest struct {
	XMLName xml.Name   `xml:"APP_REQ_ROOT"`
	PanDown AppPanDown `xml:"APP_PAN_DOWN"`
}

type AppPanInqResponse struct {
	HoldDeactiveRmks string `xml:"APP_HOLD_DEACTIVE_RMKS"`
	IpvFlag          string `xml:"APP_IPV_FLAG"`
	KycMode          string `xml:"APP_KYC_MODE"`
	Name             string `xml:"APP_NAME"`
	PanNo            string `xml:"APP_PAN_NO"`
	RequestNo        string `xml:"APP_REQ_NO"`
	ResponseNo       string `xml:"APP_RES_NO"`
	Status           string `xml:"APP_STATUS"`
	StatusDt         string `xml:"APP_STATUSDT"`
	UpdateRmks       string `xml:"APP_UPDT_RMKS"`
	UpdateStatus     string `xml:"APP_UPDT_STATUS"`
}

type InquiryResponse struct {
	XMLName xml.Name          `xml:"APP_RES_ROOT"`
	PanInq  AppPanInqResponse `xml:"APP_PAN_INQ"`
}

type NdmlTool struct {
	tools.Tool
	SoapEndpoint  string `json:"soap_endpoint"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	SkipTLSVerify bool   `json:"skip_tls_verify"`
	CaCert        string `json:"ca_cert" desc:"PEM encoded CA certificate to trust"`
}

const (
	InquiryAction          = "inquiry"
	DocumentDownloadAction = "document_download"
	GetPasscodeAction      = "get_passcode"
)

var ndmlToolActions = []tools.ToolAction{
	{
		ActionName:   InquiryAction,
		Description:  "Perform KYC inquiry for PAN details",
		SystemPrompt: "Perform KYC inquiry for PAN details using NDML web service",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(NdmlInquiryParams{}))
		},
	},
	{
		ActionName:   DocumentDownloadAction,
		Description:  "Download KYC documents for PAN",
		SystemPrompt: "Download KYC documents for PAN using NDML web service",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(NdmlDownloadParams{}))
		},
	},
	{
		ActionName:   GetPasscodeAction,
		Description:  "Get passcode from NDML service using password and passkey",
		SystemPrompt: "Get passcode from NDML service for authentication",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
	},
}

func (ndmlTool *NdmlTool) GetActionsList() []string {
	actions := []string{}
	for _, action := range ndmlToolActions {
		actions = append(actions, action.ActionName)
	}
	return actions
}

func (ndmlTool *NdmlTool) GetSpec() tools.Tooling {
	return ndmlTool
}

func (ndmlTool *NdmlTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &ndmlTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (ndmlTool *NdmlTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("NdmlTool Execute - Start")
	switch actionName {
	case InquiryAction:
		return ndmlTool.ExecuteInquiry(ctx, params)
	case DocumentDownloadAction:
		return ndmlTool.ExecuteDocumentDownload(ctx, params)
	case GetPasscodeAction:
		return ndmlTool.ExecuteGetPasscode(ctx, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
}

func (ndmlTool *NdmlTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	err := json.Unmarshal(toolObjJson, &ndmlTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return ndmlTool, nil
}

func (ndmlTool *NdmlTool) ExecuteInquiry(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("NdmlTool ExecuteInquiry - Start")

	passcode, passkey, err := ndmlTool.getPasscode(ctx)
	if err != nil {
		return nil, false, err
	}

	ndmlParams := NdmlInquiryParams{}
	ndmlParamsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, false, fmt.Errorf("error marshalling ndml params: %w", err)
	}

	err = json.Unmarshal(ndmlParamsBytes, &ndmlParams)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, false, err
	}

	// Create XML request structure
	soapRequest := InquiryRequest{
		PanInq: AppPanInq{
			PanNo:     ndmlParams.PanNo,
			PanDob:    ndmlParams.PanDob,
			MobileNo:  ndmlParams.MobileNo,
			RequestNo: ndmlParams.RequestNo,
		},
	}

	// Convert to XML string (without XML declaration)
	xmlData, err := xml.Marshal(soapRequest)
	if err != nil {
		return nil, false, fmt.Errorf("error marshalling XML: %w", err)
	}
	xmlString := string(xmlData)

	// XML escape the content for arg0 (escape < > and &)
	escapedXML := xmlEscape(xmlString)

	// Create SOAP envelope with tns:panInquiryDetails
	headers := http.Header{}
	headers.Add("Content-Type", "text/xml; charset=utf-8")
	headers.Add("SOAPAction", "\"\"")

	soapEnvelope := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:tns="http://service.webservice.pan.kra.ndml.com/">
  <soapenv:Header/>
  <soapenv:Body>
    <tns:panInquiryDetails>
      <arg0>%s</arg0>
      <arg1>%s</arg1>
      <arg2>%s</arg2>
      <arg3>%s</arg3>
    </tns:panInquiryDetails>
  </soapenv:Body>
</soapenv:Envelope>`, escapedXML, ndmlTool.Username, passcode, passkey)

	responseStr, err := ndmlTool.postSoap(ctx, ndmlTool.SoapEndpoint, headers, soapEnvelope)
	if err != nil {
		return nil, false, fmt.Errorf("error calling SOAP service: %w", err)
	}

	// Extract and parse XML response
	startTag := "<APP_RES_ROOT>"
	endTag := "</APP_RES_ROOT>"
	startIndex := strings.Index(responseStr, startTag)
	if startIndex == -1 {
		return nil, false, errors.New("invalid SOAP response: APP_RES_ROOT not found")
	}
	endIndex := strings.Index(responseStr, endTag)
	if endIndex == -1 {
		return nil, false, errors.New("invalid SOAP response: closing APP_RES_ROOT not found")
	}

	xmlResponse := responseStr[startIndex : endIndex+len(endTag)]

	// Parse XML response
	var inquiryResponse InquiryResponse
	err = xml.Unmarshal([]byte(xmlResponse), &inquiryResponse)
	if err != nil {
		return nil, false, fmt.Errorf("error unmarshalling XML response: %w", err)
	}

	// Convert to JSON response
	toolResult = make(map[string]interface{})
	toolResult["inquiry_result"] = map[string]interface{}{
		"hold_deactive_remarks": inquiryResponse.PanInq.HoldDeactiveRmks,
		"ipv_flag":              inquiryResponse.PanInq.IpvFlag,
		"kyc_mode":              inquiryResponse.PanInq.KycMode,
		"name":                  inquiryResponse.PanInq.Name,
		"pan_no":                inquiryResponse.PanInq.PanNo,
		"request_no":            inquiryResponse.PanInq.RequestNo,
		"response_no":           inquiryResponse.PanInq.ResponseNo,
		"status":                inquiryResponse.PanInq.Status,
		"status_date":           inquiryResponse.PanInq.StatusDt,
		"update_remarks":        inquiryResponse.PanInq.UpdateRmks,
		"update_status":         inquiryResponse.PanInq.UpdateStatus,
	}

	return toolResult, true, nil
}

func (ndmlTool *NdmlTool) ExecuteDocumentDownload(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("NdmlTool ExecuteDocumentDownload - Start")

	passcode, passkey, err := ndmlTool.getPasscodeWithPasskey(ctx)
	if err != nil {
		return nil, false, err
	}

	ndmlParams := NdmlDownloadParams{}
	ndmlParamsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, false, fmt.Errorf("error marshalling ndml params: %w", err)
	}

	err = json.Unmarshal(ndmlParamsBytes, &ndmlParams)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, false, err
	}

	// Create XML request structure
	soapRequest := DownloadRequest{
		PanDown: AppPanDown{
			PanNo:     ndmlParams.PanNo,
			PanDob:    ndmlParams.PanDob,
			MobileNo:  ndmlParams.MobileNo,
			RequestNo: ndmlParams.RequestNo,
		},
	}

	// Convert to XML string (without XML declaration)
	xmlData, err := xml.Marshal(soapRequest)
	if err != nil {
		return nil, false, fmt.Errorf("error marshalling XML: %w", err)
	}
	xmlString := string(xmlData)

	// XML escape the content for arg0
	escapedXML := xmlEscape(xmlString)

	// Create SOAP envelope with tns:panDownloadDetailsComplete
	headers := http.Header{}
	headers.Add("Content-Type", "text/xml; charset=utf-8")
	headers.Add("SOAPAction", "\"\"")

	soapEnvelope := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:tns="http://service.webservice.pan.kra.ndml.com/">
  <soapenv:Header/>
  <soapenv:Body>
    <tns:panDownloadDetailsComplete>
      <arg0>%s</arg0>
      <arg1>%s</arg1>
      <arg2>%s</arg2>
      <arg3>%s</arg3>
    </tns:panDownloadDetailsComplete>
  </soapenv:Body>
</soapenv:Envelope>`, escapedXML, ndmlTool.Username, passcode, passkey)

	responseStr, err := ndmlTool.postSoap(ctx, ndmlTool.SoapEndpoint, headers, soapEnvelope)
	if err != nil {
		return nil, false, fmt.Errorf("error calling SOAP service: %w", err)
	}

	// Extract and parse XML response
	startTag := "<APP_RES_ROOT>"
	endTag := "</APP_RES_ROOT>"
	startIndex := strings.Index(responseStr, startTag)
	if startIndex == -1 {
		return nil, false, errors.New("invalid SOAP response: APP_RES_ROOT not found")
	}
	endIndex := strings.Index(responseStr, endTag)
	if endIndex == -1 {
		return nil, false, errors.New("invalid SOAP response: closing APP_RES_ROOT not found")
	}

	xmlResponse := responseStr[startIndex : endIndex+len(endTag)]

	// Parse XML response
	var downloadResponse InquiryResponse
	err = xml.Unmarshal([]byte(xmlResponse), &downloadResponse)
	if err != nil {
		return nil, false, fmt.Errorf("error unmarshalling XML response: %w", err)
	}

	// Convert to JSON response
	toolResult = make(map[string]interface{})
	toolResult["download_result"] = map[string]interface{}{
		"hold_deactive_remarks": downloadResponse.PanInq.HoldDeactiveRmks,
		"ipv_flag":              downloadResponse.PanInq.IpvFlag,
		"kyc_mode":              downloadResponse.PanInq.KycMode,
		"name":                  downloadResponse.PanInq.Name,
		"pan_no":                downloadResponse.PanInq.PanNo,
		"request_no":            downloadResponse.PanInq.RequestNo,
		"response_no":           downloadResponse.PanInq.ResponseNo,
		"status":                downloadResponse.PanInq.Status,
		"status_date":           downloadResponse.PanInq.StatusDt,
		"update_remarks":        downloadResponse.PanInq.UpdateRmks,
		"update_status":         downloadResponse.PanInq.UpdateStatus,
	}

	return toolResult, true, nil
}

func (ndmlTool *NdmlTool) callSoapService(ctx context.Context, xmlData []byte, passcode string) ([]byte, error) {
	if ndmlTool.SoapEndpoint == "" {
		return nil, errors.New("SOAP endpoint not configured")
	}

	headers := http.Header{}
	headers.Add("Content-Type", "text/xml; charset=utf-8")
	headers.Add("SOAPAction", "\"\"")

	// Add basic auth if credentials are provided
	if ndmlTool.Username != "" && ndmlTool.Password != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(ndmlTool.Username + ":" + ndmlTool.Password))
		headers.Add("Authorization", "Basic "+auth)
	}

	// Create SOAP envelope with optional security header
	soapHeader := ""
	if passcode != "" {
		soapHeader = fmt.Sprintf("<APP_HEADER><APP_PASSCODE>%s</APP_PASSCODE></APP_HEADER>", passcode)
	}
	soapEnvelope := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
    <soap:Header>%s</soap:Header>
    <soap:Body>
        %s
    </soap:Body>
</soap:Envelope>`, soapHeader, string(xmlData))

	responseStr, err := ndmlTool.postSoap(ctx, ndmlTool.SoapEndpoint, headers, soapEnvelope)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	startTag := "<APP_RES_ROOT>"
	endTag := "</APP_RES_ROOT>"

	startIndex := strings.Index(responseStr, startTag)
	if startIndex == -1 {
		return nil, errors.New("invalid SOAP response: APP_RES_ROOT not found")
	}

	endIndex := strings.Index(responseStr, endTag)
	if endIndex == -1 {
		return nil, errors.New("invalid SOAP response: closing APP_RES_ROOT not found")
	}

	xmlResponse := responseStr[startIndex : endIndex+len(endTag)]
	return []byte(xmlResponse), nil
}

// ExecuteGetPasscode executes the get_passcode action
func (ndmlTool *NdmlTool) ExecuteGetPasscode(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("NdmlTool ExecuteGetPasscode - Start")

	var password, passkey string
	if paramsObj, ok := params["params"]; ok {
		if paramsMap, ok := paramsObj.(map[string]interface{}); ok {
			if pwd, ok := paramsMap["password"].(string); ok {
				password = pwd
			}
			if pk, ok := paramsMap["passkey"].(string); ok {
				passkey = pk
			}
		}
	}

	if password == "" {
		password = ndmlTool.Password
	}
	if passkey == "" {
		var genErr error
		passkey, genErr = generateRandomBase64(16)
		if genErr != nil {
			return nil, false, fmt.Errorf("error generating passkey: %w", genErr)
		}
	}

	passcode, err := ndmlTool.getPasscodeWithParams(ctx, password, passkey)
	if err != nil {
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["passcode"] = passcode
	toolResult["passkey"] = passkey
	return toolResult, false, nil
}

// getPasscode requests a dynamic passcode from NDML service using Password and generated PassKey
func (ndmlTool *NdmlTool) getPasscode(ctx context.Context) (string, string, error) {
	passcode, passkey, err := ndmlTool.getPasscodeWithPasskey(ctx)
	return passcode, passkey, err
}

// getPasscodeWithPasskey returns both passcode and passkey
func (ndmlTool *NdmlTool) getPasscodeWithPasskey(ctx context.Context) (string, string, error) {
	if ndmlTool.Password == "" {
		return "", "", errors.New("password not configured")
	}
	if ndmlTool.SoapEndpoint == "" {
		return "", "", errors.New("SOAP endpoint not configured")
	}

	passKey, err := generateRandomBase64(16)
	if err != nil {
		return "", "", err
	}
	passcode, err := ndmlTool.getPasscodeWithParams(ctx, ndmlTool.Password, passKey)
	if err != nil {
		return "", "", err
	}
	return passcode, passKey, nil
}

// getPasscodeWithParams requests passcode using provided password and passkey
func (ndmlTool *NdmlTool) getPasscodeWithParams(ctx context.Context, password, passkey string) (string, error) {
	if ndmlTool.SoapEndpoint == "" {
		return "", errors.New("SOAP endpoint not configured")
	}

	headers := http.Header{}
	headers.Add("Content-Type", "text/xml; charset=utf-8")
	headers.Add("SOAPAction", "\"\"")

	envelope := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:tns="http://service.webservice.pan.kra.ndml.com/">
  <soapenv:Header/>
  <soapenv:Body>
    <tns:getPasscode>
      <arg0>%s</arg0>
      <arg1>%s</arg1>
    </tns:getPasscode>
  </soapenv:Body>
</soapenv:Envelope>`, password, passkey)

	responseStr, err := ndmlTool.postSoap(ctx, ndmlTool.SoapEndpoint, headers, envelope)
	if err != nil {
		return "", err
	}

	startTag := "<return>"
	endTag := "</return>"
	s := strings.Index(responseStr, startTag)
	if s == -1 {
		startTag = "<APP_PASSCODE>"
		endTag = "</APP_PASSCODE>"
		s = strings.Index(responseStr, startTag)
		if s == -1 {
			return "", errors.New("passcode not found in response")
		}
	}
	e := strings.Index(responseStr, endTag)
	if e == -1 || e <= s {
		return "", errors.New("passcode closing tag not found")
	}
	return responseStr[s+len(startTag) : e], nil
}

// generateRandomBase64 returns a base64 string of n random bytes
func generateRandomBase64(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// xmlEscape escapes XML special characters (must escape & first)
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// postSoap sends the SOAP request with optional custom TLS settings and returns the response body as string
func (ndmlTool *NdmlTool) postSoap(ctx context.Context, url string, headers http.Header, body string) (string, error) {
	tlsCfg := &tls.Config{InsecureSkipVerify: ndmlTool.SkipTLSVerify} //nolint:gosec
	if ndmlTool.CaCert != "" {
		pool := x509.NewCertPool()
		if ok := pool.AppendCertsFromPEM([]byte(ndmlTool.CaCert)); !ok {
			return "", errors.New("failed to parse provided CA cert")
		}
		tlsCfg.RootCAs = pool
	}
	transport := &http.Transport{TLSClientConfig: tlsCfg}
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		return "", err
	}
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(respBytes), nil
}

func (ndmlTool *NdmlTool) GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "tool_name":
		return ndmlTool.ToolName, nil
	case "tool_type":
		return ndmlTool.ToolType, nil
	case "system_prompt":
		return ndmlTool.SystemPrompt, nil
	case "output_schema":
		return ndmlTool.OutputSchema, nil
	case "parameters":
		return ndmlTool.Parameters, nil
	case "description":
		return ndmlTool.Description, nil
	case "soap_endpoint":
		return ndmlTool.SoapEndpoint, nil
	case "username":
		return ndmlTool.Username, nil
	case "password":
		return ndmlTool.Password, nil
	default:
		err := errors.New(fmt.Sprintf("attribute not found: %s", attributeName))
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func (ndmlTool *NdmlTool) SetAttribute(ctx context.Context, attributeName string, attributeValue interface{}) (err error) {
	switch attributeName {
	case "tool_name":
		ndmlTool.ToolName = attributeValue.(string)
	case "tool_type":
		ndmlTool.ToolType = attributeValue.(string)
	case "system_prompt":
		ndmlTool.SystemPrompt = attributeValue.(string)
	case "output_schema":
		ndmlTool.OutputSchema = attributeValue.(eru_models.JSONSchema)
	case "parameters":
		ndmlTool.Parameters = attributeValue.(eru_models.JSONSchema)
	case "description":
		ndmlTool.Description = attributeValue.(string)
	case "soap_endpoint":
		ndmlTool.SoapEndpoint = attributeValue.(string)
	case "username":
		ndmlTool.Username = attributeValue.(string)
	case "password":
		ndmlTool.Password = attributeValue.(string)
	default:
		err = errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (ndmlTool *NdmlTool) GetBytes(ctx context.Context) ([]byte, error) {
	toolJson, err := json.Marshal(ndmlTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}

func (ndmlTool *NdmlTool) SetToolAction(actionName string) {
	for _, action := range ndmlToolActions {
		if action.ActionName == actionName {
			ndmlTool.ToolAction = action
			return
		}
	}
	ndmlTool.ToolAction = tools.ToolAction{}
}
