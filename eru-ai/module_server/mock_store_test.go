package module_server

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
	module_model "github.com/eru-tech/eru/eru-ai/module_model"
	module_store "github.com/eru-tech/eru/eru-ai/module_store"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	"github.com/eru-tech/eru/eru-cache/cache"
	db "github.com/eru-tech/eru/eru-db/db"
	"github.com/eru-tech/eru/eru-events/events"
	eru_models "github.com/eru-tech/eru/eru-models"
	validator "github.com/eru-tech/eru/eru-read-write/validator"
	repos "github.com/eru-tech/eru/eru-repos/repos"
	scheduler "github.com/eru-tech/eru/eru-scheduler/scheduler"
	kms "github.com/eru-tech/eru/eru-secret-manager/kms"
	sm "github.com/eru-tech/eru/eru-secret-manager/sm"
	store "github.com/eru-tech/eru/eru-store/store"
	vectorstore "github.com/eru-tech/eru/eru-vectorstore/vectorstore"
	"github.com/jmoiron/sqlx"
)

type mockTool struct {
	tools.Tool
	executeResult map[string]interface{}
	executeErr    error
}

func newMockTool(name, toolType, description string) *mockTool {
	t := &mockTool{
		executeResult: map[string]interface{}{"status": "ok"},
	}
	t.Tool.ToolName = name
	t.Tool.ToolType = toolType
	t.Tool.Description = description
	return t
}

func (m *mockTool) GetSpec() tools.Tooling { return m }

func (m *mockTool) Execute(_ context.Context, _, _ string, _ string, _ map[string]interface{}) (map[string]interface{}, bool, error) {
	return m.executeResult, false, m.executeErr
}

func (m *mockTool) GetToolDb() db.DbI                    { return nil }
func (m *mockTool) SetToolDb(_ db.DbI)                   {}
func (m *mockTool) GetParameters() eru_models.JSONSchema { return m.Tool.Parameters }

func (m *mockTool) BytesToTool(_ context.Context, _ []byte) (tools.Tooling, error)    { return m, nil }
func (m *mockTool) GetActionsList() []tools.ActionInfo                                { return nil }
func (m *mockTool) ValidateAction(_ context.Context, _ string, _ tools.Tooling) error { return nil }
func (m *mockTool) SetPrivateAttributes(_ context.Context, _ tools.Tooling) error     { return nil }
func (m *mockTool) GetInputFields() []tools.ToolInputFields                           { return nil }
func (m *mockTool) Callback(_ context.Context, _, _, _ string, _ map[string]interface{}, _ map[string][]string) (interface{}, bool, error) {
	return nil, false, nil
}
func (m *mockTool) ValidateOutput(_ context.Context, _ json.RawMessage) error { return nil }
func (m *mockTool) MakeFromJson(_ context.Context, _ *json.RawMessage) error  { return nil }
func (m *mockTool) GetAttribute(_ context.Context, attr string) (interface{}, error) {
	switch attr {
	case "tool_name":
		return m.Tool.ToolName, nil
	case "tool_type":
		return m.Tool.ToolType, nil
	case "description":
		return m.Tool.Description, nil
	case "parameters":
		return m.Tool.Parameters, nil
	case "output_schema":
		return m.Tool.OutputSchema, nil
	case "system_prompt":
		return m.Tool.SystemPrompt, nil
	}
	return nil, errors.New("attribute not found: " + attr)
}
func (m *mockTool) SetAttribute(_ context.Context, _ string, _ interface{}) error { return nil }
func (m *mockTool) GetToolCallback() tools.ToolCallback {
	return tools.ToolCallback{ResponseContentType: "application/json"}
}
func (m *mockTool) GetToolCbUrl(_, _ string) string { return "" }
func (m *mockTool) ExecuteHook(_ context.Context, _, _, _, _ string, _ map[string]interface{}, _ map[string][]string) (interface{}, error) {
	return nil, nil
}
func (m *mockTool) SetScheduler(_ scheduler.SchedulerI)                         {}
func (m *mockTool) SaveTenantSecret(_ context.Context, _, _, _, _ string) error { return nil }
func (m *mockTool) SetToolAction(_ string)                                      {}
func (m *mockTool) GetBytes(_ context.Context) ([]byte, error)                  { return nil, nil }

type mockAgent struct {
	agents.Agent
	executeResult agents.AgentMessage
	executeErr    error
}

