package module_store

import (
	"context"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/ds"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-security-rule/security_rule"
	"github.com/eru-tech/eru/eru-store/store"
	"log"
	"strings"
)

const (
	Q_SELECT = "SELECT"
	Q_WITH   = "WITH"
	Q_INSERT = "INSERT"
	Q_UPDATE = "UPDATE"
	Q_DELETE = "DELETE"
)

type StoreHolder struct {
	Store ModuleStoreI
}
type ModuleStoreI interface {
	store.StoreI
	SaveProject(ctx context.Context, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error
	GetProjectConfig(ctx context.Context, projectId string) (*module_model.Project, error)
	GetProjectConfigObject(ctx context.Context, projectId string) (pc module_model.ProjectConfig, err error)
	GetProjectList(ctx context.Context) []map[string]interface{}
	SetDataSourceConnections(ctx context.Context) (err error)
	SaveProjectConfig(ctx context.Context, projectId string, projectConfig module_model.ProjectConfig, realStore ModuleStoreI) error
	SaveDataSource(ctx context.Context, projectId string, datasource *module_model.DataSource, realStore ModuleStoreI) error
	RemoveDataSource(ctx context.Context, projectId string, dbAlias string, realStore ModuleStoreI) error
	GetDataSource(ctx context.Context, projectId string, dbAlias string) (datasource *module_model.DataSource, err error)
	GetDataSources(ctx context.Context, projectId string) (datasources map[string]*module_model.DataSource, err error)
	UpdateSchemaTables(ctx context.Context, projectId string, dbAlias string, realStore ModuleStoreI) (datasource *module_model.DataSource, err error)
	AddSchemaTable(ctx context.Context, projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (tables map[string]interface{}, err error)
	SaveSchemaTable(ctx context.Context, projectId string, dbAlias string, tableName string, tableObj map[string]module_model.TableColsMetaData, realStore ModuleStoreI) (err error)
	SaveTableSecurity(ctx context.Context, projectId string, dbAlias string, tableName string, securityRules module_model.SecurityRules, realStore ModuleStoreI) (err error)
	SaveTableTransformation(ctx context.Context, projectId string, dbAlias string, tableName string, transformRules module_model.TransformRules, realStore ModuleStoreI) (err error)
	GetTableTransformation(ctx context.Context, projectId string, dbAlias string, tableName string) (transformRules module_model.TransformRules, err error)
	GetTableSecurityRule(ctx context.Context, projectId string, dbAlias string, tableName string) (transformRules module_model.SecurityRules, err error)
	DropSchemaTable(ctx context.Context, projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (err error)
	RemoveSchemaTable(ctx context.Context, projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (tables map[string]interface{}, err error)
	SaveMyQuery(ctx context.Context, projectId string, queryName string, queryType string, dbAlias string, query string, vars map[string]interface{}, realStore ModuleStoreI, cols string, securityRule security_rule.SecurityRule) error
	RemoveMyQuery(ctx context.Context, projectId string, queryName string, realStore ModuleStoreI) error
	GetMyQuery(ctx context.Context, projectId string, queryName string) (myquery module_model.MyQuery, err error)
	GetMyQueries(ctx context.Context, projectId string, queryType string) (myqueries map[string]module_model.MyQuery, err error)
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
			log.Print("SaveStore called from SaveProject")
			return realStore.SaveStore("", realStore)
		} else {
			return nil
		}
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " already exists"))
	}
}

func (ms *ModuleStore) RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveProject - Start")
	if _, ok := ms.Projects[projectId]; ok {
		delete(ms.Projects, projectId)
		log.Print("SaveStore called from RemoveProject")
		return realStore.SaveStore("", realStore)
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
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

func (ms *ModuleStore) GetProjectConfigObject(ctx context.Context, projectId string) (pc module_model.ProjectConfig, err error) {
	logs.WithContext(ctx).Debug("GetProjectConfigObject - Start")
	if _, ok := ms.Projects[projectId]; ok {
		return ms.Projects[projectId].ProjectConfig, nil
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
		project["projectName"] = k
		//project["lastUpdateDate"] = time.Now()
		projects[i] = project
		i++
	}
	return projects
}

func (ms *ModuleStore) SetDataSourceConnections(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("SetDataSourceConnections - Start")
	for _, prj := range ms.Projects {
		for _, datasource := range prj.DataSources {
			i := ds.GetSqlMaker(datasource.DbName)
			if i != nil {
				err = i.CreateConn(ctx, datasource)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
				}
			} else {
				err = errors.New(fmt.Sprint(datasource.DbName, " not found"))
				logs.WithContext(ctx).Error(err.Error())
			}
		}
	}
	return nil
}

func (ms *ModuleStore) SaveProjectConfig(ctx context.Context, projectId string, projectConfig module_model.ProjectConfig, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("SaveProjectConfig - Start")
	err := ms.checkProjectExists(ctx, projectId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}

	ms.Projects[projectId].ProjectConfig = projectConfig

	return realStore.SaveStore("", realStore)
}

func (ms *ModuleStore) SaveDataSource(ctx context.Context, projectId string, datasource *module_model.DataSource, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("SaveDataSource - Start")
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
	//v, e := json.Marshal(store)
	//log.Print(e)
	//log.Print(string(v))

	sqlMaker := ds.GetSqlMaker(datasource.DbName)
	datasource.DbType = ds.GetDbType(datasource.DbName)

	err = sqlMaker.CreateConn(ctx, datasource)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
	}

	return realStore.SaveStore("", realStore)
}

