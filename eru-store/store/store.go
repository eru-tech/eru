package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	repos "github.com/eru-tech/eru/eru-repos/repos"
	sm "github.com/eru-tech/eru/eru-secret-manager/sm"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jmoiron/sqlx"
	"os"
	"strings"
	"sync"
	"time"
)

type StoreI interface {
	LoadStore(fp string, ms StoreI) (err error)
	GetStoreByteArray(fp string) (b []byte, err error)
	SaveStore(ctx context.Context, projectId string, fp string, ms StoreI) (err error)
	SetDbType(dbtype string)
	CreateConn() error
	GetConn() *sqlx.DB
	GetDbType() string
	ExecuteDbSave(ctx context.Context, queries []Queries) (output [][]map[string]interface{}, err error)
	ExecuteDbFetch(ctx context.Context, query Queries) (output []map[string]interface{}, err error)
	SetStoreTableName(tablename string)
	GetStoreTableName() (tablename string)
	SetVars(ctx context.Context, variables map[string]Variables)
	SaveVar(ctx context.Context, projectId string, newVar Vars, s StoreI) (err error)
	RemoveVar(ctx context.Context, projectId string, key string, s StoreI) (err error)
	SaveEnvVar(ctx context.Context, projectId string, newEnvVar EnvVars, s StoreI) (err error)
	RemoveEnvVar(ctx context.Context, projectId string, key string, s StoreI) (err error)
	SaveSecret(ctx context.Context, projectId string, newSecret Secrets, s StoreI) (err error)
	RemoveSecret(ctx context.Context, projectId string, key string, s StoreI) (err error)
	FetchVars(ctx context.Context, projectId string) (variables Variables, err error)
	ReplaceVariables(ctx context.Context, projectId string, text []byte) (returnText []byte)
	SaveRepo(ctx context.Context, projectId string, repo repos.RepoI, s StoreI, persist bool) (err error)
	SaveRepoToken(ctx context.Context, projectId string, repo repos.RepoToken, s StoreI) (err error)
	FetchRepo(ctx context.Context, projectId string) (repo repos.RepoI, err error)
	CommitRepo(ctx context.Context, projectId string, ms StoreI) (err error)
	GetProjectConfigForRepo(ctx context.Context, projectId string, ms StoreI) (repoData map[string]map[string]interface{}, accessToken string, err error)
	SaveSm(ctx context.Context, projectId string, secretManager sm.SmStoreI, s StoreI, persist bool) (err error)
	FetchSm(ctx context.Context, projectId string) (sm sm.SmStoreI, err error)
	LoadSmValue(ctx context.Context, projectId string) (err error)
	LoadEnvValue(ctx context.Context, projectId string) (err error)
	SetStoreFromBytes(ctx context.Context, storeBytes []byte, msi StoreI) (err error)
	GetMutex() *sync.RWMutex
	//SaveProject(projectId string, realStore StoreI) error
	//RemoveProject(projectId string, realStore StoreI) error
	//GetProjectConfig(projectId string) (*model.ProjectI, error)
	//GetProjectList() []map[string]interface{}
}

type Store struct {
	//Projects map[string]*model.Project //ProjectId is the key
	mu                sync.RWMutex
	Variables         map[string]Variables       `json:"variables"`
	ProjectRepos      map[string]repos.RepoI     `json:"repos"`
	ProjectRepoTokens map[string]repos.RepoToken `json:"repo_token"`
	SecretManager     map[string]sm.SmStoreI     `json:"secret_manager"`
}

type StoreCompare struct {
	DeleteVariables       []string               `json:"delete_variables"`
	NewVariables          []string               `json:"new_variables"`
	DeleteEnvVariables    []string               `json:"delete_env_variables"`
	NewEnvVariables       []string               `json:"new_env_variables"`
	DeleteSecrets         []string               `json:"delete_secrets"`
	NewSecrets            []string               `json:"new_secrets"`
	MismatchSettings      map[string]interface{} `json:"mismatch_settings"`
	MismatchSecretManager map[string]interface{} `json:"mismatch_secret_manager"`
}

