package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/segmentio/ksuid"
)

const (
	gdriveApiBase    = "https://www.googleapis.com/drive/v3"
	gdriveUploadBase = "https://www.googleapis.com/upload/drive/v3"
	gdriveFolderMime = "application/vnd.google-apps.folder"
)

type GdriveStorage struct {
	Storage
	AuthName     string `json:"auth_name" eru:"required"`
	RefreshToken string `json:"refresh_token" eru:"required"`
	RootFolderId string `json:"root_folder_id"`

	tokenMu     sync.Mutex
	accessToken string
	expiresAt   time.Time
}

func (g *GdriveStorage) GetAttribute(attributeName string) (interface{}, error) {
	switch attributeName {
	case "storage_name":
		return g.StorageName, nil
	case "storage_type":
		return g.StorageType, nil
	case "key_pair":
		return g.KeyPair, nil
	case "key_id":
		return g.KmsId, nil
	case "auth_name":
		return g.AuthName, nil
	case "root_folder_id":
		return g.RootFolderId, nil
	default:
		return nil, errors.New("Attribute not found")
	}
}

func (g *GdriveStorage) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	if err := json.Unmarshal(*rj, &g); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	if g.EncryptFiles {
		logs.WithContext(ctx).Warn("encrypt_files is not supported for GDRIVE storage; flag will be ignored")
	}
	return nil
}

func (g *GdriveStorage) Init(ctx context.Context) error {
	return nil
}

func (g *GdriveStorage) ensureAccessToken(ctx context.Context) (string, error) {
	g.tokenMu.Lock()
	defer g.tokenMu.Unlock()
	if g.accessToken != "" && time.Until(g.expiresAt) > 5*time.Minute {
		return g.accessToken, nil
	}
	if g.AuthName == "" {
		return "", errors.New("auth_name is required for GDRIVE storage")
	}
	if g.RefreshToken == "" {
		return "", errors.New("refresh_token is required for GDRIVE storage")
	}
	projectId, _ := ctx.Value("projectId").(string)
	if projectId == "" {
		return "", errors.New("projectId not found in context for GDRIVE token refresh")
	}
	baseUrl := os.Getenv("ERUAUTH_BASEURL")
	if baseUrl == "" {
		return "", errors.New("ERUAUTH_BASEURL env not set")
	}
	url := fmt.Sprintf("%s/%s/%s/idptoken/renew", strings.TrimRight(baseUrl, "/"), projectId, g.AuthName)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	body := map[string]interface{}{"refresh_token": g.RefreshToken}
	res, _, _, statusCode, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, nil, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("idptoken renew failed (status %d): %s", statusCode, err.Error()))
		return "", err
	}
	resMap, ok := res.(map[string]interface{})
	if !ok {
		return "", errors.New("idptoken response is not an object")
	}
	at, _ := resMap["access_token"].(string)
	if at == "" {
		return "", errors.New("idptoken response missing access_token")
	}
	expSec := 3600.0
	if v, ok := resMap["expires_in"].(float64); ok && v > 0 {
		expSec = v
	}
	if rt, ok := resMap["refresh_token"].(string); ok && rt != "" {
		g.RefreshToken = rt
	}
	g.accessToken = at
	g.expiresAt = time.Now().Add(time.Duration(expSec) * time.Second)
	return at, nil
}

func (g *GdriveStorage) driveCall(ctx context.Context, method, fullUrl string, headers http.Header, params map[string]string, postBody interface{}) (interface{}, http.Header, int, error) {
	tok, err := g.ensureAccessToken(ctx)
	if err != nil {
		return nil, nil, 0, err
	}
	if headers == nil {
		headers = http.Header{}
	}
	headers.Set("Authorization", "Bearer "+tok)
	res, respHeaders, _, status, err := utils.CallHttp(ctx, method, fullUrl, headers, map[string]string{}, nil, params, postBody)
	if err != nil && status == http.StatusUnauthorized {
		g.tokenMu.Lock()
		g.accessToken = ""
		g.tokenMu.Unlock()
		tok, terr := g.ensureAccessToken(ctx)
		if terr != nil {
			return nil, nil, status, terr
		}
		headers.Set("Authorization", "Bearer "+tok)
		res, respHeaders, _, status, err = utils.CallHttp(ctx, method, fullUrl, headers, map[string]string{}, nil, params, postBody)
	}
	return res, respHeaders, status, err
}

func (g *GdriveStorage) parentFolderId() string {
	if g.RootFolderId != "" {
		return g.RootFolderId
	}
	return "root"
}

func escapeDriveQ(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return s
}

