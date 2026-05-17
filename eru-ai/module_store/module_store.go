package module_store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
	module_model "github.com/eru-tech/eru/eru-ai/module_model"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	db "github.com/eru-tech/eru/eru-db/db"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	scheduler "github.com/eru-tech/eru/eru-scheduler/scheduler"
	"github.com/eru-tech/eru/eru-store/store"
	vectorstore "github.com/eru-tech/eru/eru-vectorstore/vectorstore"
)

var Erufuncbaseurl = "http://localhost:8083"
var Eruauthbaseurl = "http://localhost:8085"
var Eruqlbaseurl = "http://localhost:8087"
var Eruaibaseurl = "http://localhost:8088"
var Erufilesbaseurl = "http://localhost:8082"
var Eruaiport = "8088"

type StoreHolder struct {
	sync.RWMutex
	Store ModuleStoreI
}
type ModuleStoreI interface {
	store.StoreI
	SaveProject(ctx context.Context, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error
	GetProjectConfig(ctx context.Context, projectId string) (*module_model.Project, error)
	GetExtendedProjectConfig(ctx context.Context, projectId string, realStore ModuleStoreI) (module_model.ExtendedProject, error)
	GetProjectList(ctx context.Context) []map[string]interface{}
	SaveModel(ctx context.Context, modelObj models.ModelI, projectId string, tenantId string, realStore ModuleStoreI, persist bool) error
	RemoveModel(ctx context.Context, modelName string, projectId string, tenantId string, realStore ModuleStoreI) error
	GetModel(ctx context.Context, projectId string, tenantId string, modelName string, s ModuleStoreI) (models.ModelI, error)
	SaveAgent(ctx context.Context, agentObj agents.AgentI, projectId string, tenantId string, realStore ModuleStoreI, persist bool) error
	RemoveAgent(ctx context.Context, agentName string, projectId string, tenantId string, realStore ModuleStoreI) error
	GetAgent(ctx context.Context, projectId string, tenantId string, conversationId string, agentName string, s ModuleStoreI) (agents.AgentI, error)
	SaveVectorStore(ctx context.Context, vectorStoreObj vectorstore.VectorStoreI, projectId string, tenantId string, realStore ModuleStoreI, persist bool) error
	RemoveVectorStore(ctx context.Context, vectorStoreName string, projectId string, tenantId string, realStore ModuleStoreI) error
	GetVectorStore(ctx context.Context, projectId string, tenantId string, vectorStoreName string, realStore ModuleStoreI) (vectorstore.VectorStoreI, error)
	GetVectorStoreCloneObject(ctx context.Context, projectId string, tenantId string, vectorStoreObj vectorstore.VectorStoreI, s ModuleStoreI) (vectorStoreObjClone vectorstore.VectorStoreI, err error)
	SyncVectorStore(ctx context.Context, vectorStoreName string, projectId string, tenantId string, realStore ModuleStoreI) error
	GetVectorStoreNames(ctx context.Context, projectID string, tenantID string) (vectorStoreNames []string, err error)
	SaveVectors(ctx context.Context, vectorRecords vectorstore.VectorRecords, vectorName string, projectId string, tenantId string, realStore ModuleStoreI) error
	RemoveVectors(ctx context.Context, vectorRecords vectorstore.VectorRecordsDelete, vectorName string, projectId string, tenantId string, realStore ModuleStoreI) error
	SearchVectors(ctx context.Context, vectorRecords vectorstore.VectorRecordsSearch, vectorName string, projectId string, tenantId string, realStore ModuleStoreI) (vectorstore.VectorResults, error)
	ListVectors(ctx context.Context, vectorRecords vectorstore.VectorRecordsList, vectorName string, projectId string, tenantId string, realStore ModuleStoreI) (vectorstore.VectorResults, error)
	SaveProjectSettings(ctx context.Context, projectId string, projectSettings module_model.ProjectSettings, realStore ModuleStoreI) error
	RemoveTenants()
	SaveTool(ctx context.Context, tooling tools.Tooling, projectId string, tenantId string, realStore ModuleStoreI, persist bool) error
	RemoveTool(ctx context.Context, toolName string, projectId string, tenantId string, realStore ModuleStoreI) error
	GetTool(ctx context.Context, projectId string, tenantId string, toolName string, actionName string, s ModuleStoreI) (tools.Tooling, error)
	GetAgentNames(ctx context.Context, projectID string, tenantID string) (agentNames []string, err error)
	GetToolNames(ctx context.Context, projectID string, tenantID string) (toolNames []string, err error)
}

type ModuleStore struct {
	Projects map[string]*module_model.Project `json:"projects"` //ProjectId is the key
}

type ModuleFileStore struct {
	store.FileStore
	ModuleStore
}
type ModuleDbStore struct {
	store.DbStore
	ModuleStore
}

func (ms *ModuleStore) SaveProject(ctx context.Context, projectId string, realStore ModuleStoreI, persist bool) error {
	//TODO to handle edit project once new project attributes are finalized
	logs.WithContext(ctx).Debug("SaveProject - Start")
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}
	if _, ok := ms.Projects[projectId]; !ok {
		project := new(module_model.Project)
		project.ProjectId = projectId
		if ms.Projects == nil {
			ms.Projects = make(map[string]*module_model.Project)
		}
		if project.Tenants == nil {
			project.Tenants = make(map[string]module_model.TenantConfig)
		}

		ms.Projects[projectId] = project
		if persist {
			logs.WithContext(ctx).Info("SaveStore called from SaveProject")
			return realStore.SaveStore(ctx, projectId, "", realStore)
		} else {
			return nil
		}
	} else {
		err := errors.New("Project " + projectId + " already exists")
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}

func (ms *ModuleStore) RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveProject - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if _, ok := ms.Projects[projectId]; ok {
		delete(ms.Projects, projectId)
		logs.WithContext(ctx).Info("SaveStore called from RemoveProject")
		return realStore.SaveStore(ctx, projectId, "", realStore)
	} else {
		err := errors.New("Project " + projectId + " does not exists")
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}

func (ms *ModuleStore) GetExtendedProjectConfig(ctx context.Context, projectId string, realStore ModuleStoreI) (ePrj module_model.ExtendedProject, err error) {
	logs.WithContext(ctx).Debug("GetExtendedProjectConfig - Start")
	ePrj = module_model.ExtendedProject{}
	if prj, ok := ms.Projects[projectId]; ok {
		ePrj.TenantVariables, err = realStore.FetchTenantVars(ctx, projectId)
		if err != nil {
			logs.WithContext(ctx).Warn(err.Error())
		}
		ePrj.Variables, err = realStore.FetchVars(ctx, projectId)
		if err != nil {
			logs.WithContext(ctx).Warn(err.Error())
		}
		ePrj.SecretManager, err = realStore.FetchSm(ctx, projectId)
		if err != nil {
			logs.WithContext(ctx).Warn(err.Error())
		}
		ePrj.Scheduler, err = realStore.FetchScheduler(ctx, projectId)
		if err != nil {
			logs.WithContext(ctx).Warn(err.Error())
		}
		ePrj.ProjectId = prj.ProjectId
		ePrj.ProjectSettings = prj.ProjectSettings
		ePrj.Tenants = prj.Tenants
		return ePrj, nil
	} else {
		err = errors.New("Project " + projectId + " does not exists")
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return module_model.ExtendedProject{}, err
	}
}

func (ms *ModuleStore) GetProjectConfig(ctx context.Context, projectId string) (*module_model.Project, error) {
	logs.WithContext(ctx).Debug("GetProjectConfig - Start")
	if _, ok := ms.Projects[projectId]; ok {
		return ms.Projects[projectId], nil
	} else {
		err := errors.New("Project " + projectId + " does not exists")
		logs.WithContext(ctx).Info(err.Error())
		return nil, err
	}
}

func (ms *ModuleStore) GetProjectList(ctx context.Context) []map[string]interface{} {
	logs.WithContext(ctx).Debug("GetProjectList - Start")
	projects := make([]map[string]interface{}, len(ms.Projects))
	i := 0
	for k := range ms.Projects {
		project := make(map[string]interface{})
		project["project_name"] = k
		projects[i] = project
		i++
	}
	return projects
}
func (ms *ModuleStore) SaveModel(ctx context.Context, modelObj models.ModelI, projectId string, tenantId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveModel - Start")
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}

	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return err
	}

	//save original modelObj with variables
	err = prj.AddModel(ctx, tenantId, modelObj)
	if err != nil {
		return err
	}

	if persist {
		return realStore.SaveTenantStore(ctx, projectId, tenantId, "", prj.Tenants[tenantId])
	}
	return nil
}

