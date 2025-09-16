package module_store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	common_types "github.com/eru-tech/eru/eru-ql/common_types"
	"github.com/eru-tech/eru/eru-ql/ds"
	"github.com/eru-tech/eru/eru-ql/module_model"
	sqlengine "github.com/eru-tech/eru/eru-ql/sql_engine"
	eru_writes "github.com/eru-tech/eru/eru-read-write/eru_writes"
	"github.com/eru-tech/eru/eru-security-rule/security_rule"
	"github.com/eru-tech/eru/eru-store/store"
)

const (
	Q_SELECT = "SELECT"
	Q_WITH   = "WITH"
	Q_INSERT = "INSERT"
	Q_UPDATE = "UPDATE"
	Q_DELETE = "DELETE"
)

type StoreHolder struct {
	sync.RWMutex
	Store ModuleStoreI
}
type ModuleStoreI interface {
	store.StoreI
	SaveProject(ctx context.Context, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error
	GetExtendedProjectConfig(ctx context.Context, projectId string, realStore ModuleStoreI) (module_model.ExtendedProject, error)
	GetProjectConfig(ctx context.Context, projectId string) (*module_model.Project, error)
	GetProjectSettingsObject(ctx context.Context, projectId string) (pc module_model.ProjectSettings, err error)
	GetProjectList(ctx context.Context) []map[string]interface{}
	SetDataSourceConnections(ctx context.Context, realStore ModuleStoreI) (err error)
	SaveProjectSettings(ctx context.Context, projectId string, projectConfig module_model.ProjectSettings, realStore ModuleStoreI) error
	SaveDataSource(ctx context.Context, projectId string, datasource *module_model.DataSource, realStore ModuleStoreI) error
	RemoveDataSource(ctx context.Context, projectId string, dbAlias string, realStore ModuleStoreI) error
	GetDataSource(ctx context.Context, projectId string, dbAlias string) (datasource *module_model.DataSource, err error)
	GetDataSources(ctx context.Context, projectId string) (datasources map[string]*module_model.DataSource, err error)
	UpdateSchemaTables(ctx context.Context, projectId string, dbAlias string, realStore ModuleStoreI) (datasource *module_model.DataSource, err error)
	AddSchemaTable(ctx context.Context, projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (tables map[string]interface{}, err error)
	SaveSchemaTable(ctx context.Context, projectId string, dbAlias string, tableName string, tableObj map[string]common_types.TableColsMetaData, realStore ModuleStoreI) (err error)
	GetTableSecurity(ctx context.Context, projectId string, dbAlias string, tableName string) (transformRules module_model.SecurityRules, err error)
	SaveTableSecurity(ctx context.Context, projectId string, dbAlias string, tableName string, securityRules module_model.SecurityRules, realStore ModuleStoreI) (err error)
	RemoveTableSecurity(ctx context.Context, projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (err error)
	SaveTableTransformation(ctx context.Context, projectId string, dbAlias string, tableName string, transformRules module_model.TransformRules, realStore ModuleStoreI) (err error)
	SaveColumnMasking(ctx context.Context, projectId string, dbAlias string, tableName string, colName string, columnMasking common_types.ColumnMasking, realStore ModuleStoreI) (err error)
	GetTableTransformation(ctx context.Context, projectId string, dbAlias string, tableName string) (transformRules module_model.TransformRules, err error)
	DropSchemaTable(ctx context.Context, projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (err error)
	RemoveSchemaTable(ctx context.Context, projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (tables map[string]interface{}, err error)
	SaveMyQuery(ctx context.Context, projectId string, queryName string, queryType string, dbAlias string, query string, vars map[string]interface{}, realStore ModuleStoreI, cols string, securityRule security_rule.SecurityRule) error
	RemoveMyQuery(ctx context.Context, projectId string, queryName string, realStore ModuleStoreI) error
	GetMyQuery(ctx context.Context, projectId string, queryName string) (myquery module_model.MyQuery, err error)
	GetMyQueries(ctx context.Context, projectId string, queryType string) (myqueries map[string]module_model.MyQuery, err error)
	GetMyQueriesNames(ctx context.Context, projectId string) (myqueries []string, err error)
	AddSchemaJoin(ctx context.Context, projectId string, dbAlias string, tj *module_model.TableJoins, realStore ModuleStoreI) (tables map[string]interface{}, err error)
	RemoveSchemaJoin(ctx context.Context, projectId string, dbAlias string, tj *module_model.TableJoins, realStore ModuleStoreI) (tables map[string]interface{}, err error)
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
	logs.WithContext(ctx).Debug("SaveProject - Start")
	//TODO to handle edit project once new project attributes are finalized
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
		/*if project.Storages == nil {
			project.Storages = make(map[string]storage.StorageI)
		}*/
		ms.Projects[projectId] = project
		if persist == true {
			logs.WithContext(ctx).Info("SaveStore called from SaveProject")
			return realStore.SaveStore(ctx, projectId, "", realStore)
		} else {
			return nil
		}
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " already exists"))
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
		return errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}

func (ms *ModuleStore) GetExtendedProjectConfig(ctx context.Context, projectId string, realStore ModuleStoreI) (ePrj module_model.ExtendedProject, err error) {
	logs.WithContext(ctx).Debug("GetExtendedProjectConfig - Start")
	ePrj = module_model.ExtendedProject{}
	if prj, ok := ms.Projects[projectId]; ok {
		ePrj.Variables, err = realStore.FetchVars(ctx, projectId)
		ePrj.SecretManager, err = realStore.FetchSm(ctx, projectId)
		ePrj.ProjectId = prj.ProjectId
		ePrj.DataSources = prj.DataSources
		ePrj.ProjectSettings = prj.ProjectSettings
		ePrj.MyQueries = prj.MyQueries
		return ePrj, nil
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
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
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return nil, err
	}
}

func (ms *ModuleStore) GetProjectSettingsObject(ctx context.Context, projectId string) (pc module_model.ProjectSettings, err error) {
	logs.WithContext(ctx).Debug("GetProjectConfigObject - Start")
	if _, ok := ms.Projects[projectId]; ok {
		return ms.Projects[projectId].ProjectSettings, nil
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return pc, err
	}
}

func (ms *ModuleStore) GetProjectList(ctx context.Context) []map[string]interface{} {
	logs.WithContext(ctx).Debug("GetProjectList - Start")
	projects := make([]map[string]interface{}, len(ms.Projects))
	i := 0
	for k := range ms.Projects {
		project := make(map[string]interface{})
		project["project_name"] = k
		//project["lastUpdateDate"] = time.Now()
		projects[i] = project
		i++
	}
	return projects
}

func (ms *ModuleStore) SetDataSourceConnections(ctx context.Context, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("SetDataSourceConnections - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	for _, prj := range ms.Projects {
		for _, datasource := range prj.DataSources {
			i := ds.GetSqlMaker(datasource.DbName)
			if i != nil {
				// making clone to replace variables with actual values to create DB connection
				datasourceClone, err := ms.GetDatasourceCloneObject(ctx, prj.ProjectId, datasource, realStore)
				if err != nil {
					return err
				}
				err = i.CreateConn(ctx, datasourceClone)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
				}
				//setting DB connection object in actual store
				if datasource.DbName == "iceberg" {
					if datasourceClone.IcebergConfig.S3TablesConfig.Session != nil {
						datasource.IcebergConfig.S3TablesConfig.Session = datasourceClone.IcebergConfig.S3TablesConfig.Session
						datasource.ConStatus = true
						datasource.IcebergConfig.S3TablesConfig.BucketArn = datasourceClone.IcebergConfig.S3TablesConfig.BucketArn
					} else {
						datasource.ConStatus = false
					}
					if datasourceClone.SqlEngineType != "" {
						datasource.SqlEngineType = datasourceClone.SqlEngineType
						datasource.SqlEngine = sqlengine.GetSQLEngine(datasourceClone.SqlEngineType)
					}
				} else {
					datasource.Con = datasourceClone.Con
					datasource.ConStatus = datasourceClone.ConStatus
				}

			} else {
				err = errors.New(fmt.Sprint(datasource.DbName, " not found"))
				logs.WithContext(ctx).Error(err.Error())
			}
		}
	}
	return nil
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

func (ms *ModuleStore) GetDatasourceCloneObject(ctx context.Context, projectId string, datasource *module_model.DataSource, s ModuleStoreI) (datasourceClone *module_model.DataSource, err error) {
	logs.WithContext(ctx).Debug("GetDatasourceCloneObject - Start")
	datasourceObjJson, datasourceObjJsonErr := json.Marshal(datasource)
	if datasourceObjJsonErr != nil {
		err = errors.New(fmt.Sprint("error while cloning datasourceObj (marshal)"))
		logs.WithContext(ctx).Error(err.Error())
		logs.WithContext(ctx).Error(datasourceObjJsonErr.Error())
		return
	}
	datasourceObjJson = s.ReplaceVariables(ctx, projectId, datasourceObjJson, nil)

	iCloneI := reflect.New(reflect.TypeOf(datasource))
	datasourceObjCloneErr := json.Unmarshal(datasourceObjJson, iCloneI.Interface())
	if datasourceObjCloneErr != nil {
		err = errors.New(fmt.Sprint("error while cloning datasourceObj(unmarshal)"))
		logs.WithContext(ctx).Error(err.Error())
		logs.WithContext(ctx).Error(datasourceObjCloneErr.Error())
		return
	}
	return iCloneI.Elem().Interface().(*module_model.DataSource), nil
}

func (ms *ModuleStore) SaveDataSource(ctx context.Context, projectId string, datasource *module_model.DataSource, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("SaveDataSource - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()

	if datasource.DbType == "iceberg" && datasource.IcebergConfig.CatalogType != "s3tables" {
		return errors.New("currently only s3tables is supported for iceberg")
	}

	err := ms.checkProjectExists(ctx, projectId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}

	if ms.Projects[projectId].DataSources == nil {
		ms.Projects[projectId].DataSources = make(map[string]*module_model.DataSource)
	}

	if ms.Projects[projectId].DataSources[datasource.DbAlias] != nil {
		datasource.SchemaTables = ms.Projects[projectId].DataSources[datasource.DbAlias].SchemaTables
		datasource.SchemaTablesSecurity = ms.Projects[projectId].DataSources[datasource.DbAlias].SchemaTablesSecurity
		datasource.TableJoins = ms.Projects[projectId].DataSources[datasource.DbAlias].TableJoins
		datasource.DbSecurityRules = ms.Projects[projectId].DataSources[datasource.DbAlias].DbSecurityRules
		datasource.SchemaTablesTransformation = ms.Projects[projectId].DataSources[datasource.DbAlias].SchemaTablesTransformation
	}
	ms.Projects[projectId].DataSources[datasource.DbAlias] = datasource

	sqlMaker := ds.GetSqlMaker(datasource.DbName)
	datasource.DbType = ds.GetDbType(datasource.DbName)

	// making clone to replace variables with actual values to create DB connection
	datasourceClone, err := ms.GetDatasourceCloneObject(ctx, projectId, datasource, realStore)
	if err != nil {
		return err
	}
	if sqlMaker != nil {
		err = sqlMaker.CreateConn(ctx, datasourceClone)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		//setting DB connection object in actual store
		if datasource.DbName == "iceberg" {
			if datasourceClone.IcebergConfig.S3TablesConfig.Session != nil {
				datasource.IcebergConfig.S3TablesConfig.Session = datasourceClone.IcebergConfig.S3TablesConfig.Session
				datasource.ConStatus = true
				datasource.IcebergConfig.S3TablesConfig.BucketArn = datasourceClone.IcebergConfig.S3TablesConfig.BucketArn
			} else {
				datasource.ConStatus = false
			}
			if datasourceClone.SqlEngineType != "" {
				datasource.SqlEngineType = datasourceClone.SqlEngineType
				datasource.SqlEngine = datasourceClone.SqlEngine
			}
		} else {
			datasource.Con = datasourceClone.Con
			datasource.ConStatus = datasourceClone.ConStatus
		}
	}
	logs.WithContext(ctx).Info("SaveStore called from SaveDataSource")
	return realStore.SaveStore(ctx, projectId, "", realStore)
}

func (ms *ModuleStore) RemoveDataSource(ctx context.Context, projectId string, dbAlias string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveDataSource - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	err := ms.checkProjectDataSourceExists(ctx, projectId, dbAlias)
	if err != nil {
		return err
	}
	delete(ms.Projects[projectId].DataSources, dbAlias)
	logs.WithContext(ctx).Info("SaveStore called from RemoveDataSource")
	return realStore.SaveStore(ctx, projectId, "", realStore)
}

func (ms *ModuleStore) GetDataSource(ctx context.Context, projectId string, dbAlias string) (datasource *module_model.DataSource, err error) {
	logs.WithContext(ctx).Debug("GetDataSource - Start")
	err = ms.checkProjectDataSourceExists(ctx, projectId, dbAlias)
	if err != nil {
		return nil, err
	}
	return ms.Projects[projectId].DataSources[dbAlias], nil
}

func (ms *ModuleStore) GetDataSources(ctx context.Context, projectId string) (datasources map[string]*module_model.DataSource, err error) {
	logs.WithContext(ctx).Debug("GetDataSources - Start")
	err = ms.checkProjectExists(ctx, projectId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	return ms.Projects[projectId].DataSources, nil
}

func (ms *ModuleStore) UpdateSchemaTables(ctx context.Context, projectId string, dbAlias string, realStore ModuleStoreI) (datasource *module_model.DataSource, err error) {
	logs.WithContext(ctx).Debug("UpdateSchemaTables - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	var tmpList []string
	err = ms.checkProjectDataSourceExists(ctx, projectId, dbAlias)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

	datasource = ms.Projects[projectId].DataSources[dbAlias]
	sr := ds.GetSqlMaker(datasource.DbName)
	err = sr.GetTableList(ctx, sr.GetTableMetaDataSQL(ctx), datasource, sr)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

	for k, ot := range datasource.OtherTables {
		if _, ok := datasource.SchemaTables[k]; ok {
			datasource.SchemaTables[k] = ot
			tmpList = append(tmpList, k)
		}
	}
	for i := 0; i < len(tmpList); i++ {
		delete(datasource.OtherTables, tmpList[i])
	}
	logs.WithContext(ctx).Info("SaveStore called from UpdateSchemaTables")
	return datasource, realStore.SaveStore(ctx, projectId, "", realStore)
}
func (ms *ModuleStore) AddSchemaTable(ctx context.Context, projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (tables map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("AddSchemaTable - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	err = ms.checkProjectDataSourceExists(ctx, projectId, dbAlias)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	datasource := ms.Projects[projectId].DataSources[dbAlias]
	if val, ok := datasource.OtherTables[tableName]; ok {
		if datasource.SchemaTables == nil {
			datasource.SchemaTables = make(map[string]map[string]common_types.TableColsMetaData)
		}
		datasource.SchemaTables[tableName] = val
		delete(datasource.OtherTables, tableName)
		for k, kv := range val {
			if kv.FkConstraintName != "" {
				tjJoinKey := fmt.Sprint(kv.FkTblSchema, ".", kv.FkTblName, "___", tableName)
				tj := module_model.TableJoins{}
				if v, ok := datasource.TableJoins[tjJoinKey]; ok {
					tj = *v
				}
				//else {
				//	tjJoinKey1 := fmt.Sprint(tableName, "___", kv.FkTblName) //swapping the table names and check again
				//	if v, ok := datasource.TableJoins[tjJoinKey1]; ok {
				//		tj = v
				//	}
				//}
				tj.Table1Name = fmt.Sprint(kv.FkTblSchema, ".", kv.FkTblName)
				tj.Table1Cols = append(tj.Table1Cols, kv.FkColName)
				tj.Table2Name = tableName
				tj.Table2Cols = append(tj.Table2Cols, k)
				tj.IsCustom = false
				if _, ok := datasource.SchemaTables[tj.Table1Name]; ok {
					tj.IsActive = true
				} else {
					tj.IsActive = false
				}
				if datasource.TableJoins == nil {
					datasource.TableJoins = make(map[string]*module_model.TableJoins)
				}
				datasource.TableJoins[tjJoinKey] = &tj
			}
		}
		for k, v := range datasource.TableJoins {
			tempStr := strings.SplitN(k, "___", 2)
			if tempStr[0] == tableName {
				v.IsActive = true
				//datasource.TableJoins[k].IsActive = true
			}
		}
		logs.WithContext(ctx).Info("SaveStore called from AddSchemaTable")
		err = realStore.SaveStore(ctx, projectId, "", realStore)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		} else {
			return map[string]interface{}{"SchemaTables": datasource.SchemaTables, "OtherTables": datasource.OtherTables}, nil
		}
	} else {
		err = errors.New(fmt.Sprintf(tableName, " not found to add in the schema"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return nil, err
	}
}
func (ms *ModuleStore) RemoveSchemaTable(ctx context.Context, projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (tables map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("RemoveSchemaTable - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	err = ms.checkProjectDataSourceExists(ctx, projectId, dbAlias)
	if err != nil {
		return nil, err
	}
	datasource := ms.Projects[projectId].DataSources[dbAlias]
	if val, ok := datasource.SchemaTables[tableName]; ok {
		if datasource.OtherTables == nil {
			datasource.OtherTables = make(map[string]map[string]common_types.TableColsMetaData)
		}
		datasource.OtherTables[tableName] = val
		delete(datasource.SchemaTables, tableName)
		for k, v := range datasource.TableJoins {
			tempStr := strings.SplitN(k, "___", 2)
			if tempStr[1] == tableName {
				delete(datasource.TableJoins, k)
			} else if tempStr[0] == tableName {
				v.IsActive = false
			}
		}
		logs.WithContext(ctx).Info("SaveStore called from RemoveSchemaTable")
		err = realStore.SaveStore(ctx, projectId, "", realStore)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		} else {
			return map[string]interface{}{"SchemaTables": datasource.SchemaTables, "OtherTables": datasource.OtherTables}, nil
		}
	} else {
		err = errors.New(fmt.Sprintf(tableName, " not found to add in the schema"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return nil, err
	}
}

func (ms *ModuleStore) AddSchemaJoin(ctx context.Context, projectId string, dbAlias string, tj *module_model.TableJoins, realStore ModuleStoreI) (tables map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("AddSchemaJoin - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	err = ms.checkProjectDataSourceExists(ctx, projectId, dbAlias)
	if err != nil {
		return nil, err
	}
	datasource := ms.Projects[projectId].DataSources[dbAlias]
	datasource.AddTableJoins(ctx, tj)
	logs.WithContext(ctx).Info("SaveStore called from AddSchemaJoin")
	err = realStore.SaveStore(ctx, projectId, "", realStore)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	} else {
		return map[string]interface{}{"TableJoins": datasource.TableJoins}, nil
	}
}
func (ms *ModuleStore) RemoveSchemaJoin(ctx context.Context, projectId string, dbAlias string, tj *module_model.TableJoins, realStore ModuleStoreI) (tables map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("RemoveSchemaJoin - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	err = ms.checkProjectDataSourceExists(ctx, projectId, dbAlias)
	if err != nil {
		return nil, err
	}
	datasource := ms.Projects[projectId].DataSources[dbAlias]
	datasource.RemoveTableJoins(ctx, tj)
	logs.WithContext(ctx).Info("SaveStore called from RemoveSchemaJoin")
	err = realStore.SaveStore(ctx, projectId, "", realStore)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	} else {
		return map[string]interface{}{"TableJoins": datasource.TableJoins}, nil
	}
}

func (ms *ModuleStore) SaveMyQuery(ctx context.Context, projectId string, queryName string, queryType string, dbAlias string, query string, vars map[string]interface{}, realStore ModuleStoreI, cols string, securityRule security_rule.SecurityRule) error {
	logs.WithContext(ctx).Debug("SaveMyQuery - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()

	if _, ok := ms.Projects[projectId]; ok {
		readWrite := ""
		queryFirstWord := strings.ToUpper(strings.Split(query, " ")[0])
		if queryFirstWord == Q_SELECT || queryFirstWord == Q_WITH {
			readWrite = Q_SELECT
		} else if queryFirstWord == Q_INSERT {
			readWrite = Q_INSERT
		} else if queryFirstWord == Q_UPDATE {
			readWrite = Q_UPDATE
		} else if queryFirstWord == Q_DELETE {
			readWrite = Q_DELETE
		}
		excelStyles := make(map[string]eru_writes.CellFormatter)
		if excelStylesData, excelStylesOk := vars["excel_styles"]; excelStylesOk {
			// Marshal and unmarshal to convert interface{} to proper type
			excelStylesBytes, err := json.Marshal(excelStylesData)
			if err == nil {
				err = json.Unmarshal(excelStylesBytes, &excelStyles)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
				}
			} else {
				logs.WithContext(ctx).Error(err.Error())
			}
			delete(vars, "excel_styles")
		}
		columns := make(map[string]eru_writes.ColumnarSettings)
		if columnsData, columnsOk := vars["columns"]; columnsOk {
			columnsBytes, err := json.Marshal(columnsData)
			if err == nil {
				err = json.Unmarshal(columnsBytes, &columns)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
				}
			} else {
				logs.WithContext(ctx).Error(err.Error())
			}
			delete(vars, "columns")
		}
		pivotConfig := make(map[string]eru_writes.PivotTableConfig)
		if pivotConfigData, pivotConfigOk := vars["pivot_config"]; pivotConfigOk {
			pivotConfigBytes, err := json.Marshal(pivotConfigData)
			if err == nil {
				err = json.Unmarshal(pivotConfigBytes, &pivotConfig)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
				}
			} else {
				logs.WithContext(ctx).Error(err.Error())
			}
			delete(vars, "pivot_config")
		}

		myquery := module_model.MyQuery{
			QueryName:    queryName,
			Query:        query,
			Vars:         vars,
			QueryType:    queryType,
			DBAlias:      dbAlias,
			ReadWrite:    readWrite,
			Cols:         cols,
			SecurityRule: securityRule,
			ExcelStyles:  excelStyles,
			Columns:      columns,
			PivotConfig:  pivotConfig,
		}
		if ms.Projects[projectId].MyQueries == nil {
			ms.Projects[projectId].MyQueries = make(map[string]*module_model.MyQuery)
		}
		ms.Projects[projectId].MyQueries[queryName] = &myquery
		logs.WithContext(ctx).Info(fmt.Sprint("SaveStore called from SaveMyQuery ", queryName))
		return realStore.SaveStore(ctx, projectId, "", realStore)
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " not found"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return err
	}
}

func (ms *ModuleStore) RemoveMyQuery(ctx context.Context, projectId string, queryName string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveMyQuery - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if _, ok := ms.Projects[projectId]; ok {
		if ms.Projects[projectId].MyQueries == nil {
			return errors.New(fmt.Sprint("Query ", queryName, " not found"))
		}
		if _, ok = ms.Projects[projectId].MyQueries[queryName]; ok {
			delete(ms.Projects[projectId].MyQueries, queryName)
			logs.WithContext(ctx).Info(fmt.Sprint("SaveStore called from RemoveMyQuery ", queryName))
			return realStore.SaveStore(ctx, projectId, "", realStore)
		} else {
			err := errors.New(fmt.Sprint("Query ", queryName, " not found"))
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
			}
			return err
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " not found"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return err
	}
}

func (ms *ModuleStore) GetMyQuery(ctx context.Context, projectId string, queryName string) (myquery module_model.MyQuery, err error) {
	logs.WithContext(ctx).Debug("GetMyQuery - Start")
	if _, ok := ms.Projects[projectId]; ok {
		if ms.Projects[projectId].MyQueries == nil {
			return module_model.MyQuery{}, errors.New(fmt.Sprint("Query ", queryName, " not found"))
		}
		if myqueryPointer, ok := ms.Projects[projectId].MyQueries[queryName]; ok {
			myquery = *myqueryPointer
			return myquery, nil
		} else {
			err = errors.New(fmt.Sprint("Query ", queryName, " not found"))
			if err != nil {
				logs.WithContext(ctx).Info(err.Error())
			}
			return module_model.MyQuery{}, err
		}
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " not found"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return module_model.MyQuery{}, err
	}
}

func (ms *ModuleStore) GetMyQueries(ctx context.Context, projectId string, queryType string) (myqueries map[string]module_model.MyQuery, err error) {
	logs.WithContext(ctx).Debug("GetMyQueries - Start")
	if _, ok := ms.Projects[projectId]; ok {
		if ms.Projects[projectId].MyQueries == nil {
			return make(map[string]module_model.MyQuery), nil
		} else {
			queriesToReturn := make(map[string]module_model.MyQuery)
			for k, mq := range ms.Projects[projectId].MyQueries {
				if strings.EqualFold(mq.QueryType, queryType) {
					queriesToReturn[k] = *mq
				}
			}
			return queriesToReturn, nil
		}
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " not found"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return nil, err
	}
}

func (ms *ModuleStore) GetMyQueriesNames(ctx context.Context, projectId string) (myqueries []string, err error) {
	logs.WithContext(ctx).Debug("GetMyQueriesNames - Start")
	if _, ok := ms.Projects[projectId]; ok {
		if ms.Projects[projectId].MyQueries == nil {
			return
		} else {
			for k, _ := range ms.Projects[projectId].MyQueries {
				myqueries = append(myqueries, k)
			}
			return
		}
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " not found"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return nil, err
	}
}

// CompareTableStructures compares old and new table structures and returns the differences
func (ms *ModuleStore) CompareTableStructures(ctx context.Context, oldTableObj, newTableObj map[string]common_types.TableColsMetaData) common_types.TableStructureDiff {
	diff := common_types.TableStructureDiff{
		NewColumns:      make(map[string]common_types.TableColsMetaData),
		DroppedColumns:  []string{},
		ModifiedColumns: make(map[string]common_types.ColumnChange),
	}

	// Find new columns and modified columns
	for colName, newCol := range newTableObj {
		if oldCol, exists := oldTableObj[colName]; exists {
			// Column exists in both, check for modifications
			changedFields := ms.getChangedFields(oldCol, newCol)
			if len(changedFields) > 0 {
				diff.ModifiedColumns[colName] = common_types.ColumnChange{
					ChangeType:    common_types.ChangeTypeModifyColumn,
					ColumnName:    colName,
					OldColumn:     &oldCol,
					NewColumn:     &newCol,
					ChangedFields: changedFields,
				}
			}
		} else {
			// New column
			diff.NewColumns[colName] = newCol
		}
	}

	// Find dropped columns
	for colName := range oldTableObj {
		if _, exists := newTableObj[colName]; !exists {
			diff.DroppedColumns = append(diff.DroppedColumns, colName)
		}
	}

	return diff
}

// getChangedFields compares two TableColsMetaData structs and returns the names of changed fields
func (ms *ModuleStore) getChangedFields(oldCol, newCol common_types.TableColsMetaData) []string {
	changedFields := []string{}

	// Compare all relevant fields
	if oldCol.DataType != newCol.DataType {
		changedFields = append(changedFields, "DataType")
	}
	if oldCol.PrimaryKey != newCol.PrimaryKey {
		changedFields = append(changedFields, "PrimaryKey")
	}
	if oldCol.IsUnique != newCol.IsUnique {
		changedFields = append(changedFields, "IsUnique")
	}
	if oldCol.PkConstraintName != newCol.PkConstraintName {
		changedFields = append(changedFields, "PkConstraintName")
	}
	if oldCol.UqConstraintName != newCol.UqConstraintName {
		changedFields = append(changedFields, "UqConstraintName")
	}
	if oldCol.IsNullable != newCol.IsNullable {
		changedFields = append(changedFields, "IsNullable")
	}
	if oldCol.DefaultValue != newCol.DefaultValue {
		changedFields = append(changedFields, "DefaultValue")
	}
	if oldCol.AutoIncrement != newCol.AutoIncrement {
		changedFields = append(changedFields, "AutoIncrement")
	}
	if oldCol.CharMaxLength != newCol.CharMaxLength {
		changedFields = append(changedFields, "CharMaxLength")
	}
	if oldCol.NumericPrecision != newCol.NumericPrecision {
		changedFields = append(changedFields, "NumericPrecision")
	}
	if oldCol.NumericScale != newCol.NumericScale {
		changedFields = append(changedFields, "NumericScale")
	}
	if oldCol.DatetimePrecision != newCol.DatetimePrecision {
		changedFields = append(changedFields, "DatetimePrecision")
	}
	if oldCol.FkConstraintName != newCol.FkConstraintName {
		changedFields = append(changedFields, "FkConstraintName")
	}
	if oldCol.FkDeleteRule != newCol.FkDeleteRule {
		changedFields = append(changedFields, "FkDeleteRule")
	}
	if oldCol.FkTblSchema != newCol.FkTblSchema {
		changedFields = append(changedFields, "FkTblSchema")
	}
	if oldCol.FkTblName != newCol.FkTblName {
		changedFields = append(changedFields, "FkTblName")
	}
	if oldCol.FkColName != newCol.FkColName {
		changedFields = append(changedFields, "FkColName")
	}
	if oldCol.ColumnMasking.MaskingType != newCol.ColumnMasking.MaskingType {
		changedFields = append(changedFields, "ColumnMasking")
	}

	return changedFields
}

// getFieldValue returns the value of a specific field from a TableColsMetaData struct
func (ms *ModuleStore) getFieldValue(col *common_types.TableColsMetaData, fieldName string) interface{} {
	if col == nil {
		return nil
	}

	switch fieldName {
	case "DataType":
		return col.DataType
	case "OwnDataType":
		return col.OwnDataType
	case "PrimaryKey":
		return col.PrimaryKey
	case "IsUnique":
		return col.IsUnique
	case "PkConstraintName":
		return col.PkConstraintName
	case "UqConstraintName":
		return col.UqConstraintName
	case "IsNullable":
		return col.IsNullable
	case "ColPosition":
		return col.ColPosition
	case "DefaultValue":
		return col.DefaultValue
	case "AutoIncrement":
		return col.AutoIncrement
	case "CharMaxLength":
		return col.CharMaxLength
	case "NumericPrecision":
		return col.NumericPrecision
	case "NumericScale":
		return col.NumericScale
	case "DatetimePrecision":
		return col.DatetimePrecision
	case "FkConstraintName":
		return col.FkConstraintName
	case "FkDeleteRule":
		return col.FkDeleteRule
	case "FkTblSchema":
		return col.FkTblSchema
	case "FkTblName":
		return col.FkTblName
	case "FkColName":
		return col.FkColName
	case "ColumnMasking":
		return col.ColumnMasking.MaskingType
	default:
		return "unknown field"
	}
}

func (ms *ModuleStore) SaveSchemaTable(ctx context.Context, projectId string, dbAlias string, tableName string, tableObj map[string]common_types.TableColsMetaData, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveSchemaTable - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	tableExists := false
	oldTableObj := make(map[string]common_types.TableColsMetaData)
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if to, ok := db.SchemaTables[tableName]; ok {
				tableExists = true
				oldTableObj = to
				logs.WithContext(ctx).Info("table exists in Schema table - to alter")
			} else if to, ok := db.OtherTables[tableName]; ok {
				tableExists = true
				oldTableObj = to
				logs.WithContext(ctx).Info("table exists in Other table - to alter")
			}
			if tableExists {
				// Compare table structures to identify changes
				diff := ms.CompareTableStructures(ctx, oldTableObj, tableObj)

				// Log the changes for debugging
				logs.WithContext(ctx).Info(fmt.Sprintf("Table structure changes detected for %s:", tableName))
				logs.WithContext(ctx).Info(fmt.Sprintf("  New columns: %d", len(diff.NewColumns)))
				logs.WithContext(ctx).Info(fmt.Sprintf("  Dropped columns: %d", len(diff.DroppedColumns)))
				logs.WithContext(ctx).Info(fmt.Sprintf("  Modified columns: %d", len(diff.ModifiedColumns)))

				// Log detailed changes
				for colName, newCol := range diff.NewColumns {
					logs.WithContext(ctx).Info(fmt.Sprintf("  NEW COLUMN: %s (%s)", colName, newCol.DataType))
				}

				for _, colName := range diff.DroppedColumns {
					logs.WithContext(ctx).Info(fmt.Sprintf("  DROPPED COLUMN: %s", colName))
				}

				for colName, change := range diff.ModifiedColumns {
					logs.WithContext(ctx).Info(fmt.Sprintf("  MODIFIED COLUMN: %s - Changed fields: %v", colName, change.ChangedFields))
					for _, field := range change.ChangedFields {
						logs.WithContext(ctx).Info(fmt.Sprintf("    %s: %v -> %v", field,
							ms.getFieldValue(change.OldColumn, field),
							ms.getFieldValue(change.NewColumn, field)))
					}
				}

				// TODO: Generate and execute ALTER TABLE DDL based on the changes
				// This is where you would create the appropriate SQL DDL statements
				// based on the diff.NewColumns, diff.DroppedColumns, and diff.ModifiedColumns

				if len(diff.NewColumns) > 0 || len(diff.DroppedColumns) > 0 || len(diff.ModifiedColumns) > 0 {
					logs.WithContext(ctx).Info("Table structure changes detected - ALTER TABLE DDL generation not yet implemented")
					// TODO: Implement DDL generation and execution
					// err = ms.generateAndExecuteAlterTableDDL(ctx, tableName, diff, db)
				} else {
					logs.WithContext(ctx).Info("No table structure changes detected")
				}
			} else {
				//create table
				sr := ds.GetSqlMaker(db.DbName)
				query, err := sr.MakeCreateTableSQL(ctx, tableName, tableObj)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				res, err := sr.ExecutePreparedQuery(ctx, query, db)
				_ = res
				if err != nil {
					return err
				}
				//TODO to change store
			}
		} else {
			err = errors.New(fmt.Sprint("Datasource ", dbAlias, " not found"))
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
			}
			return err
		}
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " not found"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return err
	}
	return err
}

func (ms *ModuleStore) SaveTableSecurity(ctx context.Context, projectId string, dbAlias string, tableName string, securityRules module_model.SecurityRules, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveTableSecurity - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if _, ok := db.SchemaTables[tableName]; ok || securityRules.IsTemplate {
				if db.SchemaTablesSecurity == nil {
					db.SchemaTablesSecurity = make(map[string]module_model.SecurityRules)
				}
				db.SchemaTablesSecurity[tableName] = securityRules
			} else {
				err = errors.New(fmt.Sprint("Table ", tableName, " not found"))
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
				}
				return err
			}
		} else {
			err = errors.New(fmt.Sprint("Datasource ", dbAlias, " not found"))
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
			}
			return err
		}
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " not found"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return err
	}
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	logs.WithContext(ctx).Info(fmt.Sprint("SaveStore called from SaveTableSecurity ", tableName))
	return realStore.SaveStore(ctx, projectId, "", realStore)
}

func (ms *ModuleStore) RemoveTableSecurity(ctx context.Context, projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("RemoveTableSecurity - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if _, ok := db.SchemaTablesSecurity[tableName]; ok {
				delete(db.SchemaTablesSecurity, tableName)
			} else {
				err = errors.New(fmt.Sprint("Table ", tableName, " not found"))
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
				}
				return err
			}
		} else {
			err = errors.New(fmt.Sprint("Datasource ", dbAlias, " not found"))
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
			}
			return err
		}
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " not found"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return err
	}
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	logs.WithContext(ctx).Info(fmt.Sprint("SaveStore called from SaveTableSecurity ", tableName))
	return realStore.SaveStore(ctx, projectId, "", realStore)
}