func (g *GdriveStorage) findChild(ctx context.Context, parentId, name string, folderOnly bool) (string, error) {
	q := fmt.Sprintf("name = '%s' and '%s' in parents and trashed = false", escapeDriveQ(name), escapeDriveQ(parentId))
	if folderOnly {
		q += " and mimeType = '" + gdriveFolderMime + "'"
	} else {
		q += " and mimeType != '" + gdriveFolderMime + "'"
	}
	params := map[string]string{
		"q":                         q,
		"fields":                    "files(id,name,mimeType)",
		"pageSize":                  "1",
		"spaces":                    "drive",
		"includeItemsFromAllDrives": "true",
		"supportsAllDrives":         "true",
	}
	res, _, status, err := g.driveCall(ctx, http.MethodGet, gdriveApiBase+"/files", http.Header{}, params, nil)
	if err != nil {
		return "", err
	}
	if status >= 300 {
		return "", fmt.Errorf("drive list returned status %d", status)
	}
	resMap, ok := res.(map[string]interface{})
	if !ok {
		return "", errors.New("drive list response not an object")
	}
	filesArr, _ := resMap["files"].([]interface{})
	if len(filesArr) == 0 {
		return "", nil
	}
	first, ok := filesArr[0].(map[string]interface{})
	if !ok {
		return "", nil
	}
	id, _ := first["id"].(string)
	return id, nil
}

func (g *GdriveStorage) ensureFolder(ctx context.Context, parentId, name string) (string, error) {
	id, err := g.findChild(ctx, parentId, name, true)
	if err != nil {
		return "", err
	}
	if id != "" {
		return id, nil
	}
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	body := map[string]interface{}{
		"name":     name,
		"mimeType": gdriveFolderMime,
		"parents":  []string{parentId},
	}
	res, _, status, err := g.driveCall(ctx, http.MethodPost, gdriveApiBase+"/files", headers, map[string]string{"supportsAllDrives": "true"}, body)
	if err != nil {
		return "", err
	}
	if status >= 300 {
		return "", fmt.Errorf("drive create folder returned %d", status)
	}
	resMap, _ := res.(map[string]interface{})
	id, _ = resMap["id"].(string)
	if id == "" {
		return "", errors.New("drive create folder missing id")
	}
	return id, nil
}

func (g *GdriveStorage) resolveFolderPath(ctx context.Context, folderPath string, createIfMissing bool) (string, error) {
	parent := g.parentFolderId()
	folderPath = strings.Trim(folderPath, "/")
	if folderPath == "" {
		return parent, nil
	}
	for _, part := range strings.Split(folderPath, "/") {
		if part == "" {
			continue
		}
		var id string
		var err error
		if createIfMissing {
			id, err = g.ensureFolder(ctx, parent, part)
		} else {
			id, err = g.findChild(ctx, parent, part, true)
			if err == nil && id == "" {
				err = fmt.Errorf("folder not found: %s", part)
			}
		}
		if err != nil {
			return "", err
		}
		parent = id
	}
	return parent, nil
}

func buildGdriveFileName(docType, baseName string) string {
	id := ksuid.New().String()
	if docType != "" {
		return fmt.Sprint(docType, "_", id, "_", baseName)
	}
	return fmt.Sprint(id, "_", baseName)
}

func joinDocId(folderPath, fileName string) string {
	folderPath = strings.Trim(folderPath, "/")
	if folderPath == "" {
		return fileName
	}
	return folderPath + "/" + fileName
}

func (g *GdriveStorage) uploadBytes(ctx context.Context, parentId, fileName string, data []byte, contentType string) (string, error) {
	tok, err := g.ensureAccessToken(ctx)
	if err != nil {
		return "", err
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	meta := map[string]interface{}{
		"name":    fileName,
		"parents": []string{parentId},
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return "", err
	}
	boundary := "eru-gdrive-" + ksuid.New().String()
	var body bytes.Buffer
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Type: application/json; charset=UTF-8\r\n\r\n")
	body.Write(metaBytes)
	body.WriteString("\r\n--" + boundary + "\r\n")
	body.WriteString("Content-Type: " + contentType + "\r\n\r\n")
	body.Write(data)
	body.WriteString("\r\n--" + boundary + "--")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gdriveUploadBase+"/files?uploadType=multipart&supportsAllDrives=true", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "multipart/related; boundary="+boundary)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("drive upload failed: status %d body %s", resp.StatusCode, string(respBytes))
	}
	var respObj map[string]interface{}
	if err := json.Unmarshal(respBytes, &respObj); err != nil {
		return "", err
	}
	id, _ := respObj["id"].(string)
	return id, nil
}

func (g *GdriveStorage) UploadFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error) {
	logs.WithContext(ctx).Debug("UploadFile - Start (GDRIVE)")
	if g.EncryptFiles {
		logs.WithContext(ctx).Warn("encrypt_files ignored for GDRIVE")
	}
	byteContainer, err := io.ReadAll(file)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	finalFileName := buildGdriveFileName(docType, header.Filename)
	parentId, err := g.resolveFolderPath(ctx, folderPath, true)
	if err != nil {
		return "", err
	}
	if _, err = g.uploadBytes(ctx, parentId, finalFileName, byteContainer, header.Header.Get("Content-Type")); err != nil {
		return "", err
	}
	return joinDocId(folderPath, finalFileName), nil
}

