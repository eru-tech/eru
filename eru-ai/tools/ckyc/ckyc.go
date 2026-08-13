package ckyc

import (
	"bytes"
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"errors"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	aes "github.com/eru-tech/eru/eru-crypto/aes"
	rsa "github.com/eru-tech/eru/eru-crypto/rsa"
	sha "github.com/eru-tech/eru/eru-crypto/sha"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	server "github.com/eru-tech/eru/eru-server/server"
	utils "github.com/eru-tech/eru/eru-utils"
)

const CKYC_VERSION = "1.3"

var ckycIST = time.FixedZone("IST", 5*3600+1800)

func ckycTimestamp() string {
	return time.Now().In(ckycIST).Format("02-01-2006 15:04:05")
}

type CkycVerifyParams struct {
	IdNo   string `json:"id_no" eru:"required" desc:"ID number for search"`
	IdType string `json:"id_type" eru:"required" desc:"ID type (A-Aadhaar, B-PAN, C-Voter ID, D-Passport, E-Driving License, G-MGNREGA Job Card)"`
}

type CkycDownloadParams struct {
	CkycNo         string `json:"ckyc_no" eru:"required" desc:"14-digit CKYC Number or 14-character CKYC Reference ID"`
	AuthFactorType string `json:"auth_factor_type" eru:"required" desc:"Authentication factor type: 01-DOI (Legal), 03-Mobile (Individual/Legal), 04-Email (Legal), 05-Pincode (Legal)"`
	AuthFactor     string `json:"auth_factor" eru:"required" desc:"Authentication factor value (10-digit mobile / dd-mm-yyyy DOI / email / pincode)"`
}

type CkycValidateOtpParams struct {
	RequestId string `json:"request_id" eru:"required" desc:"Request ID returned by the download action (must match)"`
	Otp       string `json:"otp" desc:"6-digit OTP received on registered mobile (leave empty to resend OTP)"`
	Validate  string `json:"validate" desc:"Validate flag (defaults to Y)"`
}

type CkycTool struct {
	tools.Tool
	FiCode        string `json:"fi_code"`
	CkycPublicKey string `json:"ckyc_public_key"`
	FiPrivateKey  string `json:"fi_private_key"`
	FiPublicKey   string `json:"fi_public_key"`
	BaseUrl       string `json:"base_url"`
}

const (
	VERIFY       = "verify"
	DOWNLOAD     = "download"
	VALIDATE_OTP = "validate_otp"
)

var ckycToolActions = []tools.ToolAction{
	{
		ActionName:   VERIFY,
		Description:  "Search CKYC details using ID number and type",
		SystemPrompt: "Search CKYC details using ID number and type",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(CkycVerifyParams{}), []string{})
		},
	},
	{
		ActionName:   DOWNLOAD,
		Description:  "Initiate CKYC record download by CKYC number/reference ID and auth factor; triggers OTP to registered mobile for Individual records",
		SystemPrompt: "Initiate CKYC record download by CKYC number/reference ID and auth factor; triggers OTP to registered mobile for Individual records",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(CkycDownloadParams{}), []string{})
		},
	},
	{
		ActionName:   VALIDATE_OTP,
		Description:  "Validate OTP (or trigger resend) for a previously initiated CKYC download; on success returns the full CKYC record",
		SystemPrompt: "Validate OTP (or trigger resend) for a previously initiated CKYC download; on success returns the full CKYC record",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(CkycValidateOtpParams{}), []string{})
		},
	},
}

func (c *CkycTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(ckycToolActions))
	for i, action := range ckycToolActions {
		infos[i] = tools.ActionInfo{Name: action.ActionName, Description: action.Description}
	}
	return infos
}

func (c *CkycTool) GetActions() []tools.ToolAction {
	return ckycToolActions
}

func (c *CkycTool) GetSpec() tools.Tooling {
	return c
}

func (c *CkycTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("CkycTool MakeFromJson - Start")
	err := json.Unmarshal(*rj, &c)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (c *CkycTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CkycTool Execute - Start")
	var toolRequest interface{}
	switch actionName {
	case VERIFY:
		toolResult, toolRequest, persistStore, err = c.ExecuteVerify(ctx, params)
	case DOWNLOAD:
		toolResult, toolRequest, persistStore, err = c.ExecuteDownload(ctx, params)
	case VALIDATE_OTP:
		toolResult, toolRequest, persistStore, err = c.ExecuteValidateOtp(ctx, params)
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

		hookResult, err := c.ExecuteHook(bgCtx, "poex", actionName, projectId, tenantId, body, nil)
		if err != nil {
			logs.WithContext(bgCtx).Error(err.Error())
			return
		}
		logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
	}, server.ContinueOnMaxRetries)

	return toolResult, persistStore, err
}