func (storeCompare *StoreCompare) CompareSecretManager(ctx context.Context, orgSm sm.SmStoreI, compareSm sm.SmStoreI) {
	var diffR utils.DiffReporter
	if !cmp.Equal(orgSm, compareSm, cmpopts.IgnoreUnexported(sm.AwsSmStore{}), cmp.Reporter(&diffR)) {
		if storeCompare.MismatchSecretManager == nil {
			storeCompare.MismatchSecretManager = make(map[string]interface{})
		}
		storeCompare.MismatchSecretManager["sm"] = diffR.Output()
	}
}

func (storeCompare *StoreCompare) CompareVariables(ctx context.Context, orgVars Variables, compareVars Variables) {
	//variables
	for k, _ := range orgVars.Vars {
		varFound := false
		for ck, _ := range compareVars.Vars {
			if k == ck {
				varFound = true
				break
			}
		}
		if !varFound {
			storeCompare.DeleteVariables = append(storeCompare.DeleteVariables, k)
		}
	}

	for ck, _ := range compareVars.Vars {
		varFound := false
		for k, _ := range orgVars.Vars {
			if k == ck {
				varFound = true
				break
			}
		}
		if !varFound {
			storeCompare.NewVariables = append(storeCompare.NewVariables, ck)
		}
	}

	// env variables
	for k, _ := range orgVars.EnvVars {
		varFound := false
		for ck, _ := range compareVars.EnvVars {
			if k == ck {
				varFound = true
				break
			}
		}
		if !varFound {
			storeCompare.DeleteEnvVariables = append(storeCompare.DeleteEnvVariables, k)
		}
	}

	for ck, _ := range compareVars.EnvVars {
		varFound := false
		for k, _ := range orgVars.EnvVars {
			if k == ck {
				varFound = true
				break
			}
		}
		if !varFound {
			storeCompare.NewEnvVariables = append(storeCompare.NewEnvVariables, ck)
		}
	}

	// secrets
	for k, _ := range orgVars.Secrets {
		varFound := false
		for ck, _ := range compareVars.Secrets {
			if k == ck {
				varFound = true
				break
			}
		}
		if !varFound {
			storeCompare.DeleteSecrets = append(storeCompare.DeleteSecrets, k)
		}
	}

	for ck, _ := range compareVars.Secrets {
		varFound := false
		for k, _ := range orgVars.Secrets {
			if k == ck {
				varFound = true
				break
			}
		}
		if !varFound {
			storeCompare.NewSecrets = append(storeCompare.NewSecrets, ck)
		}
	}
	return
}

func (store *Store) GetMutex() *sync.RWMutex {
	return &store.mu
}

func (store *Store) LoadStore(fp string, ms StoreI) (err error) {
	err = errors.New("method not implemented")
	logs.WithContext(context.Background()).Error(err.Error())
	return
}

func (store *Store) GetStoreByteArray(fp string) (b []byte, err error) {
	err = errors.New("method not implemented")
	logs.WithContext(context.Background()).Error(err.Error())
	return
}

func (store *Store) SaveStore(ctx context.Context, fp string, ms StoreI) (err error) {
	err = errors.New("method not implemented")
	logs.WithContext(context.Background()).Error(err.Error())
	return
}

type Variables struct {
	Vars    map[string]Vars    `json:"vars"`
	EnvVars map[string]EnvVars `json:"env_vars"`
	Secrets map[string]Secrets `json:"secrets"`
}

type Vars struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type EnvVars struct {
	Key   string `json:"key"`
	Value string `json:"-"`
}

type Secrets struct {
	Key   string `json:"key"`
	Value string `json:"-"`
}

func (store *Store) GetDbType() string {
	return ""
}

func (store *Store) SetVars(ctx context.Context, variables map[string]Variables) {
	store.GetMutex().Lock()
	defer store.GetMutex().Unlock()
	store.Variables = variables
}

func (store *Store) FetchVars(ctx context.Context, projectId string) (variables Variables, err error) {
	logs.WithContext(ctx).Debug("FetchVars - Start")
	if store.Variables == nil {
		err = errors.New("No variables defined in store")
		logs.WithContext(ctx).Error(err.Error())
		return Variables{}, err
	}
	ok := false
	if variables, ok = store.Variables[projectId]; !ok {
		err = errors.New(fmt.Sprint("Variables not defined for project :", projectId))
		logs.WithContext(ctx).Error(err.Error())
		return Variables{}, err
	}
	return
}