func (ms *ModuleStore) RemoveModel(ctx context.Context, modelName string, projectId string, tenantId string, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("RemoveModel - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()

	if prj, ok := ms.Projects[projectId]; ok {
		if _, ok := prj.Tenants[tenantId]; !ok {
			err = errors.New("tenant " + tenantId + " does not exists")
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
		if _, ok := prj.Tenants[tenantId].Models[modelName]; !ok {
			err = errors.New("Model " + modelName + " does not exists")
			logs.WithContext(ctx).Info(err.Error())
			return err
		}
		err = prj.RemoveModel(ctx, tenantId, modelName)
		if err != nil {
			return err
		}
		return realStore.SaveTenantStore(ctx, projectId, tenantId, "", prj.Tenants[tenantId])
	} else {
		err = errors.New("Project " + projectId + " does not exists")
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}

func (ms *ModuleStore) GetModelClone(ctx context.Context, projectId string, tenantId string, modelName string, s ModuleStoreI) (modelObjClone models.ModelI, err error) {
	logs.WithContext(ctx).Debug("GetModelClone - Start")
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return
	}
	var modelObj models.ModelI
	if _, ok := prj.Tenants[tenantId]; !ok {
		err = errors.New("tenant " + tenantId + " not found")
		logs.WithContext(ctx).Error(err.Error())
		return
	} else if modelObj, ok = prj.Tenants[tenantId].Models[modelName]; !ok {
		if projectModelObj, ok := prj.Tenants[projectId].Models[modelName]; ok {
			modelObj = projectModelObj
		} else {
			err = errors.New("model " + modelName + " not found")
			logs.WithContext(ctx).Error(err.Error())
			return
		}
	}
	modelObjClone, err = ms.GetModelCloneObject(ctx, projectId, tenantId, modelObj, s)
	return
}

func (ms *ModuleStore) GetModelCloneObject(ctx context.Context, projectId string, tenantId string, modelObj models.ModelI, s ModuleStoreI) (modelObjClone models.ModelI, err error) {
	logs.WithContext(ctx).Debug("GetModelCloneObject - Start")

	modelObjJson, modelObjJsonErr := json.Marshal(modelObj)
	if modelObjJsonErr != nil {
		err = errors.New("error while cloning modelObj (marshal)")
		logs.WithContext(ctx).Error(err.Error())
		logs.WithContext(ctx).Error(modelObjJsonErr.Error())
		return
	}
	modelObjJson = s.ReplaceTenantVariables(ctx, projectId, tenantId, "", modelObjJson)
	modelObjJson = s.ReplaceVariables(ctx, projectId, modelObjJson, nil)

	iCloneI := reflect.New(reflect.TypeOf(modelObj))
	modelObjCloneErr := json.Unmarshal(modelObjJson, iCloneI.Interface())
	if modelObjCloneErr != nil {
		err = errors.New("error while cloning modelObj(unmarshal)")
		logs.WithContext(ctx).Error(err.Error())
		logs.WithContext(ctx).Error(modelObjCloneErr.Error())
		return
	}
	return iCloneI.Elem().Interface().(models.ModelI), nil
}
func (ms *ModuleStore) GetModel(ctx context.Context, projectId string, tenantId string, modelName string, s ModuleStoreI) (models.ModelI, error) {
	logs.WithContext(ctx).Debug("GetModel - Start")
	return ms.GetModelClone(ctx, projectId, tenantId, modelName, s)

}

func (ms *ModuleStore) SaveTool(ctx context.Context, tooling tools.Tooling, projectId string, tenantId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveTool - Start")
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}

	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return err
	}

	//save original modelObj with variables
	err = prj.AddTool(ctx, tenantId, tooling)
	if err != nil {
		return err
	}

	if persist {
		return realStore.SaveTenantStore(ctx, projectId, tenantId, "", prj.Tenants[tenantId])
	}
	return nil
}

