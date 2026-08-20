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
	"net/url"
	"strings"

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
	RootFolderId string `json:"root_folder_id"`
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

func (g *GdriveStorage) getAccessToken(ctx context.Context, projectId string) (string, error) {
	if g.AuthName == "" {
		return "", errors.New("auth_name is required for GDRIVE storage")
	}
	if projectId == "" {
		return "", errors.New("projectId is required for GDRIVE getAccessToken")
	}
	baseUrl, _ := ctx.Value("eruauthbaseurl").(string)
	if baseUrl == "" {
		return "", errors.New("eruauthbaseurl not found in context")
	}
	url := fmt.Sprintf("%s/%s/%s/gettoken", strings.TrimRight(baseUrl, "/"), projectId, g.AuthName)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	qParams := map[string]string{}
	if prefix, _ := ctx.Value("tokenkeyprefix").(string); prefix != "" {
		qParams["token_key_prefix"] = prefix
	}
	res, _, _, statusCode, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, nil, qParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("gettoken call failed (status %d): %s", statusCode, err.Error()))
		return "", err
	}
	resMap, ok := res.(map[string]interface{})
	if !ok {
		return "", errors.New("gettoken response is not an object")
	}
	at, _ := resMap["access_token"].(string)
	if at == "" {
		return "", errors.New("gettoken response missing access_token")
	}
	return at, nil
}

