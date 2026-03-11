package ckyc

import (
	"bytes"
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	aes "github.com/eru-tech/eru/eru-crypto/aes"
	rsa "github.com/eru-tech/eru/eru-crypto/rsa"
	sha "github.com/eru-tech/eru/eru-crypto/sha"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
	uuid "github.com/google/uuid"
)

const CKYC_VERSION = 1.3

type CkycVerifyParams struct {
	IdNo   string `json:"id_no" eru:"required" desc:"ID number for search"`
	IdType string `json:"id_type" eru:"required" desc:"ID type (A-Aadhaar, B-PAN, C-Voter ID, D-Passport, E-Driving License, G-MGNREGA Job Card)"`
}

type CkycTool struct {
	tools.Tool
	FiCode        string `json:"fi_code"`
	CkycPublicKey string `json:"ckyc_public_key"`
	FiPrivateKey  string `json:"fi_private_key"`
	BaseUrl       string `json:"base_url"`
}

const (
	VERIFY = "verify"
)

var ckycToolActions = []tools.ToolAction{
	{
		ActionName:   VERIFY,
		Description:  "Search CKYC details using ID number and type",
		SystemPrompt: "Search CKYC details using ID number and type",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(CkycVerifyParams{}))
		},
	},
}

func (c *CkycTool) GetActionsList() []string {
	actions := []string{}
	for _, action := range ckycToolActions {
		actions = append(actions, action.ActionName)
	}
	return actions
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
	switch actionName {
	case VERIFY:
		return c.ExecuteVerify(ctx, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
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

func (c *CkycTool) ExecuteVerify(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CkycTool ExecuteVerify - Start")

	verifyParams := CkycVerifyParams{}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, false, fmt.Errorf("error marshalling params: %w", err)
	}
	err = json.Unmarshal(paramsBytes, &verifyParams)
	if err != nil {
		return nil, false, fmt.Errorf("error unmarshalling params: %w", err)
	}

	// 1 & 2. Generate PID_DATA XML with Timestamp
	timestamp := time.Now().Format("02-01-2006 15:04:05")
	pidData := PidData{
		DateTime: timestamp,
		IdNo:     verifyParams.IdNo,
		IdType:   verifyParams.IdType,
	}
	pidXml, err := xml.Marshal(pidData)
	if err != nil {
		return nil, false, fmt.Errorf("error marshalling PID_DATA: %w", err)
	}
	pidXmlStr := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" + string(pidXml)

	// 3. Generate random 256-bit session key
	sessionKey, err := aes.GenerateKey(ctx, 32)
	if err != nil {
		return nil, false, fmt.Errorf("error generating session key: %w", err)
	}

	// 4 & 5. Encrypt PID_DATA using session key (AES-256-ECB PKCS7)
	encryptedPid, err := aes.EncryptECB(ctx, []byte(pidXmlStr), sessionKey.Key)
	if err != nil {
		return nil, false, fmt.Errorf("error encrypting PID_DATA: %w", err)
	}
	encodedPid := base64.StdEncoding.EncodeToString(encryptedPid)

	// 6 & 7. Encrypt session key using CKYC public key (RSA OAEP SHA1)
	publicKeyStr, err := base64.StdEncoding.DecodeString(c.CkycPublicKey)
	if err != nil {
		return nil, false, fmt.Errorf("error decoding public key: %w", err)
	}

	encryptedSessionKey, err := rsa.EncryptOAEP(ctx, sessionKey.Key, string(publicKeyStr), nil)
	if err != nil {
		return nil, false, fmt.Errorf("error encrypting session key: %w", err)
	}
	encodedSessionKey := base64.StdEncoding.EncodeToString(encryptedSessionKey)

	// 8. Wrap everything in REQ_ROOT
	requestId := uuid.New().String()
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
		return nil, false, fmt.Errorf("error marshalling REQ_ROOT: %w", err)
	}
	// 9. Sign the request (without the XML declaration)
	signedXml, err := c.SignXml(ctx, string(rawXml))
	if err != nil {
		return nil, false, fmt.Errorf("error signing XML: %w", err)
	}

	// Add XML declaration to the final signed XML
	finalSignedXml := `<?xml version="1.0" encoding="UTF-8"?>` + signedXml

	// 10. Call the CKYC API
	if c.BaseUrl == "" {
		return nil, false, fmt.Errorf("base url is not configured")
	}

	headers := http.Header{}
	headers.Add("Content-Type", "application/xml")
	// Some APIs might require IP address in a header or just rely on source IP
	url := fmt.Sprintf("%s%s", strings.TrimSuffix(c.BaseUrl, "/"), "/Search/ckycverificationservice/verify")

	// We use ExecuteHttp directly to avoid utils.CallHttp's automatic JSON-marshalling of the body
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(finalSignedXml))
	if err != nil {
		return nil, false, fmt.Errorf("error creating request: %w", err)
	}
	req.Header = headers

	resp, err := utils.ExecuteHttp(ctx, req)
	if err != nil {
		return nil, false, fmt.Errorf("error calling CKYC API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("error reading response body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var res map[string]interface{}
		if err := json.Unmarshal(respBody, &res); err != nil {
			return map[string]interface{}{"response": string(respBody)}, false, nil
		}
		return res, false, nil
	}

	return map[string]interface{}{"response": string(respBody)}, false, nil
}

func (c *CkycTool) SignXml(ctx context.Context, xmlStr string) (string, error) {
	// Canonicalize (simple version: use the XML string as is since it's compact)
	// Actually, for enveloped signature, we need to digest the REQ_ROOT content
	// but we'll include a Signature element within it.

	// Create SignedInfo
	// We need the digest of the XML without the Signature element
	xmlDigest := sha.NewSHA1([]byte(xmlStr))
	encodedDigest := base64.StdEncoding.EncodeToString(xmlDigest)

	signedInfo := SignedInfo{
		CanonicalizationMethod: CanonicalizationMethod{Algorithm: "http://www.w3.org/TR/2001/REC-xml-c14n-20010315"},
		SignatureMethod:        SignatureMethod{Algorithm: "http://www.w3.org/2000/09/xmldsig#rsa-sha1"},
		Reference: Reference{
			Uri: "",
			Transforms: Transforms{
				Transform: []Transform{
					{Algorithm: "http://www.w3.org/2000/09/xmldsig#envelopedsignature"},
				},
			},
			DigestMethod: DigestMethod{Algorithm: "http://www.w3.org/2000/09/xmldsig#sha1"},
			DigestValue:  encodedDigest,
		},
	}

	signedInfoXml, err := xml.Marshal(signedInfo)
	if err != nil {
		return "", err
	}
	// Add xmlns to SignedInfo for canonicalization
	signedInfoXmlStr := strings.Replace(string(signedInfoXml), "<SignedInfo", "<SignedInfo xmlns=\"http://www.w3.org/2000/09/xmldsig#\"", 1)

	// Sign SignedInfo
	privateKeyStr := c.FiPrivateKey
	if !strings.HasPrefix(strings.TrimSpace(privateKeyStr), "-----") {
		decoded, err := base64.StdEncoding.DecodeString(privateKeyStr)
		if err == nil {
			privateKeyStr = string(decoded)
		}
	}
	signatureBytes, err := rsa.Sign(ctx, []byte(signedInfoXmlStr), privateKeyStr, crypto.SHA1)
	if err != nil {
		return "", err
	}
	encodedSignature := base64.StdEncoding.EncodeToString(signatureBytes)

	// Create Final Signature Element
	signature := Signature{
		Xmlns:          "http://www.w3.org/2000/09/xmldsig#",
		SignedInfo:     signedInfo,
		SignatureValue: encodedSignature,
	}
	signatureXml, err := xml.Marshal(signature)
	if err != nil {
		return "", err
	}

	// Insert Signature before </REQ_ROOT>
	finalXml := strings.Replace(xmlStr, "</REQ_ROOT>", string(signatureXml)+"</REQ_ROOT>", 1)
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

	// 2. Decrypt session key using FI's private key (RSA OAEP SHA1)
	privateKeyStr := c.FiPrivateKey
	if !strings.HasPrefix(strings.TrimSpace(privateKeyStr), "-----") {
		decoded, err := base64.StdEncoding.DecodeString(privateKeyStr)
		if err == nil {
			privateKeyStr = string(decoded)
		}
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

type Header struct {
	FiCode    string  `xml:"FI_CODE"`
	RequestId string  `xml:"REQUEST_ID"`
	Version   float32 `xml:"VERSION"`
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

func (ckycTool *CkycTool) GetBytes(ctx context.Context) ([]byte, error) {
	toolJson, err := json.Marshal(ckycTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}