func (ms *ModuleStore) RemoveTool(ctx context.Context, toolName string, projectId string, tenantId string, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("RemoveTool - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()

	if prj, ok := ms.Projects[projectId]; ok {
		if _, ok := prj.Tenants[tenantId]; !ok {
			err = errors.New("tenant " + tenantId + " does not exists")
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
		if _, ok := prj.Tenants[tenantId].Tools[toolName]; !ok {
			err = errors.New("Tool " + toolName + " does not exists")
			logs.WithContext(ctx).Info(err.Error())
			return err
		}
		err = prj.RemoveTool(ctx, tenantId, toolName)
		if err != nil {
			return err
		}
		return realStore.SaveTenantStore(ctx, projectId, tenantId, "", prj.Tenants[tenantId])
	} else {
		err = errors.New("Project " + projectId + " does not exists")
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}

func (ms *ModuleStore) GetToolClone(ctx context.Context, projectId string, tenantId string, toolName string, actionName string, s ModuleStoreI) (toolObjClone tools.Tooling, err error) {
	logs.WithContext(ctx).Debug("GetToolClone - Start")
	logs.WithContext(ctx).Info(actionName)
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return
	}
	var toolObj tools.Tooling
	if _, ok := prj.Tenants[tenantId]; !ok {
		err = errors.New("tenant " + tenantId + " not found")
		logs.WithContext(ctx).Error(err.Error())
		return
	} else if toolObj, ok = prj.Tenants[tenantId].Tools[toolName]; !ok {
		if projectToolObj, ok := prj.Tenants[projectId].Tools[toolName]; ok {
			toolObj = projectToolObj
		} else {
			err = errors.New("tool " + toolName + " not found")
			logs.WithContext(ctx).Error(err.Error())
			return
		}
	}
	if actionName != "" {
		err = toolObj.ValidateAction(ctx, actionName, toolObj)
		if err != nil {
			return
		}
	}

	err = toolObj.SetPrivateAttributes(ctx, toolObj)
	if err != nil {
		return
	}

	toolObjClone, err = ms.GetToolCloneObject(ctx, projectId, tenantId, toolObj, s)
	if err != nil {
		return
	}
	toolObjClone.SetToolDb(db.GetDb(s.GetDbType()))
	toolObjClone.GetToolDb().SetConn(s.GetConn())
	toolObjClone.SetToolAction(actionName)

	var scheduler scheduler.SchedulerI
	scheduler, err = s.FetchScheduler(ctx, projectId)
	if err == nil {
		toolObjClone.SetScheduler(scheduler)
	} else {
		err = nil //ignore error and allow rest of object to be cloned
	}

	vectorStoreName, _ := toolObjClone.GetAttribute(ctx, "vectorstore_name")
	if vectorStoreName != nil {
		vectorStoreName := vectorStoreName.(string)
		vectorStore, vectorStoreErr := s.GetVectorStore(ctx, projectId, tenantId, vectorStoreName, s)
		if vectorStoreErr != nil {
			err = vectorStoreErr
			logs.WithContext(ctx).Error(err.Error())
			return
		}
		vectorStoreClone, vectorStoreCloneErr := s.GetVectorStoreCloneObject(ctx, projectId, tenantId, vectorStore, s)
		if vectorStoreCloneErr != nil {
			err = vectorStoreCloneErr
			logs.WithContext(ctx).Error(err.Error())
			return
		} else {
			toolObjClone.SetAttribute(ctx, "vectorstore", vectorStoreClone)
		}

		embed, embedErr := vectorStoreClone.GetEmbed(ctx)
		if embedErr != nil {
			err = embedErr
			logs.WithContext(ctx).Error(err.Error())
			return
		}

		dimension := vectorStoreClone.GetAttribute(ctx, "dimension")
		dimensionInt := 0
		if dimension != "" {
			dimensionInt, err = strconv.Atoi(dimension)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return
			}
		}
		embed.Dimension = dimensionInt
		embed.Metric = vectorStoreClone.GetAttribute(ctx, "metric")
		if embed.ModelName != "" {
			model, err := s.GetModel(ctx, projectId, tenantId, embed.ModelName, s)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
			}
			embed.Model = model
			err = vectorStoreClone.SetEmbed(ctx, embed)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
			}
		}
	}

	return
}