func (c *CkycTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	logs.WithContext(ctx).Debug("CkycTool BytesToTool - Start")
	ckycTool := CkycTool{}
	err := json.Unmarshal(toolObjJson, &ckycTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return &ckycTool, nil
}

func (c *CkycTool) ExecuteVerify(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CkycTool ExecuteVerify - Start")

	logs.WithContext(ctx).Info(fmt.Sprintf("c.FiPrivateKey: %v", c.FiPrivateKey))
	logs.WithContext(ctx).Info(fmt.Sprintf("c.FiPublicKey: %v", c.FiPublicKey))
	logs.WithContext(ctx).Info(fmt.Sprintf("c.CkycPublicKey: %v", c.CkycPublicKey))
	logs.WithContext(ctx).Info(fmt.Sprintf("c.BaseUrl: %v", c.BaseUrl))
	logs.WithContext(ctx).Info(fmt.Sprintf("c.FiCode: %v", c.FiCode))

	verifyParams := CkycVerifyParams{}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling params: %w", err)
	}
	err = json.Unmarshal(paramsBytes, &verifyParams)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error unmarshalling params: %w", err)
	}

	// 1 & 2. Generate PID_DATA XML with Timestamp
	timestamp := ckycTimestamp()
	pidData := PidData{
		DateTime: timestamp,
		IdNo:     verifyParams.IdNo,
		IdType:   verifyParams.IdType,
	}
	pidXml, err := xml.Marshal(pidData)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling PID_DATA: %w", err)
	}
	pidXmlStr := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + string(pidXml)

	// 3. Generate random 256-bit session key
	sessionKey, err := aes.GenerateKey(ctx, 32)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error generating session key: %w", err)
	}

	logs.WithContext(ctx).Info(fmt.Sprintf("CKYC PID XML: %s", pidXmlStr))
	// 4 & 5. Encrypt PID_DATA using session key (AES-256-ECB PKCS7)
	encryptedPid, err := aes.EncryptECB(ctx, []byte(pidXmlStr), sessionKey.Key)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error encrypting PID_DATA: %w", err)
	}
	encodedPid := base64.StdEncoding.EncodeToString(encryptedPid)

	// 6 & 7. Encrypt session key using CKYC public key (RSA OAEP SHA1)
	ckycPublicKeyStr := decodePemBundle(c.CkycPublicKey)
	encryptedSessionKey, err := rsa.EncryptOAEP(ctx, sessionKey.Key, ckycPublicKeyStr, nil)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error encrypting session key: %w", err)
	}
	encodedSessionKey := base64.StdEncoding.EncodeToString(encryptedSessionKey)

	// 8. Wrap everything in REQ_ROOT.
	// CKYC server constraint: REQUEST_ID must be <= 8 digits.
	requestId := fmt.Sprintf("%08d", time.Now().UnixNano()%100_000_000)
	reqRoot := ReqRoot{
		Header: Header{
			FiCode:    c.FiCode,
			RequestId: requestId,
			Version:   CKYC_VERSION,
		},
		CkycInq: CkycInq{
			Pid:        encodedPid,
			SessionKey: encodedSessionKey,
		},
	}

	rawXml, err := xml.Marshal(reqRoot)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling REQ_ROOT: %w", err)
	}
	// 9. Sign the request (including the XML declaration)
	baseXml := `<?xml version="1.0" encoding="UTF-8" standalone="no"?>` + string(rawXml)
	logs.WithContext(ctx).Info(fmt.Sprintf("CKYC Base XML: %s", baseXml))
	finalSignedXml, err := c.SignXml(ctx, baseXml, "REQ_ROOT")
	if err != nil {
		return nil, nil, false, fmt.Errorf("error signing XML: %w", err)
	}

	// 10. Call the CKYC API
	if c.BaseUrl == "" {
		return nil, nil, false, fmt.Errorf("base url is not configured")
	}

	headers := http.Header{}
	headers.Add("Content-Type", "application/xml")
	// Some APIs might require IP address in a header or just rely on source IP
	url := fmt.Sprintf("%s%s", strings.TrimSuffix(c.BaseUrl, "/"), "/Search/ckycverificationservice/verify")

	// We use ExecuteHttp directly to avoid utils.CallHttp's automatic JSON-marshalling of the body
	logs.WithContext(ctx).Info(fmt.Sprintf("CKYC Final Request: %s", finalSignedXml))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(finalSignedXml))
	if err != nil {
		return nil, nil, false, fmt.Errorf("error creating request: %w", err)
	}
	req.Header = headers

	resp, err := utils.ExecuteHttp(ctx, req)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error calling CKYC API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error reading response body: %w", err)
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("CKYC Final Response: %s", string(respBody)))
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var res map[string]interface{}
		if err := json.Unmarshal(respBody, &res); err != nil {
			return map[string]interface{}{"response": string(respBody)}, map[string]interface{}{"body": verifyParams}, false, nil
		}
		return res, map[string]interface{}{"body": verifyParams}, false, nil
	}

	respStr := string(respBody)
	result := map[string]interface{}{}
	result["header"] = parseWireHeader(respStr)

	errorMsg := extractTagValue(respStr, "ERROR")
	if errorMsg != "" {
		result["error"] = errorMsg
		return result, map[string]interface{}{"body": verifyParams}, false, nil
	}

	encPid := extractTagValue(respStr, "PID")
	encSk := extractTagValue(respStr, "SESSION_KEY")
	if encPid == "" || encSk == "" {
		return result, map[string]interface{}{"body": verifyParams}, false, nil
	}

	decryptedPid, decErr := c.DecryptPidData(ctx, encPid, encSk)
	if decErr != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("error decrypting response PID: %v", decErr))
		result["decrypt_error"] = decErr.Error()
		return result, map[string]interface{}{"body": verifyParams}, false, nil
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("CKYC Decrypted PID: %s", decryptedPid))

	records, parseErr := parseDecryptedPid(decryptedPid)
	if parseErr != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("error parsing decrypted PID: %v", parseErr))
		result["parse_error"] = parseErr.Error()
		return result, map[string]interface{}{"body": verifyParams}, false, nil
	}
	result["records"] = records
	return result, map[string]interface{}{"body": verifyParams}, false, nil
}

func (c *CkycTool) buildSignedDownloadRequest(ctx context.Context, pidXmlStr string, requestId string) (string, error) {
	sessionKey, err := aes.GenerateKey(ctx, 32)
	if err != nil {
		return "", fmt.Errorf("error generating session key: %w", err)
	}

	logs.WithContext(ctx).Info(fmt.Sprintf("CKYC PID XML: %s", pidXmlStr))
	encryptedPid, err := aes.EncryptECB(ctx, []byte(pidXmlStr), sessionKey.Key)
	if err != nil {
		return "", fmt.Errorf("error encrypting PID_DATA: %w", err)
	}
	encodedPid := base64.StdEncoding.EncodeToString(encryptedPid)

	ckycPublicKeyStr := decodePemBundle(c.CkycPublicKey)
	encryptedSessionKey, err := rsa.EncryptOAEP(ctx, sessionKey.Key, ckycPublicKeyStr, nil)
	if err != nil {
		return "", fmt.Errorf("error encrypting session key: %w", err)
	}
	encodedSessionKey := base64.StdEncoding.EncodeToString(encryptedSessionKey)

	reqRoot := DownloadRequestRoot{
		Header: Header{
			FiCode:    c.FiCode,
			RequestId: requestId,
			Version:   CKYC_VERSION,
		},
		CkycInq: CkycInq{
			Pid:        encodedPid,
			SessionKey: encodedSessionKey,
		},
	}

	rawXml, err := xml.Marshal(reqRoot)
	if err != nil {
		return "", fmt.Errorf("error marshalling CKYC_DOWNLOAD_REQUEST: %w", err)
	}
	baseXml := `<?xml version="1.0" encoding="UTF-8" standalone="no"?>` + string(rawXml)
	logs.WithContext(ctx).Info(fmt.Sprintf("CKYC Base XML: %s", baseXml))
	finalSignedXml, err := c.SignXml(ctx, baseXml, "CKYC_DOWNLOAD_REQUEST")
	if err != nil {
		return "", fmt.Errorf("error signing XML: %w", err)
	}
	return finalSignedXml, nil
}

func (c *CkycTool) postSignedXml(ctx context.Context, url string, signedXml string) ([]byte, error) {
	if c.BaseUrl == "" {
		return nil, fmt.Errorf("base url is not configured")
	}
	headers := http.Header{}
	headers.Add("Content-Type", "application/xml")

	logs.WithContext(ctx).Info(fmt.Sprintf("CKYC Final Request: %s", signedXml))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(signedXml))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header = headers

	resp, err := utils.ExecuteHttp(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error calling CKYC API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("CKYC Final Response: %s", string(respBody)))
	return respBody, nil
}