func (ms *ModuleStore) RemoveDataSource(ctx context.Context, projectId string, dbAlias string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveDataSource - Start")
	err := ms.checkProjectDataSourceExists(ctx, projectId, dbAlias)
	if err != nil {
		return err
	}
	delete(ms.Projects[projectId].DataSources, dbAlias)
	return realStore.SaveStore("", realStore)
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

	for k, _ := range datasource.OtherTables {
		if v, ok := datasource.SchemaTables[k]; ok {
			datasource.SchemaTables[k] = v
			tmpList = append(tmpList, k)
		}
	}
	for i := 0; i < len(tmpList); i++ {
		delete(datasource.OtherTables, tmpList[i])
	}

	return datasource, realStore.SaveStore("", realStore)
}
func (ms *ModuleStore) AddSchemaTable(ctx context.Context, projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (tables map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("AddSchemaTable - Start")
	err = ms.checkProjectDataSourceExists(ctx, projectId, dbAlias)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	datasource := ms.Projects[projectId].DataSources[dbAlias]
	if val, ok := datasource.OtherTables[tableName]; ok {
		if datasource.SchemaTables == nil {
			datasource.SchemaTables = make(map[string]map[string]module_model.TableColsMetaData)
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

		err = realStore.SaveStore("", realStore)
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
	err = ms.checkProjectDataSourceExists(ctx, projectId, dbAlias)
	if err != nil {
		return nil, err
	}
	datasource := ms.Projects[projectId].DataSources[dbAlias]
	if val, ok := datasource.SchemaTables[tableName]; ok {
		if datasource.OtherTables == nil {
			datasource.OtherTables = make(map[string]map[string]module_model.TableColsMetaData)
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
		err = realStore.SaveStore("", realStore)
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
	err = ms.checkProjectDataSourceExists(ctx, projectId, dbAlias)
	if err != nil {
		return nil, err
	}
	datasource := ms.Projects[projectId].DataSources[dbAlias]
	datasource.AddTableJoins(ctx, tj)
	err = realStore.SaveStore("", realStore)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	} else {
		return map[string]interface{}{"TableJoins": datasource.TableJoins}, nil
	}
}
func (ms *ModuleStore) RemoveSchemaJoin(ctx context.Context, projectId string, dbAlias string, tj *module_model.TableJoins, realStore ModuleStoreI) (tables map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("RemoveSchemaJoin - Start")
	err = ms.checkProjectDataSourceExists(ctx, projectId, dbAlias)
	if err != nil {
		return nil, err
	}
	datasource := ms.Projects[projectId].DataSources[dbAlias]
	datasource.RemoveTableJoins(ctx, tj)
	err = realStore.SaveStore("", realStore)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	} else {
		return map[string]interface{}{"TableJoins": datasource.TableJoins}, nil
	}
}

func (ms *ModuleStore) SaveMyQuery(ctx context.Context, projectId string, queryName string, queryType string, dbAlias string, query string, vars map[string]interface{}, realStore ModuleStoreI, cols string, securityRule security_rule.SecurityRule) error {
	logs.WithContext(ctx).Debug("SaveMyQuery - Start")
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
		myquery := module_model.MyQuery{queryName, query, vars, queryType, dbAlias, readWrite, cols, securityRule}
		if ms.Projects[projectId].MyQueries == nil {
			ms.Projects[projectId].MyQueries = make(map[string]*module_model.MyQuery)
		}
		ms.Projects[projectId].MyQueries[queryName] = &myquery
		return realStore.SaveStore("", realStore)
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
	if _, ok := ms.Projects[projectId]; ok {
		if ms.Projects[projectId].MyQueries == nil {
			return errors.New(fmt.Sprint("Query ", queryName, " not found"))
		}
		if _, ok = ms.Projects[projectId].MyQueries[queryName]; ok {
			delete(ms.Projects[projectId].MyQueries, queryName)
			return realStore.SaveStore("", realStore)
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
				logs.WithContext(ctx).Error(err.Error())
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

func (ms *ModuleStore) SaveSchemaTable(ctx context.Context, projectId string, dbAlias string, tableName string, tableObj map[string]module_model.TableColsMetaData, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveSchemaTable - Start")
	tableExists := false
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if _, ok := db.SchemaTables[tableName]; ok {
				tableExists = true
				logs.WithContext(ctx).Info("table exists in Schema table - to alter")
			} else if _, ok := db.OtherTables[tableName]; ok {
				tableExists = true
				logs.WithContext(ctx).Info("table exists in Other table - to alter")
			}
			if tableExists {
				//alter table
				err = errors.New("Alter table not implemented as yet")
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
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
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if _, ok := db.SchemaTables[tableName]; ok {
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
	return realStore.SaveStore("", realStore)
}

func (ms *ModuleStore) SaveTableTransformation(ctx context.Context, projectId string, dbAlias string, tableName string, transformRules module_model.TransformRules, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveTableTransformation - Start")
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
	return realStore.SaveStore("", realStore)
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

func (ms *ModuleStore) GetTableSecurityRule(ctx context.Context, projectId string, dbAlias string, tableName string) (securityRules module_model.SecurityRules, err error) {
	logs.WithContext(ctx).Debug("GetTableSecurityRule - Start")
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if _, ok := db.SchemaTables[tableName]; ok {
				securityRules = db.SchemaTablesSecurity[tableName]
			} else if _, ok := prj.MyQueries[tableName]; ok {
				securityRules.Query = prj.MyQueries[tableName].SecurityRule
			} else {
				err = errors.New(fmt.Sprint("Table ", tableName, " not found"))
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
				}
				return securityRules, err
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
	tableExists := false
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