func (ms *ModuleStore) GetToolCloneObject(ctx context.Context, projectId string, tenantId string, toolObj tools.Tooling, s ModuleStoreI) (toolObjClone tools.Tooling, err error) {
	logs.WithContext(ctx).Debug("GetToolCloneObject - Start")

	toolObjJson, toolObjJsonErr := toolObj.GetBytes(ctx)
	if toolObjJsonErr != nil {
		return
	}
	toolObjJson = s.ReplaceTenantVariables(ctx, projectId, tenantId, "", toolObjJson)
	toolObjJson = s.ReplaceVariables(ctx, projectId, toolObjJson, nil)

	return toolObj.BytesToTool(ctx, toolObjJson)

	/* iCloneI := reflect.New(reflect.TypeOf(toolObj))
	toolObjCloneErr := json.Unmarshal(toolObjJson, iCloneI.Interface())
	if toolObjCloneErr != nil {
		err = errors.New("error while cloning toolObj(unmarshal)")
		logs.WithContext(ctx).Error(err.Error())
		logs.WithContext(ctx).Error(toolObjCloneErr.Error())
		return
	}
	return iCloneI.Elem().Interface().(tools.Tooling), nil */
}
func (ms *ModuleStore) GetVectorStoreCloneObject(ctx context.Context, projectId string, tenantId string, vectorStoreObj vectorstore.VectorStoreI, s ModuleStoreI) (vectorStoreObjClone vectorstore.VectorStoreI, err error) {
	logs.WithContext(ctx).Debug("GetVectorStoreCloneObject - Start")

	vectorStoreObjJson, vectorStoreObjJsonErr := vectorStoreObj.GetBytes(ctx)
	if vectorStoreObjJsonErr != nil {
		return
	}
	vectorStoreObjJson = s.ReplaceTenantVariables(ctx, projectId, tenantId, "", vectorStoreObjJson)
	vectorStoreObjJson = s.ReplaceVariables(ctx, projectId, vectorStoreObjJson, nil)

	return vectorStoreObj.BytesToVectorStore(ctx, vectorStoreObjJson)

}
func (ms *ModuleStore) GetTool(ctx context.Context, projectId string, tenantId string, toolName string, actionName string, s ModuleStoreI) (toolObjClone tools.Tooling, err error) {
	logs.WithContext(ctx).Debug("GetTool - Start")
	return ms.GetToolClone(ctx, projectId, tenantId, toolName, actionName, s)

}

func (ms *ModuleStore) SaveAgent(ctx context.Context, agentObj agents.AgentI, projectId string, tenantId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveAgent - Start")
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}

	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return err
	}

	//save original modelObj with variables
	err = prj.AddAgent(ctx, tenantId, agentObj)
	if err != nil {
		return err
	}

	if persist {
		return realStore.SaveTenantStore(ctx, projectId, tenantId, "", prj.Tenants[tenantId])
	}
	return nil
}