func (c *CkycTool) ExecuteDownload(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CkycTool ExecuteDownload - Start")

	downloadParams := CkycDownloadParams{}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling params: %w", err)
	}
	if err := json.Unmarshal(paramsBytes, &downloadParams); err != nil {
		return nil, nil, false, fmt.Errorf("error unmarshalling params: %w", err)
	}

	timestamp := ckycTimestamp()
	pidData := DownloadPidData{
		DateTime:       timestamp,
		CkycNo:         downloadParams.CkycNo,
		AuthFactorType: downloadParams.AuthFactorType,
		AuthFactor:     downloadParams.AuthFactor,
	}
	pidXml, err := xml.Marshal(pidData)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling PID_DATA: %w", err)
	}
	pidXmlStr := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + string(pidXml)

	requestId := fmt.Sprintf("%08d", time.Now().UnixNano()%100_000_000)
	finalSignedXml, err := c.buildSignedDownloadRequest(ctx, pidXmlStr, requestId)
	if err != nil {
		return nil, nil, false, err
	}

	url := fmt.Sprintf("%s%s", strings.TrimSuffix(c.BaseUrl, "/"), "/Search/ckycverificationservice/download")
	respBody, err := c.postSignedXml(ctx, url, finalSignedXml)
	if err != nil {
		return nil, nil, false, err
	}

	respStr := string(respBody)
	result := map[string]interface{}{}
	result["header"] = parseWireHeader(respStr)
	result["request_id"] = requestId

	encPid := extractTagValue(respStr, "PID")
	encSk := extractTagValue(respStr, "SESSION_KEY")
	if encPid != "" && encSk != "" {
		decryptedPid, decErr := c.DecryptPidData(ctx, encPid, encSk)
		if decErr != nil {
			logs.WithContext(ctx).Error(fmt.Sprintf("error decrypting response PID: %v", decErr))
			result["decrypt_error"] = decErr.Error()
		} else {
			logs.WithContext(ctx).Info(fmt.Sprintf("CKYC Decrypted PID: %s", decryptedPid))
			record, parseErr := parseDownloadPid(decryptedPid)
			if parseErr != nil {
				logs.WithContext(ctx).Error(fmt.Sprintf("error parsing decrypted PID: %v", parseErr))
				result["parse_error"] = parseErr.Error()
			} else {
				result["record"] = record
			}
		}
	}

	if msg := extractTagValue(respStr, "ERROR"); msg != "" {
		result["message"] = msg
	}
	return result, map[string]interface{}{"body": downloadParams}, false, nil
}

func (c *CkycTool) ExecuteValidateOtp(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CkycTool ExecuteValidateOtp - Start")

	otpParams := CkycValidateOtpParams{}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling params: %w", err)
	}
	if err := json.Unmarshal(paramsBytes, &otpParams); err != nil {
		return nil, nil, false, fmt.Errorf("error unmarshalling params: %w", err)
	}
	if otpParams.RequestId == "" {
		return nil, nil, false, fmt.Errorf("request_id is required (use the value returned by the download action)")
	}

	timestamp := ckycTimestamp()
	validate := otpParams.Validate
	if validate == "" {
		validate = "Y"
	}
	pidData := ValidateOtpPidData{
		DateTime: timestamp,
		Otp:      otpParams.Otp,
		Validate: validate,
	}
	pidXml, err := xml.Marshal(pidData)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling PID_DATA: %w", err)
	}
	pidXmlStr := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + string(pidXml)

	finalSignedXml, err := c.buildSignedDownloadRequest(ctx, pidXmlStr, otpParams.RequestId)
	if err != nil {
		return nil, nil, false, err
	}

	url := fmt.Sprintf("%s%s", strings.TrimSuffix(c.BaseUrl, "/"), "/Search/ckycverificationservice/ValidateOTP")
	respBody, err := c.postSignedXml(ctx, url, finalSignedXml)
	if err != nil {
		return nil, nil, false, err
	}

	respStr := string(respBody)
	result := map[string]interface{}{}
	result["header"] = parseWireHeader(respStr)

	encPid := extractTagValue(respStr, "PID")
	encSk := extractTagValue(respStr, "SESSION_KEY")
	if encPid != "" && encSk != "" {
		decryptedPid, decErr := c.DecryptPidData(ctx, encPid, encSk)
		if decErr != nil {
			logs.WithContext(ctx).Error(fmt.Sprintf("error decrypting response PID: %v", decErr))
			result["decrypt_error"] = decErr.Error()
		} else {
			logs.WithContext(ctx).Info(fmt.Sprintf("CKYC Decrypted PID: %s", decryptedPid))
			record, parseErr := parseDownloadPid(decryptedPid)
			if parseErr != nil {
				logs.WithContext(ctx).Error(fmt.Sprintf("error parsing decrypted PID: %v", parseErr))
				result["parse_error"] = parseErr.Error()
			} else {
				result["record"] = record
			}
		}
	}

	if msg := extractTagValue(respStr, "ERROR"); msg != "" {
		result["message"] = msg
	}
	return result, map[string]interface{}{"body": otpParams}, false, nil
}

type CkycRecord struct {
	CkycNo               string    `xml:"CKYC_NO" json:"ckyc_no,omitempty"`
	CkycReferenceId      string    `xml:"CKYC_REFERENCE_ID" json:"ckyc_reference_id,omitempty"`
	Name                 string    `xml:"NAME" json:"name,omitempty"`
	FathersName          string    `xml:"FATHERS_NAME" json:"fathers_name,omitempty"`
	Age                  string    `xml:"AGE" json:"age,omitempty"`
	MobCode              string    `xml:"MOB_CODE" json:"mob_code,omitempty"`
	MobNum               string    `xml:"MOB_NUM" json:"mob_num,omitempty"`
	ImageType            string    `xml:"IMAGE_TYPE" json:"image_type,omitempty"`
	Photo                string    `xml:"PHOTO" json:"photo,omitempty"`
	KycDate              string    `xml:"KYC_DATE" json:"kyc_date,omitempty"`
	UpdatedDate          string    `xml:"UPDATED_DATE" json:"updated_date,omitempty"`
	ConstitutionType     string    `xml:"CONSTITUTION_TYPE" json:"constitution_type,omitempty"`
	PlaceOfIncorporation string    `xml:"PLACE_OF_INCORPORATION" json:"place_of_incorporation,omitempty"`
	IdList               idListXml `xml:"ID_LIST" json:"-"`
	Ids                  []IdEntry `xml:"-" json:"ids,omitempty"`
	Remarks              string    `xml:"REMARKS" json:"remarks,omitempty"`
}

type idListXml struct {
	Ids []IdEntry `xml:"ID"`
}

type IdEntry struct {
	Type   string `xml:"TYPE" json:"type,omitempty"`
	Status string `xml:"STATUS" json:"status,omitempty"`
}

type decryptedPidXml struct {
	XMLName            xml.Name     `xml:"PID_DATA"`
	CkycRecord                      // flat form (Individual ID_TYPE=Z, Legal)
	SearchResponsePIDs []CkycRecord `xml:"SearchResponsePID"`
}

type wireHeaderXml struct {
	XMLName   xml.Name `xml:"HEADER"`
	FiCode    string   `xml:"FI_CODE"`
	ReqDate   string   `xml:"REQ_DATE"`
	RequestId string   `xml:"REQUEST_ID"`
	Version   string   `xml:"VERSION"`
}