func (g *GdriveStorage) driveCall(ctx context.Context, projectId, method, fullUrl string, headers http.Header, params map[string]string, postBody interface{}) (interface{}, http.Header, int, error) {
	tok, err := g.getAccessToken(ctx, projectId)
	if err != nil {
		return nil, nil, 0, err
	}
	if headers == nil {
		headers = http.Header{}
	}
	headers.Set("Authorization", "Bearer "+tok)
	headers.Set("Content-Type", "application/json")
	res, respHeaders, _, status, err := utils.CallHttp(ctx, method, fullUrl, headers, map[string]string{}, nil, params, postBody)
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

func (g *GdriveStorage) findChild(ctx context.Context, projectId, parentId, name string, folderOnly bool) (string, error) {
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
	res, _, status, err := g.driveCall(ctx, projectId, http.MethodGet, gdriveApiBase+"/files", http.Header{}, params, nil)
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

func (g *GdriveStorage) ensureFolder(ctx context.Context, projectId, parentId, name string) (string, error) {
	id, err := g.findChild(ctx, projectId, parentId, name, true)
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
	res, _, status, err := g.driveCall(ctx, projectId, http.MethodPost, gdriveApiBase+"/files", headers, map[string]string{"supportsAllDrives": "true"}, body)
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

func (g *GdriveStorage) resolveFolderPath(ctx context.Context, projectId, folderPath string, createIfMissing bool) (string, error) {
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
			id, err = g.ensureFolder(ctx, projectId, parent, part)
		} else {
			id, err = g.findChild(ctx, projectId, parent, part, true)
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

func (g *GdriveStorage) uploadBytes(ctx context.Context, projectId, parentId, fileName string, data []byte, contentType string) (string, error) {
	tok, err := g.getAccessToken(ctx, projectId)
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

func (g *GdriveStorage) UploadFile(ctx context.Context, projectId string, file multipart.File, header *multipart.FileHeader, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error) {
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
	parentId, err := g.resolveFolderPath(ctx, projectId, folderPath, true)
	if err != nil {
		return "", err
	}
	if _, err = g.uploadBytes(ctx, projectId, parentId, finalFileName, byteContainer, header.Header.Get("Content-Type")); err != nil {
		return "", err
	}
	return joinDocId(folderPath, finalFileName), nil
}

func (g *GdriveStorage) UploadFileB64(ctx context.Context, projectId string, file []byte, fileName string, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error) {
	logs.WithContext(ctx).Debug("UploadFileB64 - Start (GDRIVE)")
	if g.EncryptFiles {
		logs.WithContext(ctx).Warn("encrypt_files ignored for GDRIVE")
	}
	finalFileName := buildGdriveFileName(docType, fileName)
	parentId, err := g.resolveFolderPath(ctx, projectId, folderPath, true)
	if err != nil {
		return "", err
	}
	if _, err = g.uploadBytes(ctx, projectId, parentId, finalFileName, file, ""); err != nil {
		return "", err
	}
	return joinDocId(folderPath, finalFileName), nil
}

func (g *GdriveStorage) DownloadFile(ctx context.Context, projectId string, folderPath string, fileName string, keyName eruaes.AesKey) ([]byte, error) {
	logs.WithContext(ctx).Debug("DownloadFile - Start (GDRIVE)")
	parentId, err := g.resolveFolderPath(ctx, projectId, folderPath, false)
	if err != nil {
		return nil, err
	}
	fileId, err := g.findChild(ctx, projectId, parentId, fileName, false)
	if err != nil {
		return nil, err
	}
	if fileId == "" {
		return nil, fmt.Errorf("file not found: %s", joinDocId(folderPath, fileName))
	}
	tok, err := g.getAccessToken(ctx, projectId)
	if err != nil {
		return nil, err
	}
	data, _, _, err := g.downloadByDriveId(ctx, tok, fileId, "")
	return data, err
}

type GdriveSearchFilters struct {
	FileName      string
	SharedWithMe  bool
	OwnerEmail    string
	ModifiedAfter string
	MimeType      string
	MaxResults    int
}

func gdriveBuildSearchQuery(f GdriveSearchFilters) string {
	parts := []string{"trashed = false"}
	if f.FileName != "" {
		parts = append(parts, fmt.Sprintf("name = '%s'", escapeDriveQ(f.FileName)))
	}
	if f.SharedWithMe {
		parts = append(parts, "sharedWithMe = true")
	}
	if f.OwnerEmail != "" {
		parts = append(parts, fmt.Sprintf("'%s' in owners", escapeDriveQ(f.OwnerEmail)))
	}
	if f.ModifiedAfter != "" {
		parts = append(parts, fmt.Sprintf("modifiedTime > '%s'", escapeDriveQ(f.ModifiedAfter)))
	}
	if f.MimeType != "" {
		parts = append(parts, fmt.Sprintf("mimeType = '%s'", escapeDriveQ(f.MimeType)))
	}
	return strings.Join(parts, " and ")
}

func (g *GdriveStorage) SearchFiles(ctx context.Context, projectId string, f GdriveSearchFilters) ([]map[string]interface{}, error) {
	pageSize := f.MaxResults
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	params := map[string]string{
		"q":                         gdriveBuildSearchQuery(f),
		"fields":                    "files(id,name,mimeType,owners(emailAddress,displayName),parents,sharedWithMeTime,modifiedTime,size)",
		"pageSize":                  fmt.Sprint(pageSize),
		"spaces":                    "drive",
		"includeItemsFromAllDrives": "true",
		"supportsAllDrives":         "true",
	}
	res, _, status, err := g.driveCall(ctx, projectId, http.MethodGet, gdriveApiBase+"/files", http.Header{}, params, nil)
	if err != nil {
		return nil, fmt.Errorf("drive search failed (status %d): %w", status, err)
	}
	rm, ok := res.(map[string]interface{})
	if !ok {
		return nil, errors.New("drive search response not an object")
	}
	filesArr, _ := rm["files"].([]interface{})
	out := make([]map[string]interface{}, 0, len(filesArr))
	for _, fi := range filesArr {
		if fm, ok := fi.(map[string]interface{}); ok {
			out = append(out, fm)
		}
	}
	return out, nil
}

func gdriveDefaultExportMime(googleMime string) string {
	switch googleMime {
	case "application/vnd.google-apps.document":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "application/vnd.google-apps.spreadsheet":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "application/vnd.google-apps.presentation":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "application/vnd.google-apps.drawing":
		return "application/pdf"
	case "application/vnd.google-apps.script":
		return "application/vnd.google-apps.script+json"
	default:
		return "application/pdf"
	}
}

func gdriveResolveExportMime(googleMime, requested string) string {
	if requested == "" {
		return gdriveDefaultExportMime(googleMime)
	}
	switch strings.ToLower(requested) {
	case "pdf":
		return "application/pdf"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "odt":
		return "application/vnd.oasis.opendocument.text"
	case "ods":
		return "application/vnd.oasis.opendocument.spreadsheet"
	case "csv":
		return "text/csv"
	case "txt":
		return "text/plain"
	case "html":
		return "text/html"
	case "png":
		return "image/png"
	case "jpeg", "jpg":
		return "image/jpeg"
	case "svg":
		return "image/svg+xml"
	default:
		return requested
	}
}

func (g *GdriveStorage) downloadByDriveId(ctx context.Context, accessToken, fileId, exportMime string) (data []byte, mime string, name string, err error) {
	metaParams := map[string]string{
		"fields":            "id,name,mimeType,size,version,modifiedTime,headRevisionId",
		"supportsAllDrives": "true",
	}
	h := http.Header{}
	h.Set("Authorization", "Bearer "+accessToken)
	h.Set("Content-Type", "application/json")
	h.Set("Cache-Control", "no-cache, no-store, max-age=0")
	h.Set("Pragma", "no-cache")
	metaRes, _, _, mStatus, mErr := utils.CallHttp(ctx, http.MethodGet, fmt.Sprintf("%s/files/%s", gdriveApiBase, fileId), h, map[string]string{}, nil, metaParams, nil)
	if mErr != nil {
		return nil, "", "", fmt.Errorf("drive metadata failed (status %d): %w", mStatus, mErr)
	}
	srcMime := ""
	version := ""
	modifiedTime := ""
	revisionId := ""
	if mm, ok := metaRes.(map[string]interface{}); ok {
		name, _ = mm["name"].(string)
		srcMime, _ = mm["mimeType"].(string)
		version = fmt.Sprint(mm["version"])
		modifiedTime, _ = mm["modifiedTime"].(string)
		revisionId, _ = mm["headRevisionId"].(string)
	}
	cacheBuster := url.QueryEscape(strings.Join([]string{version, revisionId, modifiedTime}, "_"))
	logs.WithContext(ctx).Info(fmt.Sprintf("drive download %s : version %s revision %s modifiedTime %s", fileId, version, revisionId, modifiedTime))

	var dlUrl string
	if strings.HasPrefix(srcMime, "application/vnd.google-apps.") {
		targetMime := gdriveResolveExportMime(srcMime, exportMime)
		dlUrl = fmt.Sprintf("%s/files/%s/export?mimeType=%s&supportsAllDrives=true&eruv=%s", gdriveApiBase, fileId, url.QueryEscape(targetMime), cacheBuster)
		mime = targetMime
	} else {
		dlUrl = fmt.Sprintf("%s/files/%s?alt=media&supportsAllDrives=true&eruv=%s", gdriveApiBase, fileId, cacheBuster)
		mime = srcMime
	}

	req, rErr := http.NewRequestWithContext(ctx, http.MethodGet, dlUrl, nil)
	if rErr != nil {
		return nil, "", "", rErr
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Cache-Control", "no-cache, no-store, max-age=0")
	req.Header.Set("Pragma", "no-cache")
	resp, rErr := http.DefaultClient.Do(req)
	if rErr != nil {
		return nil, "", "", rErr
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, "", "", fmt.Errorf("drive download failed: status %d body %s", resp.StatusCode, string(body))
	}
	return body, mime, name, nil
}

func (g *GdriveStorage) DownloadById(ctx context.Context, projectId, fileId, exportMime string) (data []byte, mime string, name string, err error) {
	tok, err := g.getAccessToken(ctx, projectId)
	if err != nil {
		return nil, "", "", err
	}
	return g.downloadByDriveId(ctx, tok, fileId, exportMime)
}

func (g *GdriveStorage) BucketExists(ctx context.Context) (bool, error) {
	return g.RootFolderId != "", nil
}

func (g *GdriveStorage) CreateStorage(ctx context.Context, projectId string, cloneStorage StorageI, persist bool) error {
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
	id, err := g.ensureFolder(ctx, projectId, "root", name)
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

func (g *GdriveStorage) DeleteStorage(ctx context.Context, projectId string, forceDelete bool, cloneStorage StorageI) error {
	cloneG, ok := cloneStorage.(*GdriveStorage)
	if !ok {
		return errors.New("clone is not GdriveStorage")
	}
	if cloneG.RootFolderId == "" {
		logs.WithContext(ctx).Info("GDrive storage has no root_folder_id; nothing to delete")
		return nil
	}
	if forceDelete {
		if err := cloneStorage.EmptyBucket(ctx, projectId); err != nil {
			return err
		}
	}
	_, _, status, err := cloneG.driveCall(ctx, projectId, http.MethodDelete, fmt.Sprintf("%s/files/%s", gdriveApiBase, cloneG.RootFolderId), http.Header{}, map[string]string{"supportsAllDrives": "true"}, nil)
	if err != nil && status != http.StatusNotFound {
		return err
	}
	if status >= 300 && status != http.StatusNotFound {
		return fmt.Errorf("drive delete folder returned %d", status)
	}
	return nil
}

func (g *GdriveStorage) EmptyBucket(ctx context.Context, projectId string) error {
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
		res, _, status, err := g.driveCall(ctx, projectId, http.MethodGet, gdriveApiBase+"/files", http.Header{}, params, nil)
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
			_, _, dStatus, dErr := g.driveCall(ctx, projectId, http.MethodDelete, fmt.Sprintf("%s/files/%s", gdriveApiBase, id), http.Header{}, map[string]string{"supportsAllDrives": "true"}, nil)
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

func (g *GdriveStorage) StartPageToken(ctx context.Context, projectId string) (string, error) {
	tok, err := g.getAccessToken(ctx, projectId)
	if err != nil {
		return "", err
	}
	h := http.Header{}
	h.Set("Authorization", "Bearer "+tok)
	h.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, gdriveApiBase+"/changes/startPageToken", h, map[string]string{}, nil, map[string]string{"supportsAllDrives": "true"}, nil)
	if err != nil {
		return "", err
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		return "", errors.New("startPageToken response not an object")
	}
	out, _ := m["startPageToken"].(string)
	if out == "" {
		return "", errors.New("startPageToken response missing token")
	}
	return out, nil
}

func (g *GdriveStorage) WatchChanges(ctx context.Context, projectId, channelId, pushEndpoint string, expirationMs int64) (resourceId, startPageToken, expiration string, err error) {
	startPageToken, err = g.StartPageToken(ctx, projectId)
	if err != nil {
		return
	}
	body := map[string]interface{}{
		"id":      channelId,
		"type":    "web_hook",
		"address": pushEndpoint,
	}
	if expirationMs > 0 {
		body["expiration"] = fmt.Sprint(expirationMs)
	}
	watchUrl := fmt.Sprintf("%s/changes/watch?pageToken=%s&supportsAllDrives=true", gdriveApiBase, url.QueryEscape(startPageToken))
	res, _, status, cErr := g.driveCall(ctx, projectId, http.MethodPost, watchUrl, http.Header{}, map[string]string{}, body)
	if cErr != nil {
		err = fmt.Errorf("drive changes.watch failed (status %d): %w", status, cErr)
		return
	}
	m, _ := res.(map[string]interface{})
	resourceId, _ = m["resourceId"].(string)
	expiration, _ = m["expiration"].(string)
	return
}

func (g *GdriveStorage) WatchFile(ctx context.Context, projectId, fileId, channelId, pushEndpoint string, expirationMs int64) (resourceId, expiration string, err error) {
	if fileId == "" {
		err = errors.New("file_id is required")
		return
	}
	body := map[string]interface{}{
		"id":      channelId,
		"type":    "web_hook",
		"address": pushEndpoint,
	}
	if expirationMs > 0 {
		body["expiration"] = fmt.Sprint(expirationMs)
	}
	watchUrl := fmt.Sprintf("%s/files/%s/watch?supportsAllDrives=true", gdriveApiBase, fileId)
	res, _, status, cErr := g.driveCall(ctx, projectId, http.MethodPost, watchUrl, http.Header{}, map[string]string{}, body)
	if cErr != nil {
		err = fmt.Errorf("drive files.watch failed (status %d): %w", status, cErr)
		return
	}
	m, _ := res.(map[string]interface{})
	resourceId, _ = m["resourceId"].(string)
	expiration, _ = m["expiration"].(string)
	return
}

func (g *GdriveStorage) StopWatch(ctx context.Context, projectId, channelId, resourceId string) error {
	if channelId == "" || resourceId == "" {
		return errors.New("channel_id and resource_id are required")
	}
	body := map[string]interface{}{"id": channelId, "resourceId": resourceId}
	_, _, status, err := g.driveCall(ctx, projectId, http.MethodPost, gdriveApiBase+"/channels/stop", http.Header{}, map[string]string{}, body)
	if err != nil && status != http.StatusNotFound {
		return fmt.Errorf("drive channels.stop failed (status %d): %w", status, err)
	}
	return nil
}

func (g *GdriveStorage) ListChanges(ctx context.Context, projectId, pageToken string) (changes []map[string]interface{}, newStartPageToken, nextPageToken string, err error) {
	if pageToken == "" {
		err = errors.New("page_token is required")
		return
	}
	params := map[string]string{
		"pageToken":                 pageToken,
		"includeRemoved":            "true",
		"includeItemsFromAllDrives": "true",
		"supportsAllDrives":         "true",
		"fields":                    "newStartPageToken,nextPageToken,changes(fileId,removed,file(id,name,parents,mimeType,modifiedTime))",
	}
	res, _, status, cErr := g.driveCall(ctx, projectId, http.MethodGet, gdriveApiBase+"/changes", http.Header{}, params, nil)
	if cErr != nil {
		err = fmt.Errorf("drive changes list failed (status %d): %w", status, cErr)
		return
	}
	m, _ := res.(map[string]interface{})
	if arr, ok := m["changes"].([]interface{}); ok {
		changes = make([]map[string]interface{}, 0, len(arr))
		for _, c := range arr {
			if cm, ok := c.(map[string]interface{}); ok {
				changes = append(changes, cm)
			}
		}
	}
	newStartPageToken, _ = m["newStartPageToken"].(string)
	nextPageToken, _ = m["nextPageToken"].(string)
	return
}
