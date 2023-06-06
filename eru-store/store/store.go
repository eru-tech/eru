package store

import (
	"context"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"strings"
)

type StoreI interface {
	LoadStore(fp string, ms StoreI) (err error)
	GetStoreByteArray(fp string) (b []byte, err error)
	SaveStore(ctx context.Context, fp string, ms StoreI) (err error)
	SetDbType(dbtype string)
	SetStoreTableName(tablename string)
	SetVars(ctx context.Context, variables map[string]*Variables)
	SaveVar(ctx context.Context, projectId string, newVar Vars, s StoreI) (err error)
	RemoveVar(ctx context.Context, projectId string, key string, s StoreI) (err error)
	SaveEnvVar(ctx context.Context, projectId string, newEnvVar EnvVars, s StoreI) (err error)
	RemoveEnvVar(ctx context.Context, projectId string, key string, s StoreI) (err error)
	SaveSecret(ctx context.Context, projectId string, newSecret Secrets, s StoreI) (err error)
	RemoveSecret(ctx context.Context, projectId string, key string, s StoreI) (err error)
	FetchVars(ctx context.Context, projectId string) (variables *Variables, err error)
	ReplaceVariables(ctx context.Context, projectId string, text []byte) (returnText []byte)
	//SaveProject(projectId string, realStore StoreI) error
	//RemoveProject(projectId string, realStore StoreI) error
	//GetProjectConfig(projectId string) (*model.ProjectI, error)
	//GetProjectList() []map[string]interface{}
}

type Store struct {
	//Projects map[string]*model.Project //ProjectId is the key
	Variables map[string]*Variables
}

type Variables struct {
	Vars    map[string]*Vars
	EnvVars map[string]*EnvVars
	Secrets map[string]*Secrets
}

type Vars struct {
	Key   string
	Value string
}

type EnvVars struct {
	Key   string
	Value string `json:"-"`
}

type Secrets struct {
	Key   string
	Value string `json:"-"`
}

func (store *Store) SetVars(ctx context.Context, variables map[string]*Variables) {
	store.Variables = variables
}

func (store *Store) FetchVars(ctx context.Context, projectId string) (variables *Variables, err error) {
	logs.WithContext(ctx).Debug("FetchVars - Start")
	if store.Variables == nil {
		err = errors.New("No variables defined in store")
		logs.WithContext(ctx).Error(err.Error())
		return &Variables{}, err
	}
	ok := false
	if variables, ok = store.Variables[projectId]; !ok {
		err = errors.New(fmt.Sprint("Variables not defined for project :", projectId))
		logs.WithContext(ctx).Error(err.Error())
		return &Variables{}, err
	}
	return
}

func (store *Store) SaveVar(ctx context.Context, projectId string, newVar Vars, s StoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveVar - Start")
	if store.Variables == nil {
		store.Variables = make(map[string]*Variables)
	}
	var variables *Variables
	ok := false
	if variables, ok = store.Variables[projectId]; !ok {
		logs.WithContext(ctx).Info(fmt.Sprint("making new variable object for project : ", projectId))
		store.Variables[projectId] = &Variables{}
		variables = store.Variables[projectId]
	}
	if variables.Vars == nil {
		variables.Vars = make(map[string]*Vars)
	}
	variables.Vars[newVar.Key] = &newVar
	err = s.SaveStore(ctx, "", s)
	return
}

func (store *Store) RemoveVar(ctx context.Context, projectId string, key string, s StoreI) (err error) {
	logs.WithContext(ctx).Debug("RemoveVar - Start")
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
	err = s.SaveStore(ctx, "", s)
	return
}

func (store *Store) SaveEnvVar(ctx context.Context, projectId string, newEnvVar EnvVars, s StoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveEnvVar - Start")
	if store.Variables == nil {
		store.Variables = make(map[string]*Variables)
	}
	if variables, ok := store.Variables[projectId]; !ok {
		logs.WithContext(ctx).Info(fmt.Sprint("making new variable object for project : ", projectId))
		store.Variables[projectId] = &Variables{}
	} else {
		if variables.EnvVars == nil {
			variables.EnvVars = make(map[string]*EnvVars)
		}
		variables.EnvVars[newEnvVar.Key] = &newEnvVar
	}
	err = s.SaveStore(ctx, "", s)
	return
}

func (store *Store) RemoveEnvVar(ctx context.Context, projectId string, key string, s StoreI) (err error) {
	logs.WithContext(ctx).Debug("RemoveEnvVar - Start")
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
	err = s.SaveStore(ctx, "", s)
	return
}

func (store *Store) SaveSecret(ctx context.Context, projectId string, newSecret Secrets, s StoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveSecret - Start")
	if store.Variables == nil {
		store.Variables = make(map[string]*Variables)
	}
	if variables, ok := store.Variables[projectId]; !ok {
		logs.WithContext(ctx).Info(fmt.Sprint("making new variable object for project : ", projectId))
		store.Variables[projectId] = &Variables{}
	} else {
		if variables.Secrets == nil {
			variables.Secrets = make(map[string]*Secrets)
		}
		variables.Secrets[newSecret.Key] = &newSecret
	}
	err = s.SaveStore(ctx, "", s)
	return
}

func (store *Store) RemoveSecret(ctx context.Context, projectId string, key string, s StoreI) (err error) {
	logs.WithContext(ctx).Debug("RemoveSecret - Start")
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
	err = s.SaveStore(ctx, "", s)
	return
}

func (store *Store) SetDbType(dbtype string) {
	//do nothing
}

func (store *Store) SetStoreTableName(tablename string) {
	//do nothing
}

func (store *Store) ReplaceVariables(ctx context.Context, projectId string, text []byte) (returnText []byte) {
	logs.WithContext(ctx).Debug("ReplaceVariables - Start")
	textStr := string(text)
	if store.Variables[projectId] != nil {
		for k, v := range store.Variables[projectId].Vars {
			textStr = strings.Replace(textStr, fmt.Sprint("$VAR_", k), v.Value, -1)
		}
		for k, v := range store.Variables[projectId].EnvVars {
			textStr = strings.Replace(textStr, fmt.Sprint("$ENV_", k), v.Value, -1)
		}
		for k, v := range store.Variables[projectId].Secrets {
			textStr = strings.Replace(textStr, fmt.Sprint("$SECRET_", k), v.Value, -1)
		}
	}
	return []byte(textStr)
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