func (store *Store) SaveVar(ctx context.Context, projectId string, newVar Vars, s StoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveVar - Start")
	s.GetMutex().Lock()
	defer s.GetMutex().Unlock()
	if store.Variables == nil {
		store.Variables = make(map[string]Variables)
	}
	var v Variables
	ok := false
	if v, ok = store.Variables[projectId]; !ok {
		logs.WithContext(ctx).Info(fmt.Sprint("making new variable object for project : ", projectId))
		store.Variables[projectId] = Variables{}
		v = store.Variables[projectId]
	}
	if v.Vars == nil {
		v.Vars = make(map[string]Vars)
	}
	v.Vars[newVar.Key] = newVar
	store.Variables[projectId] = v
	err = s.SaveStore(ctx, projectId, "", s)
	return
}

func (store *Store) RemoveVar(ctx context.Context, projectId string, key string, s StoreI) (err error) {
	logs.WithContext(ctx).Debug("RemoveVar - Start")
	s.GetMutex().Lock()
	defer s.GetMutex().Unlock()
	if store.Variables == nil {
		err = errors.New("No variables defined in store")
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	if _, ok := store.Variables[projectId]; !ok {
		err = errors.New(fmt.Sprint("Variables not defined for project :", projectId))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	if _, ok := store.Variables[projectId].Vars[key]; !ok {
		err = errors.New(fmt.Sprint("Variable key not defined :", key))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	delete(store.Variables[projectId].Vars, key)
	err = s.SaveStore(ctx, projectId, "", s)
	return
}

func (store *Store) SaveEnvVar(ctx context.Context, projectId string, newEnvVar EnvVars, s StoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveEnvVar - Start")
	s.GetMutex().Lock()
	defer s.GetMutex().Unlock()
	if store.Variables == nil {
		store.Variables = make(map[string]Variables)
	}
	var v Variables
	ok := false
	if v, ok = store.Variables[projectId]; !ok {
		logs.WithContext(ctx).Info(fmt.Sprint("making new variable object for project : ", projectId))
		store.Variables[projectId] = Variables{}
		v = store.Variables[projectId]
	}
	if v.EnvVars == nil {
		v.EnvVars = make(map[string]EnvVars)
	}
	v.EnvVars[newEnvVar.Key] = newEnvVar
	store.Variables[projectId] = v
	err = s.SaveStore(ctx, projectId, "", s)
	return
}

func (store *Store) RemoveEnvVar(ctx context.Context, projectId string, key string, s StoreI) (err error) {
	logs.WithContext(ctx).Debug("RemoveEnvVar - Start")
	s.GetMutex().Lock()
	defer s.GetMutex().Unlock()
	if store.Variables == nil {
		err = errors.New("No variables defined in store")
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	if _, ok := store.Variables[projectId]; !ok {
		err = errors.New(fmt.Sprint("Variables not defined for project :", projectId))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	if _, ok := store.Variables[projectId].EnvVars[key]; !ok {
		err = errors.New(fmt.Sprint("Env. Variable key not defined :", key))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	delete(store.Variables[projectId].EnvVars, key)
	err = s.SaveStore(ctx, projectId, "", s)
	return
}

func (store *Store) SaveSecret(ctx context.Context, projectId string, newSecret Secrets, s StoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveSecret - Start")
	s.GetMutex().Lock()
	defer s.GetMutex().Unlock()
	if store.Variables == nil {
		store.Variables = make(map[string]Variables)
	}
	var v Variables
	ok := false
	if v, ok = store.Variables[projectId]; !ok {
		logs.WithContext(ctx).Info(fmt.Sprint("making new variable object for project : ", projectId))
		store.Variables[projectId] = Variables{}
		v = store.Variables[projectId]
	}
	if v.Secrets == nil {
		v.Secrets = make(map[string]Secrets)
	}
	v.Secrets[newSecret.Key] = newSecret
	store.Variables[projectId] = v
	err = s.SaveStore(ctx, projectId, "", s)
	return
}

func (store *Store) RemoveSecret(ctx context.Context, projectId string, key string, s StoreI) (err error) {
	logs.WithContext(ctx).Debug("RemoveSecret - Start")
	s.GetMutex().Lock()
	defer s.GetMutex().Unlock()
	if store.Variables == nil {
		err = errors.New("No variables defined in store")
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	if _, ok := store.Variables[projectId]; !ok {
		err = errors.New(fmt.Sprint("Variables not defined for project :", projectId))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	if _, ok := store.Variables[projectId].Secrets[key]; !ok {
		err = errors.New(fmt.Sprint("Secret key not defined :", key))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	delete(store.Variables[projectId].Secrets, key)
	err = s.SaveStore(ctx, projectId, "", s)
	return
}

func (store *Store) SetDbType(dbtype string) {
	//do nothing
}

func (store *Store) SetStoreTableName(tablename string) {
	//do nothing
}

func (store *Store) GetStoreTableName() (tablename string) {
	return
}

func (store *Store) CreateConn() error {
	logs.Logger.Info("CreateConn not implemented")
	return nil
}

func (store *Store) GetConn() *sqlx.DB {
	logs.Logger.Info("GetConn not implemented")
	return nil
}

func (store *Store) ExecuteDbFetch(ctx context.Context, query Queries) (output []map[string]interface{}, err error) {
	logs.Logger.Info("ExecuteDbFetch not implemented")
	return
}

func (store *Store) ExecuteDbSave(ctx context.Context, queries []Queries) (output [][]map[string]interface{}, err error) {
	logs.Logger.Info("ExecuteDbFetch not implemented")
	return
}

func (store *Store) ReplaceVariables(ctx context.Context, projectId string, text []byte) (returnText []byte) {
	logs.WithContext(ctx).Debug("ReplaceVariables - Start")
	textStr := string(text)
	if _, prjVarsOk := store.Variables[projectId]; prjVarsOk {
		for k, v := range store.Variables[projectId].Vars {
			logs.WithContext(ctx).Info(fmt.Sprint(k, " : ", v))
			textStr = strings.Replace(textStr, fmt.Sprint("$VAR_", k), v.Value, -1)
		}
		for k, v := range store.Variables[projectId].EnvVars {
			logs.WithContext(ctx).Info(fmt.Sprint(k, " : ", v))
			textStr = strings.Replace(textStr, fmt.Sprint("$ENV_", k), v.Value, -1)
		}
		for k, v := range store.Variables[projectId].Secrets {
			logs.WithContext(ctx).Info(fmt.Sprint(k, " : ", v))
			textStr = strings.Replace(textStr, fmt.Sprint("$SECRET_", k), v.Value, -1)
		}
	}
	return []byte(textStr)
}

func (store *Store) FetchRepo(ctx context.Context, projectId string) (repo repos.RepoI, err error) {
	logs.WithContext(ctx).Debug("FetchRepo - Start")
	if store.ProjectRepos == nil {
		err = errors.New("no repo defined in store")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	ok := false
	if repo, ok = store.ProjectRepos[projectId]; !ok {
		err = errors.New(fmt.Sprint("Repo not defined for project :", projectId))
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	return
}

func (store *Store) CommitRepo(ctx context.Context, projectId string, s StoreI) (err error) {
	logs.WithContext(ctx).Debug("CommitRepo - Start")
	repo, err := store.FetchRepo(ctx, projectId)
	if err != nil {
		return
	}

	repoConfigBytes, err := json.Marshal(repo)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	repoConfigBytes = s.ReplaceVariables(ctx, projectId, repoConfigBytes)

	cloneRepo := repos.GetRepo(repo.GetAttribute("repo_type").(string))

	err = json.Unmarshal(repoConfigBytes, &cloneRepo)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	repoData, token, err := store.GetProjectConfigForRepo(ctx, projectId, s)
	if err != nil {
		return
	}
	_ = token
	repoBytes, err := json.MarshalIndent(repoData, "", " ")
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	if token != "" {
		cloneRepo.SetAuthKey(ctx, token)
	}
	err = cloneRepo.Commit(ctx, repoBytes, fmt.Sprint(s.GetStoreTableName(), ".json"))
	if err != nil {
		return
	}
	repo.SetLastCommitAt()
	return
}

func (store *Store) SaveRepo(ctx context.Context, projectId string, repo repos.RepoI, s StoreI, persist bool) (err error) {
	logs.WithContext(ctx).Debug("SaveRepo - Start")
	if persist {
		s.GetMutex().Lock()
		defer s.GetMutex().Unlock()
	}
	if store.ProjectRepos == nil {
		store.ProjectRepos = make(map[string]repos.RepoI)
	}
	store.ProjectRepos[projectId] = repo
	if persist {
		err = s.SaveStore(ctx, projectId, "", s)
	}
	return
}

func (store *Store) SaveRepoToken(ctx context.Context, projectId string, repoToken repos.RepoToken, s StoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveRepoToken - Start")
	s.GetMutex().Lock()
	defer s.GetMutex().Unlock()
	if store.ProjectRepoTokens == nil {
		store.ProjectRepoTokens = make(map[string]repos.RepoToken)
	}
	var prjRepoToken repos.RepoToken
	_ = prjRepoToken
	ok := false
	if prjRepoToken, ok = store.ProjectRepoTokens[projectId]; !ok {
		logs.WithContext(ctx).Info(fmt.Sprint("making new repo token object for project : ", projectId))
		store.ProjectRepoTokens[projectId] = repos.RepoToken{}
	}
	store.ProjectRepoTokens[projectId] = repoToken
	err = s.SaveStore(ctx, projectId, "", s)
	return
}

func (store *Store) FetchSm(ctx context.Context, projectId string) (smObj sm.SmStoreI, err error) {
	logs.WithContext(ctx).Debug("FetchSm - Start")
	if store.SecretManager == nil {
		err = errors.New("No secret manager defined in store")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	ok := false
	if smObj, ok = store.SecretManager[projectId]; !ok {
		err = errors.New(fmt.Sprint("Secret Manager not defined for project :", projectId))
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	return
}

func (store *Store) LoadEnvValue(ctx context.Context, projectId string) (err error) {
	logs.WithContext(ctx).Info("LoadEnvValue - Start")
	if store.Variables != nil {
		for prjId, _ := range store.Variables {
			if projectId == prjId || projectId == "" {
				if _, prjVarsOk := store.Variables[prjId]; prjVarsOk {
					if store.Variables[prjId].EnvVars != nil {
						for k, v := range store.Variables[prjId].EnvVars {
							envValue := os.Getenv(k)
							if envValue != "" {
								v.Value = envValue
								store.Variables[prjId].EnvVars[k] = v
							} else {
								logs.WithContext(ctx).Warn(fmt.Sprint("no environment value found for ", k))
							}
						}
					} else {
						err = errors.New(fmt.Sprint("environment variables not defined for project : ", prjId))
						return
					}
				}
			}
		}
	}
	return
}
func (store *Store) LoadSmValue(ctx context.Context, projectId string) (err error) {
	logs.WithContext(ctx).Info("LoadSmValue - Start")
	smFound := true

	if store.Variables != nil {
		for prjId, _ := range store.Variables {
			if projectId == prjId || projectId == "" {
				logs.WithContext(ctx).Info(fmt.Sprint("loading secrets for :", prjId))
				if _, prjVarsOk := store.Variables[prjId]; prjVarsOk {
					if store.Variables[prjId].Secrets != nil {
						if store.SecretManager == nil {
							err = errors.New("No secret manager defined in store")
							smFound = false
							logs.WithContext(ctx).Error(err.Error())
						} else if smObj, smObjOk := store.SecretManager[prjId]; !smObjOk {
							err = errors.New(fmt.Sprint("Secret Manager not defined for project :", prjId))
							smFound = false
							logs.WithContext(ctx).Error(err.Error())
						} else {
							if smObj != nil {
								result, resultErr := smObj.FetchSmValue(ctx)
								if resultErr != nil {
									smFound = false
								}

								if smFound {
									for k, v := range store.Variables[prjId].Secrets {
										if _, seretOk := result[k]; seretOk {
											v.Value = result[k]
											store.Variables[prjId].Secrets[k] = v
											logs.WithContext(ctx).Info(fmt.Sprint(v.Key, " : ", v.Value))
										} else {
											logs.WithContext(ctx).Warn(fmt.Sprint("secret manager does not have any secret value for ", k, ", trying to load from environment variables"))
											v.Value = os.Getenv(k)
											store.Variables[prjId].Secrets[k] = v
										}
									}
								}
							}
						}
					} else {
						err = errors.New(fmt.Sprint("secret not defined for project : ", prjId))
						smFound = false
						logs.WithContext(ctx).Error(err.Error())
					}
				} else {
					err = errors.New(fmt.Sprint("variables not defined for project : ", prjId))
					smFound = false
					logs.WithContext(ctx).Error(err.Error())
				}
				if !smFound {
					logs.WithContext(ctx).Warn(fmt.Sprint("no secret manager found, trying to load from environment variables"))
					for k, v := range store.Variables[prjId].Secrets {
						v.Value = os.Getenv(k)
						logs.WithContext(ctx).Info(fmt.Sprint(v))
						store.Variables[prjId].Secrets[k] = v
					}
				}
			}
		}
	}
	logs.WithContext(ctx).Info(fmt.Sprint(store.Variables))
	return
}

func (store *Store) SaveSm(ctx context.Context, projectId string, secretManager sm.SmStoreI, s StoreI, persist bool) (err error) {
	logs.WithContext(ctx).Debug("SaveSm - Start")
	if persist {
		s.GetMutex().Lock()
		defer s.GetMutex().Unlock()
	}
	if store.SecretManager == nil {
		store.SecretManager = make(map[string]sm.SmStoreI)
	}
	store.SecretManager[projectId] = secretManager
	if persist {
		err = s.SaveStore(ctx, projectId, "", s)
	}
	return
}

func (store *Store) GetProjectConfigForRepo(ctx context.Context, projectId string, ms StoreI) (repoData map[string]map[string]interface{}, accessToken string, err error) {
	logs.WithContext(ctx).Debug("GetProjectConfigForRepo - Start")
	repoData = make(map[string]map[string]interface{})
	repoInnerData := make(map[string]interface{})
	repoData[projectId] = repoInnerData
	storeBytes, err := json.Marshal(ms)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, "", err
	}
	storeMap := make(map[string]interface{})
	err = json.Unmarshal(storeBytes, &storeMap)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, "", err
	}
	td := 0.0
	authMode := ""
	at := ""
	for k, v := range storeMap {
		if k == "projects" {
			if prjMap, prjMapOk := v.(map[string]interface{}); prjMapOk {
				if prj, ok := prjMap[projectId]; ok {
					repoInnerData["config"] = prj
				} else {
					return nil, "", errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
				}
			} else {
				logs.WithContext(ctx).Info("map failed")
			}
		} else if k == "variables" {
			if VarsMap, VarsMapOk := v.(map[string]interface{}); VarsMapOk {
				if vars, ok := VarsMap[projectId]; ok {
					repoInnerData["variables"] = vars
				}
			}
		} else if k == "repos" {
			if ReposMap, ReposMapOk := v.(map[string]interface{}); ReposMapOk {
				if repo, ok := ReposMap[projectId]; ok {
					repoBytes, repoBytesErr := json.Marshal(repo)
					if repoBytesErr != nil {
						return nil, "", repoBytesErr
					}
					repoStruct := repos.Repo{}
					repoMapErr := json.Unmarshal(repoBytes, &repoStruct)
					if repoMapErr != nil {
						return nil, "", repoMapErr
					}
					repoInnerData["repo"] = repoStruct
					authMode = repoStruct.AuthMode
				}
			}
		} else if k == "repo_token" {
			if RepoTokenMap, RepoTokenMapOk := v.(map[string]interface{}); RepoTokenMapOk {
				if repoToken, ok := RepoTokenMap[projectId]; ok {
					repoTokenBytes, repoTokenBytesErr := json.Marshal(repoToken)
					if repoTokenBytesErr != nil {
						return nil, "", repoTokenBytesErr
					}
					repoTokenStruct := repos.RepoToken{}
					repoMapErr := json.Unmarshal(repoTokenBytes, &repoTokenStruct)
					if repoMapErr != nil {
						return nil, "", repoMapErr
					}

					t, e := time.Parse("2006-01-02T15:04:05Z", repoTokenStruct.RepoTokenExpiry)
					if e != nil {
						logs.WithContext(ctx).Error(e.Error())
						return nil, "", e
					}
					td = t.Sub(time.Now()).Seconds()
					at = repoTokenStruct.RepoToken
				}
			}
		}
	}
	if authMode == "GITHUBAPP" {
		if td > 0 {
			accessToken = at
		} else {
			err = errors.New("token expired")
			logs.WithContext(ctx).Error(err.Error())
			return
		}
	}
	return
}

func (store *Store) SetStoreFromBytes(ctx context.Context, storeBytes []byte, msi StoreI) (err error) {
	logs.WithContext(ctx).Debug("SetStoreFromBytes - Start")
	msi.GetMutex().Lock()
	defer msi.GetMutex().Unlock()

	var storeMap map[string]*json.RawMessage
	err = json.Unmarshal(storeBytes, &storeMap)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	var prjSm map[string]*json.RawMessage
	if _, ok := storeMap["secret_manager"]; ok {
		if storeMap["secret_manager"] != nil {
			err = json.Unmarshal(*storeMap["secret_manager"], &prjSm)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			for prj, smJson := range prjSm {
				if smJson != nil {
					var smObj map[string]*json.RawMessage
					err = json.Unmarshal(*smJson, &smObj)
					if err != nil {
						logs.WithContext(ctx).Error(err.Error())
						return err
					}
					var smType string
					if _, stOk := smObj["sm_store_type"]; stOk {
						err = json.Unmarshal(*smObj["sm_store_type"], &smType)
						if err != nil {
							logs.WithContext(ctx).Error(err.Error())
							return err
						}
						smI := sm.GetSm(smType)
						if smI != nil {
							err = smI.MakeFromJson(ctx, smJson)
							if err == nil {
								err = msi.SaveSm(ctx, prj, smI, msi, false)
								if err != nil {
									return err
								}
							} else {
								return err
							}
						}
					} else {
						logs.WithContext(ctx).Info("ignoring secret manager as sm_store_type attribute not found")
					}
				}
			}
		} else {
			logs.WithContext(ctx).Info("secret manager attribute is nil")
		}
	} else {
		logs.WithContext(ctx).Info("secret manager attribute not found in store")
	}

	var prjRepo map[string]*json.RawMessage
	if _, ok := storeMap["repos"]; ok {
		if storeMap["repos"] != nil {
			err = json.Unmarshal(*storeMap["repos"], &prjRepo)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			for prj, repoJson := range prjRepo {
				var repoObj map[string]*json.RawMessage
				err = json.Unmarshal(*repoJson, &repoObj)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				var repoType string
				if _, rtOk := repoObj["repo_type"]; rtOk {
					err = json.Unmarshal(*repoObj["repo_type"], &repoType)
					if err != nil {
						logs.WithContext(ctx).Error(err.Error())
						return err
					}
					repoI := repos.GetRepo(repoType)
					err = repoI.MakeFromJson(ctx, repoJson)
					if err == nil {
						err = msi.SaveRepo(ctx, prj, repoI, msi, false)
						if err != nil {
							return err
						}
					} else {
						return err
					}
				} else {
					logs.WithContext(ctx).Info("ignoring repo as repo type not found")
				}
			}
		} else {
			logs.WithContext(ctx).Info("repos attribute is nil")
		}
	} else {
		logs.WithContext(ctx).Info("repos attribute not found in store")
	}
	logs.WithContext(ctx).Error("SetStoreFromBytes before return")
	return
}

/*
func (store *Store) SaveProject(projectId string, realStore StoreI) error {
	//TODO to handle edit project once new project attributes are finalized
	if _, ok := store.Projects[projectId]; !ok {
		project := new(model.Project)
		project.ProjectId = projectId
		if store.Projects == nil {
			store.Projects = make(map[string]*model.Project)
		}
		store.Projects[projectId] = project
		return realStore.SaveStore("")
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " already exists"))
	}
}

func (store *Store) RemoveProject(projectId string, realStore StoreI) error {
	if _, ok := store.Projects[projectId]; ok {
		delete(store.Projects, projectId)
		return realStore.SaveStore("")
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}

func (store *Store) GetProjectConfig(projectId string) (*model.ProjectI, error) {
	if _, ok := store.Projects[projectId]; ok {
		var p model.ProjectI
		p = store.Projects[projectId]
		return &p, nil
	} else {
		return nil, errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}

func (store *Store) GetProjectList() []map[string]interface{} {
	projects := make([]map[string]interface{}, len(store.Projects))
	i := 0
	for k := range store.Projects {
		project := make(map[string]interface{})
		project["projectName"] = k
		//project["lastUpdateDate"] = time.Now()
		projects[i] = project
		i++
	}
	return projects
}
*/