func parseWireHeader(wireXml string) map[string]string {
	headerStart := strings.Index(wireXml, "<HEADER>")
	headerEnd := strings.Index(wireXml, "</HEADER>")
	if headerStart == -1 || headerEnd == -1 {
		return nil
	}
	headerStr := wireXml[headerStart : headerEnd+len("</HEADER>")]
	var h wireHeaderXml
	if err := xml.Unmarshal([]byte(headerStr), &h); err != nil {
		return nil
	}
	return map[string]string{
		"fi_code":    h.FiCode,
		"req_date":   h.ReqDate,
		"request_id": h.RequestId,
		"version":    h.Version,
	}
}

func parseDecryptedPid(decryptedPid string) ([]CkycRecord, error) {
	var pid decryptedPidXml
	if err := xml.Unmarshal([]byte(decryptedPid), &pid); err != nil {
		return nil, err
	}
	records := pid.SearchResponsePIDs
	if len(records) == 0 && (pid.CkycNo != "" || pid.Name != "" || pid.CkycReferenceId != "") {
		records = []CkycRecord{pid.CkycRecord}
	}
	for i := range records {
		records[i].Ids = records[i].IdList.Ids
	}
	return records, nil
}

type DownloadRecord struct {
	XMLName              xml.Name              `xml:"PID_DATA" json:"-"`
	RecordCountDetails   *RecordCountDetails   `xml:"RECORD_COUNT_DETAILS" json:"record_count_details,omitempty"`
	PersonalDetails      *PersonalDetails      `xml:"PERSONAL_DETAILS" json:"personal_details,omitempty"`
	IdentityDetails      *IdentityDetails      `xml:"IDENTITY_DETAILS" json:"identity_details,omitempty"`
	RelatedPersonDetails *RelatedPersonDetails `xml:"RELATED_PERSON_DETAILS" json:"related_person_details,omitempty"`
	ImageDetails         *ImageDetails         `xml:"IMAGE_DETAILS" json:"image_details,omitempty"`
}

type RecordCountDetails struct {
	DownloadCount string `xml:"DOWNLOAD_COUNT" json:"download_count,omitempty"`
	UpdateCount   string `xml:"UPDATE_COUNT" json:"update_count,omitempty"`
}

type PersonalDetails struct {
	ConstiType         string `xml:"CONSTI_TYPE" json:"consti_type,omitempty"`
	ConstiTypeOthers   string `xml:"CONSTI_TYPE_OTHERS" json:"consti_type_others,omitempty"`
	AccType            string `xml:"ACC_TYPE" json:"acc_type,omitempty"`
	CkycNo             string `xml:"CKYC_NO" json:"ckyc_no,omitempty"`
	CkycReferenceId    string `xml:"CKYC_REFERENCE_ID" json:"ckyc_reference_id,omitempty"`
	Prefix             string `xml:"PREFIX" json:"prefix,omitempty"`
	Fname              string `xml:"FNAME" json:"fname,omitempty"`
	Mname              string `xml:"MNAME" json:"mname,omitempty"`
	Lname              string `xml:"LNAME" json:"lname,omitempty"`
	Fullname           string `xml:"FULLNAME" json:"fullname,omitempty"`
	MaidenPrefix       string `xml:"MAIDEN_PREFIX" json:"maiden_prefix,omitempty"`
	MaidenFname        string `xml:"MAIDEN_FNAME" json:"maiden_fname,omitempty"`
	MaidenMname        string `xml:"MAIDEN_MNAME" json:"maiden_mname,omitempty"`
	MaidenLname        string `xml:"MAIDEN_LNAME" json:"maiden_lname,omitempty"`
	MaidenFullname     string `xml:"MAIDEN_FULLNAME" json:"maiden_fullname,omitempty"`
	FatherSpouseFlag   string `xml:"FATHERSPOUSE_FLAG" json:"father_spouse_flag,omitempty"`
	FatherPrefix       string `xml:"FATHER_PREFIX" json:"father_prefix,omitempty"`
	FatherFname        string `xml:"FATHER_FNAME" json:"father_fname,omitempty"`
	FatherMname        string `xml:"FATHER_MNAME" json:"father_mname,omitempty"`
	FatherLname        string `xml:"FATHER_LNAME" json:"father_lname,omitempty"`
	FatherFullname     string `xml:"FATHER_FULLNAME" json:"father_fullname,omitempty"`
	MotherPrefix       string `xml:"MOTHER_PREFIX" json:"mother_prefix,omitempty"`
	MotherFname        string `xml:"MOTHER_FNAME" json:"mother_fname,omitempty"`
	MotherMname        string `xml:"MOTHER_MNAME" json:"mother_mname,omitempty"`
	MotherLname        string `xml:"MOTHER_LNAME" json:"mother_lname,omitempty"`
	MotherFullname     string `xml:"MOTHER_FULLNAME" json:"mother_fullname,omitempty"`
	Gender             string `xml:"GENDER" json:"gender,omitempty"`
	Dob                string `xml:"DOB" json:"dob,omitempty"`
	Pan                string `xml:"PAN" json:"pan,omitempty"`
	FormSixty          string `xml:"FORM_SIXTY" json:"form_sixty,omitempty"`
	DisFlag            string `xml:"DIS_FLAG" json:"dis_flag,omitempty"`
	DisType            string `xml:"DIS_TYPE" json:"dis_type,omitempty"`
	DisPercent         string `xml:"DIS_PERCENT" json:"dis_percent,omitempty"`
	DisUdidNumber      string `xml:"DIS_UDID_NUMBER" json:"dis_udid_number,omitempty"`
	ResiStatus         string `xml:"RESI_STATUS" json:"resi_status,omitempty"`
	PermLine1          string `xml:"PERM_LINE1" json:"perm_line1,omitempty"`
	PermLine2          string `xml:"PERM_LINE2" json:"perm_line2,omitempty"`
	PermLine3          string `xml:"PERM_LINE3" json:"perm_line3,omitempty"`
	PermCity           string `xml:"PERM_CITY" json:"perm_city,omitempty"`
	PermDist           string `xml:"PERM_DIST" json:"perm_dist,omitempty"`
	PermState          string `xml:"PERM_STATE" json:"perm_state,omitempty"`
	PermCountry        string `xml:"PERM_COUNTRY" json:"perm_country,omitempty"`
	PermPin            string `xml:"PERM_PIN" json:"perm_pin,omitempty"`
	PermPoa            string `xml:"PERM_POA" json:"perm_poa,omitempty"`
	PermPoaOthers      string `xml:"PERM_POAOTHERS" json:"perm_poa_others,omitempty"`
	PermCorresSameflag string `xml:"PERM_CORRES_SAMEFLAG" json:"perm_corres_sameflag,omitempty"`
	CorresLine1        string `xml:"CORRES_LINE1" json:"corres_line1,omitempty"`
	CorresLine2        string `xml:"CORRES_LINE2" json:"corres_line2,omitempty"`
	CorresLine3        string `xml:"CORRES_LINE3" json:"corres_line3,omitempty"`
	CorresCity         string `xml:"CORRES_CITY" json:"corres_city,omitempty"`
	CorresDist         string `xml:"CORRES_DIST" json:"corres_dist,omitempty"`
	CorresState        string `xml:"CORRES_STATE" json:"corres_state,omitempty"`
	CorresCountry      string `xml:"CORRES_COUNTRY" json:"corres_country,omitempty"`
	CorresPin          string `xml:"CORRES_PIN" json:"corres_pin,omitempty"`
	CorresPoa          string `xml:"CORRES_POA" json:"corres_poa,omitempty"`
	ProofAddress       string `xml:"PROOF_ADDRESS" json:"proof_address,omitempty"`
	ResiStdCode        string `xml:"RESI_STD_CODE" json:"resi_std_code,omitempty"`
	ResiTelNum         string `xml:"RESI_TEL_NUM" json:"resi_tel_num,omitempty"`
	OffStdCode         string `xml:"OFF_STD_CODE" json:"off_std_code,omitempty"`
	OffTelNum          string `xml:"OFF_TEL_NUM" json:"off_tel_num,omitempty"`
	MobCode            string `xml:"MOB_CODE" json:"mob_code,omitempty"`
	MobNum             string `xml:"MOB_NUM" json:"mob_num,omitempty"`
	MobCode2           string `xml:"MOB_CODE_2" json:"mob_code_2,omitempty"`
	MobNum2            string `xml:"MOB_NUM_2" json:"mob_num_2,omitempty"`
	Email              string `xml:"EMAIL" json:"email,omitempty"`
	Email2             string `xml:"EMAIL_2" json:"email_2,omitempty"`
	FaxCode            string `xml:"FAX_CODE" json:"fax_code,omitempty"`
	FaxNo              string `xml:"FAX_NO" json:"fax_no,omitempty"`
	PlaceInc           string `xml:"PLACE_INC" json:"place_inc,omitempty"`
	DateCommBus        string `xml:"DATE_COMM_BUS" json:"date_comm_bus,omitempty"`
	CounInc            string `xml:"COUN_INC" json:"coun_inc,omitempty"`
	TinGst             string `xml:"TIN_GST" json:"tin_gst,omitempty"`
	TinCoun            string `xml:"TIN_COUN" json:"tin_coun,omitempty"`
	IpvFlag            string `xml:"IPV_FLAG" json:"ipv_flag,omitempty"`
	Remarks            string `xml:"REMARKS" json:"remarks,omitempty"`
	DecDate            string `xml:"DEC_DATE" json:"dec_date,omitempty"`
	DecPlace           string `xml:"DEC_PLACE" json:"dec_place,omitempty"`
	KycDate            string `xml:"KYC_DATE" json:"kyc_date,omitempty"`
	DocSub             string `xml:"DOC_SUB" json:"doc_sub,omitempty"`
	KycName            string `xml:"KYC_NAME" json:"kyc_name,omitempty"`
	KycDesignation     string `xml:"KYC_DESIGNATION" json:"kyc_designation,omitempty"`
	KycBranch          string `xml:"KYC_BRANCH" json:"kyc_branch,omitempty"`
	KycEmpcode         string `xml:"KYC_EMPCODE" json:"kyc_empcode,omitempty"`
	OrgName            string `xml:"ORG_NAME" json:"org_name,omitempty"`
	OrgCode            string `xml:"ORG_CODE" json:"org_code,omitempty"`
	NumIdentity        string `xml:"NUM_IDENTITY" json:"num_identity,omitempty"`
	NumRelated         string `xml:"NUM_RELATED" json:"num_related,omitempty"`
	NumImages          string `xml:"NUM_IMAGES" json:"num_images,omitempty"`
}