func newMockAgent(name, agentType, description string) *mockAgent {
	a := &mockAgent{
		executeResult: agents.AgentMessage{Content: "mock agent response"},
	}
	a.Agent.AgentName = name
	a.Agent.AgentType = agentType
	a.Agent.Description = description
	return a
}

func (m *mockAgent) GetSpec() agents.AgentI { return m }

func (m *mockAgent) Execute(_ context.Context, _ agents.AgentMessage, _, _, _ string) (agents.AgentMessage, error) {
	return m.executeResult, m.executeErr
}

func (m *mockAgent) GetChatMemory() cache.CacheStoreI { return nil }

func (m *mockAgent) SetChatMemory(_ context.Context, _ cache.CacheStoreI) error { return nil }

type mockModuleStore struct {
	mu       sync.RWMutex
	projects map[string]*module_model.Project
	toolMap  map[string]map[string]map[string]tools.Tooling
	agentMap map[string]map[string]map[string]agents.AgentI
}

func newMockModuleStore() *mockModuleStore {
	return &mockModuleStore{
		projects: make(map[string]*module_model.Project),
		toolMap:  make(map[string]map[string]map[string]tools.Tooling),
		agentMap: make(map[string]map[string]map[string]agents.AgentI),
	}
}

func (m *mockModuleStore) addProject(p *module_model.Project) {
	m.mu.Lock()
	m.projects[p.ProjectId] = p
	m.mu.Unlock()
}

func (m *mockModuleStore) addTool(projectId, tenantId string, t tools.Tooling) {
	m.mu.Lock()
	if m.toolMap[projectId] == nil {
		m.toolMap[projectId] = make(map[string]map[string]tools.Tooling)
	}
	if m.toolMap[projectId][tenantId] == nil {
		m.toolMap[projectId][tenantId] = make(map[string]tools.Tooling)
	}
	name, _ := t.GetAttribute(context.Background(), "tool_name")
	m.toolMap[projectId][tenantId][name.(string)] = t
	m.mu.Unlock()
}

func (m *mockModuleStore) addAgent(projectId, tenantId string, a agents.AgentI) {
	m.mu.Lock()
	if m.agentMap[projectId] == nil {
		m.agentMap[projectId] = make(map[string]map[string]agents.AgentI)
	}
	if m.agentMap[projectId][tenantId] == nil {
		m.agentMap[projectId][tenantId] = make(map[string]agents.AgentI)
	}
	name, _ := a.GetAttribute(context.Background(), "agent_name")
	m.agentMap[projectId][tenantId][name.(string)] = a
	m.mu.Unlock()
}

func (m *mockModuleStore) GetProjectList(_ context.Context) []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []map[string]interface{}
	for id, p := range m.projects {
		list = append(list, map[string]interface{}{
			"project_id":   id,
			"project_name": p.ProjectId,
		})
	}
	return list
}

func (m *mockModuleStore) GetProjectConfig(_ context.Context, projectId string) (*module_model.Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.projects[projectId]
	if !ok {
		return nil, errors.New("project not found: " + projectId)
	}
	return p, nil
}

func (m *mockModuleStore) GetExtendedProjectConfig(_ context.Context, _ string, _ module_store.ModuleStoreI) (module_model.ExtendedProject, error) {
	return module_model.ExtendedProject{}, nil
}

func (m *mockModuleStore) GetToolNames(_ context.Context, projectId, tenantId string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.toolMap[projectId] == nil || m.toolMap[projectId][tenantId] == nil {
		return nil, nil
	}
	var names []string
	for n := range m.toolMap[projectId][tenantId] {
		names = append(names, n)
	}
	return names, nil
}

func (m *mockModuleStore) GetTool(_ context.Context, projectId, tenantId, toolName, _ string, _ module_store.ModuleStoreI) (tools.Tooling, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.toolMap[projectId] != nil && m.toolMap[projectId][tenantId] != nil {
		if t, ok := m.toolMap[projectId][tenantId][toolName]; ok {
			return t, nil
		}
	}
	return nil, errors.New("tool not found: " + toolName)
}

func (m *mockModuleStore) GetAgentNames(_ context.Context, projectId, tenantId string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.agentMap[projectId] == nil || m.agentMap[projectId][tenantId] == nil {
		return nil, nil
	}
	var names []string
	for n := range m.agentMap[projectId][tenantId] {
		names = append(names, n)
	}
	return names, nil
}