func (ms *ModuleStore) SaveColumnMasking(ctx context.Context, projectId string, dbAlias string, tableName string, colName string, columnMasking common_types.ColumnMasking, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveColumnMasking - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if tb, ok := db.SchemaTables[tableName]; ok {
				if cl, ok := tb[colName]; ok {
					cl.ColumnMasking = columnMasking
					db.SchemaTables[tableName][colName] = cl
				} else {
					err = errors.New(fmt.Sprint("Column ", colName, " not found"))
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
			} else {
				err = errors.New(fmt.Sprint("Table ", tableName, " not found"))
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
		} else {
			err = errors.New(fmt.Sprint("Datasource ", dbAlias, " not found"))
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
			}
			return err
		}
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " not found"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return err
	}
	logs.WithContext(ctx).Info(fmt.Sprint("SaveStore called from SaveColumnMasking ", tableName))
	return realStore.SaveStore(ctx, projectId, "", realStore)
}
func (ms *ModuleStore) SaveTableTransformation(ctx context.Context, projectId string, dbAlias string, tableName string, transformRules module_model.TransformRules, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveTableTransformation - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if _, ok := db.SchemaTables[tableName]; ok {
				if db.SchemaTablesTransformation == nil {
					db.SchemaTablesTransformation = make(map[string]module_model.TransformRules)
				}
				db.SchemaTablesTransformation[tableName] = transformRules
			} else {
				err = errors.New(fmt.Sprint("Table ", tableName, " not found"))
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
		} else {
			err = errors.New(fmt.Sprint("Datasource ", dbAlias, " not found"))
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
			}
			return err
		}
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " not found"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return err
	}
	logs.WithContext(ctx).Info(fmt.Sprint("SaveStore called from SaveTableTransformation ", tableName))
	return realStore.SaveStore(ctx, projectId, "", realStore)
}