type IdentityDetails struct {
	Identities []Identity `xml:"IDENTITY" json:"identities,omitempty"`
}

type Identity struct {
	SequenceNo  string `xml:"SEQUENCE_NO" json:"sequence_no,omitempty"`
	IdentType   string `xml:"IDENT_TYPE" json:"ident_type,omitempty"`
	IdentNum    string `xml:"IDENT_NUM" json:"ident_num,omitempty"`
	IdverStatus string `xml:"IDVER_STATUS" json:"idver_status,omitempty"`
}

type RelatedPersonDetails struct {
	RelatedPersons []RelatedPerson `xml:"RELATED_PERSON" json:"related_persons,omitempty"`
}

type RelatedPerson struct {
	SequenceNo                 string `xml:"SEQUENCE_NO" json:"sequence_no,omitempty"`
	RelType                    string `xml:"REL_TYPE" json:"rel_type,omitempty"`
	RelTypeOthers              string `xml:"REL_TYPE_OTHERS" json:"rel_type_others,omitempty"`
	AddDelFlag                 string `xml:"ADD_DEL_FLAG" json:"add_del_flag,omitempty"`
	CkycNo                     string `xml:"CKYC_NO" json:"ckyc_no,omitempty"`
	Prefix                     string `xml:"PREFIX" json:"prefix,omitempty"`
	Fname                      string `xml:"FNAME" json:"fname,omitempty"`
	Mname                      string `xml:"MNAME" json:"mname,omitempty"`
	Lname                      string `xml:"LNAME" json:"lname,omitempty"`
	MaidenPrefix               string `xml:"MAIDEN_PREFIX" json:"maiden_prefix,omitempty"`
	MaidenFname                string `xml:"MAIDEN_FNAME" json:"maiden_fname,omitempty"`
	MaidenMname                string `xml:"MAIDEN_MNAME" json:"maiden_mname,omitempty"`
	MaidenLname                string `xml:"MAIDEN_LNAME" json:"maiden_lname,omitempty"`
	FatherSpouseFlag           string `xml:"FATHERSPOUSE_FLAG" json:"father_spouse_flag,omitempty"`
	FatherPrefix               string `xml:"FATHER_PREFIX" json:"father_prefix,omitempty"`
	FatherFname                string `xml:"FATHER_FNAME" json:"father_fname,omitempty"`
	FatherMname                string `xml:"FATHER_MNAME" json:"father_mname,omitempty"`
	FatherLname                string `xml:"FATHER_LNAME" json:"father_lname,omitempty"`
	MotherPrefix               string `xml:"MOTHER_PREFIX" json:"mother_prefix,omitempty"`
	MotherFname                string `xml:"MOTHER_FNAME" json:"mother_fname,omitempty"`
	MotherMname                string `xml:"MOTHER_MNAME" json:"mother_mname,omitempty"`
	MotherLname                string `xml:"MOTHER_LNAME" json:"mother_lname,omitempty"`
	Dob                        string `xml:"DOB" json:"dob,omitempty"`
	Gender                     string `xml:"GENDER" json:"gender,omitempty"`
	Nationality                string `xml:"NATIONALITY" json:"nationality,omitempty"`
	Pan                        string `xml:"PAN" json:"pan,omitempty"`
	FormSixty                  string `xml:"FORM_SIXTY" json:"form_sixty,omitempty"`
	DisFlag                    string `xml:"DIS_FLAG" json:"dis_flag,omitempty"`
	DisType                    string `xml:"DIS_TYPE" json:"dis_type,omitempty"`
	DisPercent                 string `xml:"DIS_PERCENT" json:"dis_percent,omitempty"`
	DisUdidNumber              string `xml:"DIS_UDID_NUMBER" json:"dis_udid_number,omitempty"`
	ResiStatus                 string `xml:"RESI_STATUS" json:"resi_status,omitempty"`
	AddLine1                   string `xml:"ADD_LINE1" json:"add_line1,omitempty"`
	AddLine2                   string `xml:"ADD_LINE2" json:"add_line2,omitempty"`
	AddLine3                   string `xml:"ADD_LINE3" json:"add_line3,omitempty"`
	AddCity                    string `xml:"ADD_CITY" json:"add_city,omitempty"`
	AddDist                    string `xml:"ADD_DIST" json:"add_dist,omitempty"`
	AddDistrict                string `xml:"ADD_DISTRICT" json:"add_district,omitempty"`
	AddState                   string `xml:"ADD_STATE" json:"add_state,omitempty"`
	AddCountry                 string `xml:"ADD_COUNTRY" json:"add_country,omitempty"`
	AddPin                     string `xml:"ADD_PIN" json:"add_pin,omitempty"`
	PermPoiType                string `xml:"PERM_POI_TYPE" json:"perm_poi_type,omitempty"`
	SameAsPermFlag             string `xml:"SAME_AS_PERM_FLAG" json:"same_as_perm_flag,omitempty"`
	CorresAddLine1             string `xml:"CORRES_ADD_LINE1" json:"corres_add_line1,omitempty"`
	CorresAddLine2             string `xml:"CORRES_ADD_LINE2" json:"corres_add_line2,omitempty"`
	CorresAddLine3             string `xml:"CORRES_ADD_LINE3" json:"corres_add_line3,omitempty"`
	CorresAddCity              string `xml:"CORRES_ADD_CITY" json:"corres_add_city,omitempty"`
	CorresAddDist              string `xml:"CORRES_ADD_DIST" json:"corres_add_dist,omitempty"`
	CorresAddDistrict          string `xml:"CORRES_ADD_DISTRICT" json:"corres_add_district,omitempty"`
	CorresAddState             string `xml:"CORRES_ADD_STATE" json:"corres_add_state,omitempty"`
	CorresAddCountry           string `xml:"CORRES_ADD_COUNTRY" json:"corres_add_country,omitempty"`
	CorresAddPin               string `xml:"CORRES_ADD_PIN" json:"corres_add_pin,omitempty"`
	CorresPoiType              string `xml:"CORRES_POI_TYPE" json:"corres_poi_type,omitempty"`
	ResiStdCode                string `xml:"RESI_STD_CODE" json:"resi_std_code,omitempty"`
	ResiTelNum                 string `xml:"RESI_TEL_NUM" json:"resi_tel_num,omitempty"`
	OffStdCode                 string `xml:"OFF_STD_CODE" json:"off_std_code,omitempty"`
	OffTelNum                  string `xml:"OFF_TEL_NUM" json:"off_tel_num,omitempty"`
	MobCode                    string `xml:"MOB_CODE" json:"mob_code,omitempty"`
	MobNum                     string `xml:"MOB_NUM" json:"mob_num,omitempty"`
	Email                      string `xml:"EMAIL" json:"email,omitempty"`
	Remarks                    string `xml:"REMARKS" json:"remarks,omitempty"`
	PhotoType                  string `xml:"PHOTO_TYPE" json:"photo_type,omitempty"`
	PhotoData                  string `xml:"PHOTO_DATA" json:"photo_data,omitempty"`
	PermPoiImageType           string `xml:"PERM_POI_IMAGE_TYPE" json:"perm_poi_image_type,omitempty"`
	PermPoiData                string `xml:"PERM_POI_DATA" json:"perm_poi_data,omitempty"`
	CorresPoiImageType         string `xml:"CORRES_POI_IMAGE_TYPE" json:"corres_poi_image_type,omitempty"`
	CorresPoiData              string `xml:"CORRES_POI_DATA" json:"corres_poi_data,omitempty"`
	ProofOfPossessionOfAadhaar string `xml:"PROOF_OF_POSSESSION_OF_AADHAAR" json:"proof_of_possession_of_aadhaar,omitempty"`
	VoterId                    string `xml:"VOTERID" json:"voter_id,omitempty"`
	Nrega                      string `xml:"NREGA" json:"nrega,omitempty"`
	Passport                   string `xml:"PASSPORT" json:"passport,omitempty"`
	PassportExp                string `xml:"PASSPORT_EXP" json:"passport_exp,omitempty"`
	ForeignNationalId          string `xml:"FOREIGN_NATIONAL_ID" json:"foreign_national_id,omitempty"`
	DrivingLicence             string `xml:"DRIVING_LICENCE" json:"driving_licence,omitempty"`
	NationalPopulationRegLetter string `xml:"NATIONAL_POPULATION_REG_LETTER" json:"national_population_reg_letter,omitempty"`
	OfflineVerificationAadhaar string `xml:"OFFLINE_VERIFICATION_AADHAAR" json:"offline_verification_aadhaar,omitempty"`
	EKycAuthentication         string `xml:"E_KYC_AUTHENTICATION" json:"e_kyc_authentication,omitempty"`
	DecDate                    string `xml:"DEC_DATE" json:"dec_date,omitempty"`
	DecPlace                   string `xml:"DEC_PLACE" json:"dec_place,omitempty"`
	KycDate                    string `xml:"KYC_DATE" json:"kyc_date,omitempty"`
	DocSub                     string `xml:"DOC_SUB" json:"doc_sub,omitempty"`
	KycName                    string `xml:"KYC_NAME" json:"kyc_name,omitempty"`
	KycDesignation             string `xml:"KYC_DESIGNATION" json:"kyc_designation,omitempty"`
	KycBranch                  string `xml:"KYC_BRANCH" json:"kyc_branch,omitempty"`
	KycEmpcode                 string `xml:"KYC_EMPCODE" json:"kyc_empcode,omitempty"`
	OrgName                    string `xml:"ORG_NAME" json:"org_name,omitempty"`
	OrgCode                    string `xml:"ORG_CODE" json:"org_code,omitempty"`
	Din                        string `xml:"DIN" json:"din,omitempty"`
}