func (m *mockModuleStore) GetAgent(_ context.Context, projectId, tenantId, _ string, agentName string, _ module_store.ModuleStoreI) (agents.AgentI, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.agentMap[projectId] != nil && m.agentMap[projectId][tenantId] != nil {
		if a, ok := m.agentMap[projectId][tenantId][agentName]; ok {
			return a, nil
		}
	}
	return nil, errors.New("agent not found: " + agentName)
}

func (m *mockModuleStore) SaveProject(_ context.Context, _ string, _ module_store.ModuleStoreI, _ bool) error {
	return nil
}
func (m *mockModuleStore) RemoveProject(_ context.Context, _ string, _ module_store.ModuleStoreI) error {
	return nil
}
func (m *mockModuleStore) SaveModel(_ context.Context, _ models.ModelI, _, _ string, _ module_store.ModuleStoreI, _ bool) error {
	return nil
}
func (m *mockModuleStore) RemoveModel(_ context.Context, _, _, _ string, _ module_store.ModuleStoreI) error {
	return nil
}
func (m *mockModuleStore) GetModel(_ context.Context, _, _, _ string, _ module_store.ModuleStoreI) (models.ModelI, error) {
	return nil, nil
}
func (m *mockModuleStore) SaveAgent(_ context.Context, _ agents.AgentI, _, _ string, _ module_store.ModuleStoreI, _ bool) error {
	return nil
}
func (m *mockModuleStore) RemoveAgent(_ context.Context, _, _, _ string, _ module_store.ModuleStoreI) error {
	return nil
}
func (m *mockModuleStore) SaveVectorStore(_ context.Context, _ vectorstore.VectorStoreI, _, _ string, _ module_store.ModuleStoreI, _ bool) error {
	return nil
}
func (m *mockModuleStore) RemoveVectorStore(_ context.Context, _, _, _ string, _ module_store.ModuleStoreI) error {
	return nil
}
func (m *mockModuleStore) GetVectorStore(_ context.Context, _, _, _ string, _ module_store.ModuleStoreI) (vectorstore.VectorStoreI, error) {
	return nil, nil
}
func (m *mockModuleStore) GetVectorStoreCloneObject(_ context.Context, _, _ string, _ vectorstore.VectorStoreI, _ module_store.ModuleStoreI) (vectorstore.VectorStoreI, error) {
	return nil, nil
}
func (m *mockModuleStore) SyncVectorStore(_ context.Context, _, _, _ string, _ module_store.ModuleStoreI) error {
	return nil
}
func (m *mockModuleStore) GetVectorStoreNames(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}
func (m *mockModuleStore) SaveVectors(_ context.Context, _ vectorstore.VectorRecords, _, _, _ string, _ module_store.ModuleStoreI) error {
	return nil
}
func (m *mockModuleStore) RemoveVectors(_ context.Context, _ vectorstore.VectorRecordsDelete, _, _, _ string, _ module_store.ModuleStoreI) error {
	return nil
}
func (m *mockModuleStore) SearchVectors(_ context.Context, _ vectorstore.VectorRecordsSearch, _, _, _ string, _ module_store.ModuleStoreI) (vectorstore.VectorResults, error) {
	return vectorstore.VectorResults{}, nil
}
func (m *mockModuleStore) ListVectors(_ context.Context, _ vectorstore.VectorRecordsList, _, _, _ string, _ module_store.ModuleStoreI) (vectorstore.VectorResults, error) {
	return vectorstore.VectorResults{}, nil
}
func (m *mockModuleStore) SaveProjectSettings(_ context.Context, _ string, _ module_model.ProjectSettings, _ module_store.ModuleStoreI) error {
	return nil
}
func (m *mockModuleStore) RemoveTenants() {}
func (m *mockModuleStore) SaveTool(_ context.Context, _ tools.Tooling, _, _ string, _ module_store.ModuleStoreI, _ bool) error {
	return nil
}
func (m *mockModuleStore) RemoveTool(_ context.Context, _, _, _ string, _ module_store.ModuleStoreI) error {
	return nil
}