func (ms *ModuleStore) RemoveAgent(ctx context.Context, agentName string, projectId string, tenantId string, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("RemoveAgent - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()

	if prj, ok := ms.Projects[projectId]; ok {
		if _, ok := prj.Tenants[tenantId]; !ok {
			err = errors.New("tenant " + tenantId + " does not exists")
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
		if _, ok := prj.Tenants[tenantId].Agents[agentName]; !ok {
			err = errors.New("Agent " + agentName + " does not exists")
			logs.WithContext(ctx).Info(err.Error())
			return err
		}
		err = prj.RemoveAgent(ctx, tenantId, agentName)
		if err != nil {
			return err
		}
		return realStore.SaveTenantStore(ctx, projectId, tenantId, "", prj.Tenants[tenantId])
	} else {
		err = errors.New("Project " + projectId + " does not exists")
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}

func (ms *ModuleStore) GetAgentClone(ctx context.Context, projectId string, tenantId string, conversationId string, agentName string, s ModuleStoreI) (agentObjClone agents.AgentI, err error) {
	logs.WithContext(ctx).Debug("GetAgentClone - Start")
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return
	}
	var agentObj agents.AgentI
	if _, ok := prj.Tenants[tenantId]; !ok {
		err = errors.New("tenant " + tenantId + " not found")
		logs.WithContext(ctx).Error(err.Error())
		return
	} else if agentObj, ok = prj.Tenants[tenantId].Agents[agentName]; !ok {
		if projectAgentObj, ok := prj.Tenants[projectId].Agents[agentName]; ok {
			agentObj = projectAgentObj
		} else {
			err = errors.New("agent " + agentName + " not found")
			logs.WithContext(ctx).Error(err.Error())
			return
		}
	}
	agentObjClone, err = ms.GetAgentCloneObject(ctx, projectId, tenantId, conversationId, agentObj, s)
	if err != nil {
		return
	}
	agentObjClone.SetProvider(agentObj.GetProvider())
	cacheI := agentObj.GetChatMemory()
	if cacheI != nil {
		err := agentObjClone.GetChatMemory().SyncPersistence(ctx, agentObj.GetChatMemory())
		if err != nil {
			return nil, err
		}
	}
	err = agentObjClone.ValidateChatMemory(ctx, projectId)
	if err != nil {
		return nil, err
	}
	return
}

func (ms *ModuleStore) GetAgentCloneObject(ctx context.Context, projectId string, tenantId string, conversationId string, agentObj agents.AgentI, s ModuleStoreI) (agentObjClone agents.AgentI, err error) {
	logs.WithContext(ctx).Debug("GetAgentCloneObject - Start")

	agentObjJson, agentObjJsonErr := json.Marshal(agentObj)
	if agentObjJsonErr != nil {
		err = errors.New("error while cloning agentObj (marshal)")
		logs.WithContext(ctx).Error(err.Error())
		logs.WithContext(ctx).Error(agentObjJsonErr.Error())
		return
	}
	agentObjJson = s.ReplaceTenantVariables(ctx, projectId, tenantId, conversationId, agentObjJson)
	agentObjJson = s.ReplaceVariables(ctx, projectId, agentObjJson, nil)

	iCloneI := reflect.New(reflect.TypeOf(agentObj))
	agentObjCloneErr := json.Unmarshal(agentObjJson, iCloneI.Interface())
	if agentObjCloneErr != nil {
		err = errors.New("error while cloning agentObj(unmarshal)")
		logs.WithContext(ctx).Error(err.Error())
		logs.WithContext(ctx).Error(agentObjCloneErr.Error())
		return
	}
	return iCloneI.Elem().Interface().(agents.AgentI), nil
}

// populateAgentTools recursively populates all tools including dependent tools
func (ms *ModuleStore) populateAgentTools(ctx context.Context, projectId string, tenantId string, agentTools []agents.AgentTools, s ModuleStoreI) error {
	for i, agentTool := range agentTools {
		// Get the main tool
		tool, err := ms.GetTool(ctx, projectId, tenantId, agentTool.ToolName, agentTool.ActionName, s)
		if err != nil {
			return err
		}
		agentTools[i].Tool = tool

		// Recursively populate dependent tools if they exist
		if len(agentTool.DependentTools) > 0 {
			err = ms.populateAgentTools(ctx, projectId, tenantId, agentTool.DependentTools, s)
			if err != nil {
				return err
			}
			// Update the dependent tools in the current agent tool
			agentTools[i].DependentTools = agentTool.DependentTools
		}
	}
	return nil
}

func (ms *ModuleStore) GetAgent(ctx context.Context, projectId string, tenantId string, conversationId string, agentName string, s ModuleStoreI) (agents.AgentI, error) {
	logs.WithContext(ctx).Debug("GetAgent - Start")
	agent, err := ms.GetAgentClone(ctx, projectId, tenantId, conversationId, agentName, s)
	if err != nil {
		return nil, err
	}
	agentToolsI, err := agent.GetAttribute(ctx, "agent_tools")
	if err != nil {
		return nil, err
	}
	agentTools, ok := agentToolsI.([]agents.AgentTools)
	if !ok {
		return nil, errors.New("agent_tools attribute is not an array")
	}

	// Use the recursive function to populate all tools including dependent tools
	err = ms.populateAgentTools(ctx, projectId, tenantId, agentTools, s)
	if err != nil {
		return nil, err
	}

	modelNameI, err := agent.GetAttribute(ctx, "model")
	if err != nil {
		return nil, err
	}
	modelName, ok := modelNameI.(string)
	if !ok {
		return nil, errors.New("model attribute is not a string")
	}
	model, err := ms.GetModel(ctx, projectId, tenantId, modelName, s)
	if err != nil {
		return nil, err
	}
	agent.SetModel(model)

	agent.InitializeConversationManager(ctx)
	summaryModelNameI, err := agent.GetAttribute(ctx, "summary_model")
	if err != nil {
		err = nil //ignore error and continue with main model
		return agent, nil
	}
	summaryModelName, ok := summaryModelNameI.(string)
	if !ok {
		logs.WithContext(ctx).Error("summary model attribute is not a string")
		err = nil //ignore error and continue with main model
		return agent, nil
	}
	if summaryModelName != "" {
		summaryModel, err := ms.GetModel(ctx, projectId, tenantId, summaryModelName, s)
		if err != nil {
			err = nil //ignore error and continue with main model
			return agent, nil
		}
		agent.SetSummaryModel(summaryModel)
	}
	return agent, nil
}

func (ms *ModuleStore) SaveProjectSettings(ctx context.Context, projectId string, projectSettings module_model.ProjectSettings, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("SaveProjectConfig - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	err := ms.checkProjectExists(ctx, projectId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	ms.Projects[projectId].ProjectSettings = projectSettings
	logs.WithContext(ctx).Info("SaveStore called from SaveProjectSettings")
	return realStore.SaveStore(ctx, projectId, "", realStore)
}
func (ms *ModuleStore) GetStoreWithoutTenants(ctx context.Context, realStore store.StoreI) (b []byte, err error) {
	logs.WithContext(ctx).Debug("GetStoreByteArrayWithoutTenants - Start")
	logs.WithContext(ctx).Info("calling custom get store byte array without tenants from eruai")

	realStoreJson, err := json.Marshal(realStore)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	newMs := new(ModuleDbStore)
	err = UnMarshalStore(ctx, realStoreJson, newMs)
	//err = json.Unmarshal(realStoreJson, newMs)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	newMs.RemoveTenants()
	b, err = json.Marshal(newMs)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	return
}

func (ms *ModuleStore) RemoveTenants() {
	for key, project := range ms.Projects {
		project.Tenants = nil
		ms.Projects[key] = project
	}
}

func (ms *ModuleStore) GetAgentNames(ctx context.Context, projectId string, tenantId string) (agentNames []string, err error) {
	logs.WithContext(ctx).Debug("GetAgentNames - Start")
	if prj, ok := ms.Projects[projectId]; ok {
		for _, tenant := range prj.Tenants {
			if tenantId == "" || tenantId == tenant.TenantId || projectId == tenant.TenantId {
				for agentName := range tenant.Agents {
					agentNames = append(agentNames, agentName)
				}
			}
		}
		return agentNames, nil
	} else {
		err = errors.New("Project " + projectId + " does not exist")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func (ms *ModuleStore) GetToolNames(ctx context.Context, projectId string, tenantId string) (toolNames []string, err error) {
	logs.WithContext(ctx).Debug("GetToolNames - Start")

	if prj, ok := ms.Projects[projectId]; ok {
		for _, tenant := range prj.Tenants {
			if tenantId == "" || tenantId == tenant.TenantId || projectId == tenant.TenantId {
				for toolName := range tenant.Tools {
					toolNames = append(toolNames, toolName)
				}
			}
		}
		return toolNames, nil
	} else {
		err = errors.New("Project " + projectId + " does not exist")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func (ms *ModuleStore) SaveVectorStore(ctx context.Context, vectorStoreObj vectorstore.VectorStoreI, projectId string, tenantId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveVectorStore - Start")
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}

	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return err
	}

	isNew, updatedVectorStoreObj, err := prj.AddVectorStore(ctx, tenantId, vectorStoreObj)
	if err != nil {
		return err
	}
	if persist {
		vectorStoreObjClone, err := ms.GetVectorStoreCloneObject(ctx, projectId, tenantId, updatedVectorStoreObj, realStore)
		if err != nil {
			return err
		}

		if isNew {
			err = updatedVectorStoreObj.CreateIndex(ctx, vectorStoreObjClone)
			if err != nil {
				return err
			}
		} else {
			err = updatedVectorStoreObj.EditIndex(ctx, vectorStoreObjClone)
			if err != nil {
				return err
			}
		}
		return realStore.SaveTenantStore(ctx, projectId, tenantId, "", prj.Tenants[tenantId])
	}

	return nil
}
func (ms *ModuleStore) SyncVectorStore(ctx context.Context, vectorStoreName string, projectId string, tenantId string, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("SyncVectorStore - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()

	if prj, ok := ms.Projects[projectId]; ok {
		if _, ok := prj.Tenants[tenantId]; !ok {
			err = errors.New("tenant " + tenantId + " does not exists")
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
		if vs, ok := prj.Tenants[tenantId].VectorStores[vectorStoreName]; !ok {
			err = errors.New("VectorStore " + vectorStoreName + " does not exists")
			logs.WithContext(ctx).Info(err.Error())
			return err
		} else {
			vectorStoreClone, err := ms.GetVectorStoreCloneObject(ctx, projectId, tenantId, vs, realStore)
			if err != nil {
				return err
			}
			err = vs.SyncIndexDefinition(ctx, vectorStoreClone)
			if err != nil {
				return err
			}
			logs.WithContext(ctx).Info(fmt.Sprint(vs))
		}
		return realStore.SaveTenantStore(ctx, projectId, tenantId, "", prj.Tenants[tenantId])
	} else {
		err = errors.New("Project " + projectId + " does not exists")
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}
func (ms *ModuleStore) GetVectorStore(ctx context.Context, projectId string, tenantId string, vectorStoreName string, realStore ModuleStoreI) (vectorStore vectorstore.VectorStoreI, err error) {
	logs.WithContext(ctx).Debug("GetVectorStore - Start")

	if prj, ok := ms.Projects[projectId]; ok {
		if _, ok := prj.Tenants[tenantId]; !ok {
			err = errors.New("tenant " + tenantId + " does not exists")
			logs.WithContext(ctx).Error(err.Error())
			return
		}
		if vs, ok := prj.Tenants[tenantId].VectorStores[vectorStoreName]; !ok {
			if vs, ok = prj.Tenants[projectId].VectorStores[vectorStoreName]; !ok {
				err = errors.New("VectorStore " + vectorStoreName + " does not exists")
				logs.WithContext(ctx).Info(err.Error())
				return
			} else {
				return vs, nil
			}
		} else {
			return vs, nil
		}
	} else {
		err = errors.New("Project " + projectId + " does not exists")
		logs.WithContext(ctx).Info(err.Error())
		return nil, err
	}
}
func (ms *ModuleStore) RemoveVectorStore(ctx context.Context, vectorStoreName string, projectId string, tenantId string, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("RemoveVectorStore - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()

	if prj, ok := ms.Projects[projectId]; ok {
		if _, ok := prj.Tenants[tenantId]; !ok {
			err = errors.New("tenant " + tenantId + " does not exists")
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
		if vs, ok := prj.Tenants[tenantId].VectorStores[vectorStoreName]; !ok {
			err = errors.New("VectorStore " + vectorStoreName + " does not exists")
			logs.WithContext(ctx).Info(err.Error())
			return err
		} else {
			err = prj.RemoveVectorStore(ctx, tenantId, vectorStoreName)
			if err != nil {
				return err
			} else {
				vectorStoreClone, err := ms.GetVectorStoreCloneObject(ctx, projectId, tenantId, vs, realStore)
				if err != nil {
					return err
				}
				_ = vectorStoreClone.DeleteIndex(ctx, vs.GetAttribute(ctx, "index_name"))
				// ignore error from DeleteIndex and still persists the vectorstore
			}
			return realStore.SaveTenantStore(ctx, projectId, tenantId, "", prj.Tenants[tenantId])
		}
	} else {
		err = errors.New("Project " + projectId + " does not exists")
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}
func (ms *ModuleStore) SaveVectors(ctx context.Context, vectorRecords vectorstore.VectorRecords, vectorName string, projectId string, tenantId string, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveVectors - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()

	if prj, ok := ms.Projects[projectId]; ok {
		if _, ok := prj.Tenants[tenantId]; !ok {
			err = errors.New("tenant " + tenantId + " does not exists")
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
		if vs, ok := prj.Tenants[tenantId].VectorStores[vectorName]; !ok {
			err = errors.New("VectorStore " + vectorName + " does not exists")
			logs.WithContext(ctx).Info(err.Error())
			return err
		} else {
			vectorStoreClone, err := ms.GetVectorStoreCloneObject(ctx, projectId, tenantId, vs, realStore)
			if err != nil {
				return err
			}

			embed, err := vectorStoreClone.GetEmbed(ctx)
			if err != nil {
				return err
			}

			dimension := vectorStoreClone.GetAttribute(ctx, "dimension")
			dimensionInt := 0
			if dimension != "" {
				dimensionInt, err = strconv.Atoi(dimension)
				if err != nil {
					return err
				}
			}
			embed.Dimension = dimensionInt
			embed.Metric = vectorStoreClone.GetAttribute(ctx, "metric")
			if embed.ModelName != "" {
				model, err := ms.GetModel(ctx, projectId, tenantId, embed.ModelName, realStore)
				if err != nil {
					return err
				}
				embed.Model = model
				err = vectorStoreClone.SetEmbed(ctx, embed)
				if err != nil {
					return err
				}
			}

			err = vectorStoreClone.SaveVectors(ctx, vectorRecords)
			if err != nil {
				return err
			}
		}
	} else {
		err = errors.New("Project " + projectId + " does not exists")
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
	return nil
}
func (ms *ModuleStore) RemoveVectors(ctx context.Context, vectorRecordsDelete vectorstore.VectorRecordsDelete, vectorName string, projectId string, tenantId string, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("RemoveVectors - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()

	if prj, ok := ms.Projects[projectId]; ok {
		if _, ok := prj.Tenants[tenantId]; !ok {
			err = errors.New("tenant " + tenantId + " does not exists")
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
		if vs, ok := prj.Tenants[tenantId].VectorStores[vectorName]; !ok {
			err = errors.New("VectorStore " + vectorName + " does not exists")
			logs.WithContext(ctx).Info(err.Error())
			return err
		} else {
			vectorStoreClone, err := ms.GetVectorStoreCloneObject(ctx, projectId, tenantId, vs, realStore)
			if err != nil {
				return err
			}
			err = vectorStoreClone.DeleteVectors(ctx, vectorRecordsDelete)
			if err != nil {
				return err
			}
		}
	} else {
		err = errors.New("Project " + projectId + " does not exists")
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
	return nil

}
func (ms *ModuleStore) ListVectors(ctx context.Context, vectorRecordsList vectorstore.VectorRecordsList, vectorName string, projectId string, tenantId string, realStore ModuleStoreI) (vectorResults vectorstore.VectorResults, err error) {
	logs.WithContext(ctx).Debug("ListVectors - Start")

	if prj, ok := ms.Projects[projectId]; ok {
		if _, ok := prj.Tenants[tenantId]; !ok {
			err = errors.New("tenant " + tenantId + " does not exists")
			logs.WithContext(ctx).Error(err.Error())
			return vectorstore.VectorResults{}, err
		}
		if vs, ok := prj.Tenants[tenantId].VectorStores[vectorName]; !ok {
			err = errors.New("VectorStore " + vectorName + " does not exists")
			logs.WithContext(ctx).Info(err.Error())
			return vectorstore.VectorResults{}, err
		} else {
			vectorStoreClone, err := ms.GetVectorStoreCloneObject(ctx, projectId, tenantId, vs, realStore)
			if err != nil {
				return vectorstore.VectorResults{}, err
			}
			vectorResults, err = vectorStoreClone.ListVectors(ctx, vectorRecordsList)
			if err != nil {
				return vectorstore.VectorResults{}, err
			}
		}
	} else {
		err = errors.New("Project " + projectId + " does not exists")
		logs.WithContext(ctx).Info(err.Error())
		return vectorstore.VectorResults{}, err
	}
	return vectorResults, nil

}

func (ms *ModuleStore) SearchVectors(ctx context.Context, vectorRecords vectorstore.VectorRecordsSearch, vectorName string, projectId string, tenantId string, realStore ModuleStoreI) (vectorResults vectorstore.VectorResults, err error) {
	logs.WithContext(ctx).Debug("SearchVectors - Start")

	if prj, ok := ms.Projects[projectId]; ok {
		if _, ok := prj.Tenants[tenantId]; !ok {
			err = errors.New("tenant " + tenantId + " does not exists")
			logs.WithContext(ctx).Error(err.Error())
			return vectorstore.VectorResults{}, err
		}
		if vs, ok := prj.Tenants[tenantId].VectorStores[vectorName]; !ok {
			err = errors.New("VectorStore " + vectorName + " does not exists")
			logs.WithContext(ctx).Info(err.Error())
			return vectorstore.VectorResults{}, err
		} else {
			vectorStoreClone, err := ms.GetVectorStoreCloneObject(ctx, projectId, tenantId, vs, realStore)
			if err != nil {
				return vectorstore.VectorResults{}, err
			}

			embed, err := vectorStoreClone.GetEmbed(ctx)
			if err != nil {
				return vectorstore.VectorResults{}, err
			}

			dimension := vectorStoreClone.GetAttribute(ctx, "dimension")
			dimensionInt := 0
			if dimension != "" {
				dimensionInt, err = strconv.Atoi(dimension)
				if err != nil {
					return vectorstore.VectorResults{}, err
				}
			}
			embed.Dimension = dimensionInt
			embed.Metric = vectorStoreClone.GetAttribute(ctx, "metric")
			if embed.ModelName != "" {
				model, err := ms.GetModel(ctx, projectId, tenantId, embed.ModelName, realStore)
				if err != nil {
					return vectorstore.VectorResults{}, err
				}
				embed.Model = model
				err = vectorStoreClone.SetEmbed(ctx, embed)
				if err != nil {
					return vectorstore.VectorResults{}, err
				}
			}

			vectorResults, err = vectorStoreClone.SearchVectors(ctx, vectorRecords)
			if err != nil {
				return vectorstore.VectorResults{}, err
			}
		}
	} else {
		err = errors.New("Project " + projectId + " does not exists")
		logs.WithContext(ctx).Info(err.Error())
		return vectorstore.VectorResults{}, err
	}
	return vectorResults, nil
}
func (ms *ModuleStore) GetVectorStoreNames(ctx context.Context, projectId string, tenantId string) (vectorStoreNames []string, err error) {
	logs.WithContext(ctx).Debug("GetVectorStoreNames - Start")

	if prj, ok := ms.Projects[projectId]; ok {
		for _, tenant := range prj.Tenants {
			if tenantId == "" || tenantId == tenant.TenantId {
				for vectorStoreName := range tenant.VectorStores {
					vectorStoreNames = append(vectorStoreNames, vectorStoreName)
				}
			}
		}
		return vectorStoreNames, nil
	} else {
		err = errors.New("Project " + projectId + " does not exist")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}
func LoadStore(ctx context.Context, StoreTableName string, StoreTenantTableName string) (ModuleStoreI, error) {
	logs.WithContext(ctx).Info("Loading store")

	storeType := strings.ToUpper(os.Getenv("STORE_TYPE"))
	if storeType == "" {
		storeType = "STANDALONE"
		logs.WithContext(ctx).Info("STORE_TYPE environment variable not found - loading default standlone store")
	}
	var myStore ModuleStoreI
	var err error
	switch storeType {
	case "POSTGRES":
		myStore = new(ModuleDbStore)
		myStore.SetDbType(storeType)
		myStore.SetStoreTableName(StoreTableName)
		myStore.SetStoreTenantTableName(StoreTenantTableName)
		myStore.CreateConn()
	case "STANDALONE":
		// myStore, err = store.LoadStoreFromFile()
		myStore = new(ModuleFileStore)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New(fmt.Sprint("Invalid STORE_TYPE ", storeType))
	}
	storeBytes, err := myStore.GetStoreByteArray("")
	if err == nil {
		UnMarshalStore(ctx, storeBytes, myStore)
	} else {
		logs.WithContext(ctx).Error(err.Error())
	}
	//s.Store = myStore
	return myStore, err
}