type ImageDetails struct {
	Images []Image `xml:"IMAGE" json:"images,omitempty"`
}

type Image struct {
	SequenceNo string `xml:"SEQUENCE_NO" json:"sequence_no,omitempty"`
	ImageType  string `xml:"IMAGE_TYPE" json:"image_type,omitempty"`
	ImageCode  string `xml:"IMAGE_CODE" json:"image_code,omitempty"`
	GlobalFlag string `xml:"GLOBAL_FLAG" json:"global_flag,omitempty"`
	BranchCode string `xml:"BRANCH_CODE" json:"branch_code,omitempty"`
	ImageData  string `xml:"IMAGE_DATA" json:"image_data,omitempty"`
}

func parseDownloadPid(decryptedPid string) (*DownloadRecord, error) {
	var rec DownloadRecord
	if err := xml.Unmarshal([]byte(decryptedPid), &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func extractTagValue(xmlStr, tag string) string {
	openTag := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	start := strings.Index(xmlStr, openTag)
	if start == -1 {
		return ""
	}
	start += len(openTag)
	end := strings.Index(xmlStr[start:], closeTag)
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(xmlStr[start : start+end])
}

func decodePemBundle(keyStr string) string {
	if !strings.HasPrefix(strings.TrimSpace(keyStr), "-----") {
		decoded, err := base64.StdEncoding.DecodeString(keyStr)
		if err == nil {
			return string(decoded)
		}
	}
	return keyStr
}

func extractPrivateKeyPEM(bundle string) (string, error) {
	rest := []byte(bundle)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		switch block.Type {
		case "RSA PRIVATE KEY", "PRIVATE KEY", "EC PRIVATE KEY", "ENCRYPTED PRIVATE KEY":
			return string(pem.EncodeToMemory(block)), nil
		}
	}
	return "", fmt.Errorf("no private key block found in PEM bundle")
}

func (c *CkycTool) SignXml(ctx context.Context, xmlStr string, rootTag string) (string, error) {
	// Canonicalize (simple version: use the XML string as is since it's compact)
	// Actually, for enveloped signature, we need to digest the REQ_ROOT content
	// but we'll include a Signature element within it.

	// Create SignedInfo
	// We need the digest of the XML without the Signature element
	// Standard C14N for an element with no namespace usually just keeps it as is if it's compact.
	// xml-c14n-20010315 strips the XML declaration; digest the body bytes only
	// so what we hash matches what the server hashes after applying c14n.
	digestInput := xmlStr
	if idx := strings.Index(digestInput, "?>"); idx != -1 {
		digestInput = digestInput[idx+2:]
	}
	xmlDigest := sha.NewSHA256([]byte(digestInput))
	encodedDigest := base64.StdEncoding.EncodeToString(xmlDigest)

	// Build SignedInfo in c14n-canonical form: long-form (no self-closing) tags
	// for empty elements. Server re-canonicalizes the wire SignedInfo before
	// verifying SignatureValue; if the bytes we sign don't match c14n output,
	// the server reports "Digital signature cannot be verified".
	signedInfoXmlStr := fmt.Sprintf("<SignedInfo xmlns=\"http://www.w3.org/2000/09/xmldsig#\">"+
		"<CanonicalizationMethod Algorithm=\"http://www.w3.org/TR/2001/REC-xml-c14n-20010315\"></CanonicalizationMethod>"+
		"<SignatureMethod Algorithm=\"http://www.w3.org/2001/04/xmldsig-more#rsa-sha256\"></SignatureMethod>"+
		"<Reference URI=\"\">"+
		"<Transforms>"+
		"<Transform Algorithm=\"http://www.w3.org/2000/09/xmldsig#enveloped-signature\"></Transform>"+
		"</Transforms>"+
		"<DigestMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#sha256\"></DigestMethod>"+
		"<DigestValue>%s</DigestValue>"+
		"</Reference>"+
		"</SignedInfo>", encodedDigest)

	bundle := decodePemBundle(c.FiPrivateKey)
	privateKeyStr, err := extractPrivateKeyPEM(bundle)
	if err != nil {
		return "", fmt.Errorf("error extracting private key: %w", err)
	}
	signatureBytes, err := rsa.Sign(ctx, []byte(signedInfoXmlStr), privateKeyStr, crypto.SHA256)
	if err != nil {
		return "", err
	}
	encodedSignature := base64.StdEncoding.EncodeToString(signatureBytes)

	signatureXmlStr := fmt.Sprintf("<Signature xmlns=\"http://www.w3.org/2000/09/xmldsig#\">%s<SignatureValue>%s</SignatureValue></Signature>", signedInfoXmlStr, encodedSignature)

	// Insert Signature before the closing root tag
	closingTag := "</" + rootTag + ">"
	finalXml := strings.Replace(xmlStr, closingTag, signatureXmlStr+closingTag, 1)
	return finalXml, nil
}

func (c *CkycTool) GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "fi_code":
		return c.FiCode, nil
	case "ckyc_public_key":
		return c.CkycPublicKey, nil
	case "fi_private_key":
		return c.FiPrivateKey, nil
	case "fi_public_key":
		return c.FiPublicKey, nil
	case "base_url":
		return c.BaseUrl, nil
	default:
		return c.Tool.GetAttribute(ctx, attributeName)
	}
}