func (m *mockModuleStore) LoadStore(_ string, _ store.StoreI) error   { return nil }
func (m *mockModuleStore) GetStoreByteArray(_ string) ([]byte, error) { return nil, nil }
func (m *mockModuleStore) SaveStore(_ context.Context, _, _ string, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) SaveTenantStore(_ context.Context, _, _, _ string, _ interface{}) error {
	return nil
}
func (m *mockModuleStore) SaveTenantObject(_ context.Context, _, _, _, _, _, _ string, _ interface{}, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) RemoveTenantObject(_ context.Context, _, _, _, _, _, _ string, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) SetStoreTenantLoadQuery(_ string) {}
func (m *mockModuleStore) SetDbType(_ string)               {}
func (m *mockModuleStore) CreateConn() error  { return nil }
func (m *mockModuleStore) GetConn() *sqlx.DB  { return nil }
func (m *mockModuleStore) GetDbType() string  { return "STANDALONE" }
func (m *mockModuleStore) ExecuteDbSave(_ context.Context, _ []store.Queries) ([][]map[string]interface{}, error) {
	return nil, nil
}
func (m *mockModuleStore) ExecuteDbFetch(_ context.Context, _ store.Queries) ([]map[string]interface{}, error) {
	return nil, nil
}
func (m *mockModuleStore) SetStoreTableName(_ string)       {}
func (m *mockModuleStore) SetStoreTenantTableName(_ string) {}
func (m *mockModuleStore) GetStoreTableName() string        { return "" }
func (m *mockModuleStore) GetStoreWithoutTenants(_ context.Context, _ store.StoreI) ([]byte, error) {
	return nil, nil
}
func (m *mockModuleStore) GetStoreTenantTableName() string                         { return "" }
func (m *mockModuleStore) SetVars(_ context.Context, _ map[string]store.Variables) {}
func (m *mockModuleStore) SetTenantVars(_ context.Context, _ map[string]map[string]store.Variables) {
}
func (m *mockModuleStore) SaveVar(_ context.Context, _ string, _ store.Vars, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) RemoveVar(_ context.Context, _, _ string, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) SaveEnvVar(_ context.Context, _ string, _ store.EnvVars, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) RemoveEnvVar(_ context.Context, _, _ string, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) SaveSecret(_ context.Context, _ string, _ store.Secrets, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) RemoveSecret(_ context.Context, _, _ string, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) FetchVars(_ context.Context, _ string) (store.Variables, error) {
	return store.Variables{}, nil
}
func (m *mockModuleStore) FetchTenantVars(_ context.Context, _ string) (map[string]store.Variables, error) {
	return nil, nil
}
func (m *mockModuleStore) ReplaceVariables(_ context.Context, _ string, b []byte, _ map[string]interface{}) []byte {
	return b
}
func (m *mockModuleStore) ReplaceTenantVariables(_ context.Context, _, _, _ string, b []byte) []byte {
	return b
}
func (m *mockModuleStore) SaveTenantSecret(_ context.Context, _, _ string, _ store.Secrets, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) RemoveTenantSecret(_ context.Context, _, _, _ string, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) SaveRepo(_ context.Context, _ string, _ repos.RepoI, _ store.StoreI, _ bool) error {
	return nil
}
func (m *mockModuleStore) SaveRepoToken(_ context.Context, _ string, _ repos.RepoToken, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) FetchRepo(_ context.Context, _ string) (repos.RepoI, error) {
	return nil, nil
}
func (m *mockModuleStore) CommitRepo(_ context.Context, _ string, _ store.StoreI) error { return nil }
func (m *mockModuleStore) GetProjectConfigForRepo(_ context.Context, _ string, _ store.StoreI) (map[string]map[string]interface{}, string, error) {
	return nil, "", nil
}
func (m *mockModuleStore) SaveSm(_ context.Context, _ string, _ sm.SmStoreI, _ store.StoreI, _ bool) error {
	return nil
}
func (m *mockModuleStore) FetchSm(_ context.Context, _ string) (sm.SmStoreI, error) { return nil, nil }
func (m *mockModuleStore) LoadSmValue(_ context.Context, _ string) error            { return nil }
func (m *mockModuleStore) SetSmValue(_ context.Context, _ string, _ string, _ map[string]string) error {
	return nil
}
func (m *mockModuleStore) UnsetSmValue(_ context.Context, _, _, _ string) error { return nil }
func (m *mockModuleStore) GetSmValue(_ context.Context, _, _, _ string, _ bool) (interface{}, error) {
	return nil, nil
}
func (m *mockModuleStore) LoadEnvValue(_ context.Context, _ string) error { return nil }
func (m *mockModuleStore) SetStoreFromBytes(_ context.Context, _ []byte, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) GetMutex() *sync.RWMutex { return &sync.RWMutex{} }
func (m *mockModuleStore) FetchKms(_ context.Context, _ string) (map[string]kms.KmsStoreI, error) {
	return nil, nil
}
func (m *mockModuleStore) SaveKms(_ context.Context, _ string, _ kms.KmsStoreI, _ store.StoreI, _ bool) error {
	return nil
}
func (m *mockModuleStore) RemoveKms(_ context.Context, _ string, _ string, _ bool, _ int32, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) GetCacheValue(_ context.Context, _, _ string) (interface{}, error) {
	return nil, nil
}
func (m *mockModuleStore) SetCacheValue(_ context.Context, _, _ string, _ interface{}) error {
	return nil
}
func (m *mockModuleStore) ValidateJSON(_ context.Context, _ validator.Schema, data []interface{}) ([]interface{}, []interface{}) {
	return data, nil
}
func (m *mockModuleStore) FetchEvents(_ context.Context, _ string) (map[string]events.EventI, error) {
	return nil, nil
}
func (m *mockModuleStore) FetchEvent(_ context.Context, _, _ string) (events.EventI, error) {
	return nil, nil
}
func (m *mockModuleStore) SaveEvent(_ context.Context, _ string, _ events.EventI, _ store.StoreI, _ bool) error {
	return nil
}
func (m *mockModuleStore) CloneEvent(_ context.Context, _ string, _ events.EventI, _ store.StoreI) (events.EventI, error) {
	return nil, nil
}
func (m *mockModuleStore) RemoveEvent(_ context.Context, _ string, _ string, _ bool, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) PublishEvent(_ context.Context, _, _ string, _ interface{}, _ store.StoreI) (string, error) {
	return "", nil
}
func (m *mockModuleStore) PollEvent(_ context.Context, _, _ string, _ store.StoreI) error { return nil }
func (m *mockModuleStore) SaveScheduler(_ context.Context, _ string, _ scheduler.SchedulerI, _ store.StoreI, _ bool) error {
	return nil
}
func (m *mockModuleStore) FetchScheduler(_ context.Context, _ string) (scheduler.SchedulerI, error) {
	return nil, nil
}
func (m *mockModuleStore) InitScheduler(_ context.Context, _ store.StoreI) error { return nil }
func (m *mockModuleStore) SaveRequest(_ context.Context, _ eru_models.SampleRequest, _, _ string, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) GetRequests(_ context.Context, _, _, _ string, _ store.StoreI) ([]eru_models.SampleRequest, error) {
	return nil, nil
}
func (m *mockModuleStore) RemoveRequest(_ context.Context, _ string, _ store.StoreI) error {
	return nil
}
func (m *mockModuleStore) SetServiceName(_ string)     {}
func (m *mockModuleStore) SetInstanceId(_ string)      {}
func (m *mockModuleStore) SetBaseUrl(_ string)         {}
func (m *mockModuleStore) SetConfigSyncEvent(_ string) {}
func (m *mockModuleStore) GetUpdateTime() time.Time    { return time.Now() }

func newTestStoreHolder() *module_store.StoreHolder {
	ms := newMockModuleStore()

	ms.addProject(&module_model.Project{
		ProjectId: "test-project",
		Tenants: map[string]module_model.TenantConfig{
			"tenant-abc123": {TenantId: "tenant-abc123"},
		},
	})
	ms.addTool("test-project", "tenant-abc123", newMockTool("search", "CUSTOM", "Search the knowledge base"))
	ms.addAgent("test-project", "tenant-abc123", newMockAgent("chatbot", "REFLEX", "Conversational AI agent"))

	ms.addProject(&module_model.Project{
		ProjectId: "processo",
		Tenants: map[string]module_model.TenantConfig{
			"39acd634-577e-41ba-aa56-df5695208696": {TenantId: "39acd634-577e-41ba-aa56-df5695208696"},
		},
	})
	ms.addAgent("processo", "39acd634-577e-41ba-aa56-df5695208696", newMockAgent("chatbot", "REFLEX", "Conversational AI agent"))

	return &module_store.StoreHolder{Store: ms}
}