func (ms *ModuleStore) GetTableTransformation(ctx context.Context, projectId string, dbAlias string, tableName string) (transformRules module_model.TransformRules, err error) {
	logs.WithContext(ctx).Debug("GetTableTransformation - Start")
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if _, ok := db.SchemaTables[tableName]; ok {
				transformRules = db.SchemaTablesTransformation[tableName]
			} else if _, ok := prj.MyQueries[tableName]; ok {
				//do nothing as there are no transform rule feature for my query TODO check feasibility
			} else {
				err = errors.New(fmt.Sprint("Table ", tableName, " not found"))
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
				}
				return transformRules, err
			}
		} else {
			err = errors.New(fmt.Sprint("Datasource ", dbAlias, " not found"))
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
			}
			return transformRules, err
		}
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " not found"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return transformRules, err
	}
	return
}

func (ms *ModuleStore) GetTableSecurity(ctx context.Context, projectId string, dbAlias string, tableName string) (securityRules module_model.SecurityRules, err error) {
	logs.WithContext(ctx).Debug("GetTableSecurity - Start")
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if sr, srOk := db.SchemaTablesSecurity[tableName]; srOk {
				securityRules = sr
			} else {
				err = errors.New(fmt.Sprint("Security Rule not defined for  ", tableName))
				return
			}
			if !securityRules.IsTemplate {
				if _, ok := db.SchemaTables[tableName]; ok {
					//do nothing
				} else if _, ok := prj.MyQueries[tableName]; ok {
					securityRules.Query = prj.MyQueries[tableName].SecurityRule
				} else {
					err = errors.New(fmt.Sprint("Table ", tableName, " not found"))
					if err != nil {
						logs.WithContext(ctx).Error(err.Error())
					}
					return securityRules, err
				}
			}
		} else {
			err = errors.New(fmt.Sprint("Datasource ", dbAlias, " not found"))
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
			}
			return securityRules, err
		}
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " not found"))
		logs.WithContext(ctx).Error(err.Error())
		return securityRules, err
	}
	return
}