func (c *CkycTool) SetAttribute(ctx context.Context, attributeName string, attributeValue interface{}) (err error) {
	switch attributeName {
	case "fi_code":
		c.FiCode = attributeValue.(string)
	case "ckyc_public_key":
		c.CkycPublicKey = attributeValue.(string)
	case "fi_private_key":
		c.FiPrivateKey = attributeValue.(string)
	case "fi_public_key":
		c.FiPublicKey = attributeValue.(string)
	case "base_url":
		c.BaseUrl = attributeValue.(string)
	default:
		return c.Tool.SetAttribute(ctx, attributeName, attributeValue)
	}
	return nil
}

func (c *CkycTool) DecryptPidData(ctx context.Context, encodedPid string, encodedSessionKey string) (string, error) {
	logs.WithContext(ctx).Debug("CkycTool DecryptPidData - Start")

	// 1. Decode base64 session key
	skEncBytes, err := base64.StdEncoding.DecodeString(encodedSessionKey)
	if err != nil {
		return "", fmt.Errorf("error decoding session key: %w", err)
	}

	// 2. Decrypt session key using FI's private key (RSA OAEP SHA256)
	bundle := decodePemBundle(c.FiPrivateKey)
	privateKeyStr, err := extractPrivateKeyPEM(bundle)
	if err != nil {
		return "", fmt.Errorf("error extracting private key: %w", err)
	}
	sessionKey, err := rsa.DecryptOAEP(ctx, skEncBytes, privateKeyStr, nil)
	if err != nil {
		return "", fmt.Errorf("error decrypting session key: %w", err)
	}

	// 3. Decode base64 PID
	pidEncBytes, err := base64.StdEncoding.DecodeString(encodedPid)
	if err != nil {
		return "", fmt.Errorf("error decoding PID: %w", err)
	}

	// 4. Decrypt PID using decrypted session key (AES-256-ECB PKCS7)
	pidDecBytes, err := aes.DecryptECB(ctx, pidEncBytes, sessionKey)
	if err != nil {
		return "", fmt.Errorf("error decrypting PID: %w", err)
	}

	return string(pidDecBytes), nil
}

// XML Structs
type PidData struct {
	XMLName  xml.Name `xml:"PID_DATA"`
	DateTime string   `xml:"DATE_TIME"`
	IdNo     string   `xml:"ID_NO"`
	IdType   string   `xml:"ID_TYPE"`
}