func (g *GdriveStorage) UploadFileB64(ctx context.Context, file []byte, fileName string, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error) {
	logs.WithContext(ctx).Debug("UploadFileB64 - Start (GDRIVE)")
	if g.EncryptFiles {
		logs.WithContext(ctx).Warn("encrypt_files ignored for GDRIVE")
	}
	finalFileName := buildGdriveFileName(docType, fileName)
	parentId, err := g.resolveFolderPath(ctx, folderPath, true)
	if err != nil {
		return "", err
	}
	if _, err = g.uploadBytes(ctx, parentId, finalFileName, file, ""); err != nil {
		return "", err
	}
	return joinDocId(folderPath, finalFileName), nil
}

func (g *GdriveStorage) DownloadFile(ctx context.Context, folderPath string, fileName string, keyName eruaes.AesKey) ([]byte, error) {
	logs.WithContext(ctx).Debug("DownloadFile - Start (GDRIVE)")
	parentId, err := g.resolveFolderPath(ctx, folderPath, false)
	if err != nil {
		return nil, err
	}
	fileId, err := g.findChild(ctx, parentId, fileName, false)
	if err != nil {
		return nil, err
	}
	if fileId == "" {
		return nil, fmt.Errorf("file not found: %s", joinDocId(folderPath, fileName))
	}
	tok, err := g.ensureAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	dlUrl := fmt.Sprintf("%s/files/%s?alt=media&supportsAllDrives=true", gdriveApiBase, fileId)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dlUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("drive download failed: status %d body %s", resp.StatusCode, string(b))
	}
	return io.ReadAll(resp.Body)
}

func (g *GdriveStorage) BucketExists(ctx context.Context) (bool, error) {
	return g.RootFolderId != "", nil
}

func (g *GdriveStorage) CreateStorage(ctx context.Context, cloneStorage StorageI, persist bool) error {
	if !persist {
		return nil
	}
	exists, err := cloneStorage.BucketExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		logs.WithContext(ctx).Info("skipping GDrive folder creation as root_folder_id is already set")
		return nil
	}
	nameI, _ := cloneStorage.GetAttribute("storage_name")
	name, _ := nameI.(string)
	if name == "" {
		return errors.New("storage_name required to create GDrive folder")
	}
	id, err := g.ensureFolder(ctx, "root", name)
	if err != nil {
		return err
	}
	g.RootFolderId = id
	if cg, ok := cloneStorage.(*GdriveStorage); ok {
		cg.RootFolderId = id
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("created GDrive root folder: %s (id=%s)", name, id))
	return nil
}

func (g *GdriveStorage) DeleteStorage(ctx context.Context, forceDelete bool, cloneStorage StorageI) error {
	cloneG, ok := cloneStorage.(*GdriveStorage)
	if !ok {
		return errors.New("clone is not GdriveStorage")
	}
	if cloneG.RootFolderId == "" {
		logs.WithContext(ctx).Info("GDrive storage has no root_folder_id; nothing to delete")
		return nil
	}
	if forceDelete {
		if err := cloneStorage.EmptyBucket(ctx); err != nil {
			return err
		}
	}
	_, _, status, err := cloneG.driveCall(ctx, http.MethodDelete, fmt.Sprintf("%s/files/%s", gdriveApiBase, cloneG.RootFolderId), http.Header{}, map[string]string{"supportsAllDrives": "true"}, nil)
	if err != nil && status != http.StatusNotFound {
		return err
	}
	if status >= 300 && status != http.StatusNotFound {
		return fmt.Errorf("drive delete folder returned %d", status)
	}
	return nil
}

func (g *GdriveStorage) EmptyBucket(ctx context.Context) error {
	if g.RootFolderId == "" {
		return errors.New("EmptyBucket refused: root_folder_id is not set (refusing to empty entire Drive)")
	}
	parent := g.RootFolderId
	pageToken := ""
	for {
		params := map[string]string{
			"q":                         fmt.Sprintf("'%s' in parents and trashed = false", escapeDriveQ(parent)),
			"fields":                    "nextPageToken, files(id,name)",
			"pageSize":                  "100",
			"spaces":                    "drive",
			"includeItemsFromAllDrives": "true",
			"supportsAllDrives":         "true",
		}
		if pageToken != "" {
			params["pageToken"] = pageToken
		}
		res, _, status, err := g.driveCall(ctx, http.MethodGet, gdriveApiBase+"/files", http.Header{}, params, nil)
		if err != nil {
			return err
		}
		if status >= 300 {
			return fmt.Errorf("drive list returned %d", status)
		}
		m, _ := res.(map[string]interface{})
		filesArr, _ := m["files"].([]interface{})
		for _, f := range filesArr {
			fm, _ := f.(map[string]interface{})
			id, _ := fm["id"].(string)
			if id == "" {
				continue
			}
			_, _, dStatus, dErr := g.driveCall(ctx, http.MethodDelete, fmt.Sprintf("%s/files/%s", gdriveApiBase, id), http.Header{}, map[string]string{"supportsAllDrives": "true"}, nil)
			if dErr != nil && dStatus != http.StatusNotFound {
				return dErr
			}
			if dStatus >= 300 && dStatus != http.StatusNotFound {
				return fmt.Errorf("drive delete file returned %d", dStatus)
			}
		}
		nt, _ := m["nextPageToken"].(string)
		if nt == "" {
			return nil
		}
		pageToken = nt
	}
}