func (ms *ModuleStore) DropSchemaTable(ctx context.Context, projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("DropSchemaTable - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	tableExists := false
	//TODO - to check if drop is allowed
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if _, ok := db.SchemaTables[tableName]; ok {
				tableExists = true
				delete(db.SchemaTables, tableName)
				logs.WithContext(ctx).Info("table exists in Schema table - to alter")
			} else if _, ok := db.OtherTables[tableName]; ok {
				tableExists = true
				delete(db.OtherTables, tableName)
				logs.WithContext(ctx).Info("table exists in Other table - to alter")
			}
			if tableExists {
				//drop table
				sr := ds.GetSqlMaker(db.DbName)
				query, err := sr.MakeDropTableSQL(ctx, tableName)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				res, err := sr.ExecutePreparedQuery(ctx, query, db)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				_ = res
				//TODO to change store
			} else {
				err = errors.New(fmt.Sprint("Table ", tableName, " does not exists"))
				logs.WithContext(ctx).Error(err.Error())
			}
		} else {
			err = errors.New(fmt.Sprint("Datasource ", dbAlias, " not found"))
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " not found"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return err
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
		//myStore.SetStoreTenantTableName(StoreTenantTableName)
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
		err = json.Unmarshal(storeBytes, myStore)
		if err != nil {
			logs.WithContext(ctx).Warn(err.Error())
		}
		err = myStore.SetStoreFromBytes(ctx, storeBytes, myStore)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
	} else {
		logs.WithContext(ctx).Error(err.Error())
	}
	//s.Store = myStore
	return myStore, err
}