type ReqRoot struct {
	XMLName xml.Name `xml:"REQ_ROOT"`
	Header  Header   `xml:"HEADER"`
	CkycInq CkycInq  `xml:"CKYC_INQ"`
}

type DownloadRequestRoot struct {
	XMLName xml.Name `xml:"CKYC_DOWNLOAD_REQUEST"`
	Header  Header   `xml:"HEADER"`
	CkycInq CkycInq  `xml:"CKYC_INQ"`
}

type DownloadPidData struct {
	XMLName        xml.Name `xml:"PID_DATA"`
	DateTime       string   `xml:"DATE_TIME"`
	CkycNo         string   `xml:"CKYC_NO"`
	AuthFactorType string   `xml:"AUTH_FACTOR_TYPE"`
	AuthFactor     string   `xml:"AUTH_FACTOR"`
}

type ValidateOtpPidData struct {
	XMLName  xml.Name `xml:"PID_DATA"`
	DateTime string   `xml:"DATE_TIME"`
	Otp      string   `xml:"OTP"`
	Validate string   `xml:"VALIDATE"`
}

type Header struct {
	FiCode    string `xml:"FI_CODE"`
	RequestId string `xml:"REQUEST_ID"`
	Version   string `xml:"VERSION"`
}

type CkycInq struct {
	Pid        string `xml:"PID"`
	SessionKey string `xml:"SESSION_KEY"`
}

type Signature struct {
	XMLName        xml.Name   `xml:"Signature"`
	Xmlns          string     `xml:"xmlns,attr"`
	SignedInfo     SignedInfo `xml:"SignedInfo"`
	SignatureValue string     `xml:"SignatureValue"`
}

type SignedInfo struct {
	XMLName                xml.Name               `xml:"SignedInfo"`
	CanonicalizationMethod CanonicalizationMethod `xml:"CanonicalizationMethod"`
	SignatureMethod        SignatureMethod        `xml:"SignatureMethod"`
	Reference              Reference              `xml:"Reference"`
}

type CanonicalizationMethod struct {
	Algorithm string `xml:"Algorithm,attr"`
}

type SignatureMethod struct {
	Algorithm string `xml:"Algorithm,attr"`
}

type Reference struct {
	Uri          string       `xml:"URI,attr"`
	Transforms   Transforms   `xml:"Transforms"`
	DigestMethod DigestMethod `xml:"DigestMethod"`
	DigestValue  string       `xml:"DigestValue"`
}

type Transforms struct {
	Transform []Transform `xml:"Transform"`
}

type Transform struct {
	Algorithm string `xml:"Algorithm,attr"`
}

type DigestMethod struct {
	Algorithm string `xml:"Algorithm,attr"`
}

func (c *CkycTool) VerifyXml(ctx context.Context, xmlStr string) (bool, error) {
	logs.WithContext(ctx).Debug("VerifyXml - Start")

	// 1. Extract SignatureValue
	sigValStart := strings.Index(xmlStr, "<SignatureValue>")
	sigValEnd := strings.Index(xmlStr, "</SignatureValue>")
	if sigValStart == -1 || sigValEnd == -1 {
		return false, fmt.Errorf("SignatureValue not found")
	}
	encodedSig := strings.TrimSpace(xmlStr[sigValStart+len("<SignatureValue>") : sigValEnd])
	encodedSig = strings.ReplaceAll(encodedSig, "\n", "")
	encodedSig = strings.ReplaceAll(encodedSig, "\r", "")
	encodedSig = strings.ReplaceAll(encodedSig, " ", "")

	signatureBytes, err := base64.StdEncoding.DecodeString(encodedSig)
	if err != nil {
		return false, fmt.Errorf("error decoding SignatureValue: %w", err)
	}

	// 2. Extract SignedInfo
	siStart := strings.Index(xmlStr, "<SignedInfo")
	siEnd := strings.Index(xmlStr, "</SignedInfo>")
	if siStart == -1 || siEnd == -1 {
		return false, fmt.Errorf("SignedInfo not found")
	}
	signedInfoStr := xmlStr[siStart : siEnd+len("</SignedInfo>")]

	// 3. Verify Signature over SignedInfo
	publicKeyStr := c.FiPublicKey
	if publicKeyStr == "" {
		return false, fmt.Errorf("fi_public_key is empty")
	}
	publicKeyStr = decodePemBundle(publicKeyStr)

	err = rsa.VerifyWithCert(ctx, []byte(signedInfoStr), signatureBytes, publicKeyStr, crypto.SHA256)
	if err != nil {
		// Try regular public key if cert fails
		err2 := rsa.Verify(ctx, []byte(signedInfoStr), signatureBytes, publicKeyStr, crypto.SHA256)
		if err2 != nil {
			return false, fmt.Errorf("signature verification failed: %w", err2)
		}
	}

	// 4. Verify DigestValue
	dvStart := strings.Index(signedInfoStr, "<DigestValue>")
	dvEnd := strings.Index(signedInfoStr, "</DigestValue>")
	if dvStart == -1 || dvEnd == -1 {
		return false, fmt.Errorf("DigestValue not found")
	}
	encodedDigest := strings.TrimSpace(signedInfoStr[dvStart+len("<DigestValue>") : dvEnd])

	// Reconstruct original XML by removing <Signature>...</Signature>
	sigStart := strings.Index(xmlStr, "<Signature")
	sigEnd := strings.Index(xmlStr, "</Signature>")
	if sigStart == -1 || sigEnd == -1 {
		return false, fmt.Errorf("Signature element not found")
	}
	originalXml := xmlStr[:sigStart] + xmlStr[sigEnd+len("</Signature>"):]

	recalculatedDigest := sha.NewSHA256([]byte(originalXml))
	encodedRecalculated := base64.StdEncoding.EncodeToString(recalculatedDigest)

	if encodedDigest != recalculatedDigestStr(encodedRecalculated) && encodedDigest != encodedRecalculated {
		return false, fmt.Errorf("DigestValue mismatch: expected %s, got %s", encodedDigest, encodedRecalculated)
	}

	return true, nil
}

// recalculatedDigestStr is a helper to handle potential minor formatting differences if any
func recalculatedDigestStr(s string) string {
	return s
}

func (ckycTool *CkycTool) GetBytes(ctx context.Context) ([]byte, error) {
	toolJson, err := json.Marshal(ckycTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		Public:      true,
		ToolType:    "Ckyc",
		Category:    "KYC/Compliance",
		Description: "Central KYC registry search and verification for financial compliance",
		Actions: func() []tools.ActionInfo {
			infos := make([]tools.ActionInfo, len(ckycToolActions))
			for i, a := range ckycToolActions {
				infos[i] = tools.ActionInfo{Name: a.ActionName, Description: a.Description}
			}
			return infos
		}(),
		OAuthEnabled: false,
		Icon:         "",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(CkycTool{}), []string{}),
	})
}
