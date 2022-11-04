package module_store

import (
	"errors"
	"fmt"
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
	SaveProject(projectId string, realStore ModuleStoreI, persist bool) error
	RemoveProject(projectId string, realStore ModuleStoreI) error
	GetProjectConfig(projectId string) (*module_model.Project, error)
	GetProjectConfigObject(projectId string) (pc module_model.ProjectConfig, err error)
	GetProjectList() []map[string]interface{}
	SetDataSourceConnections() (err error)
	SaveProjectConfig(projectId string, projectConfig module_model.ProjectConfig, realStore ModuleStoreI) error
	SaveDataSource(projectId string, datasource *module_model.DataSource, realStore ModuleStoreI) error
	RemoveDataSource(projectId string, dbAlias string, realStore ModuleStoreI) error
	GetDataSource(projectId string, dbAlias string) (datasource *module_model.DataSource, err error)
	GetDataSources(projectId string) (datasources map[string]*module_model.DataSource, err error)
	UpdateSchemaTables(projectId string, dbAlias string, realStore ModuleStoreI) (datasource *module_model.DataSource, err error)
	AddSchemaTable(projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (tables map[string]interface{}, err error)
	SaveSchemaTable(projectId string, dbAlias string, tableName string, tableObj map[string]module_model.TableColsMetaData, realStore ModuleStoreI) (err error)
	SaveTableSecurity(projectId string, dbAlias string, tableName string, securityRules module_model.SecurityRules, realStore ModuleStoreI) (err error)
	SaveTableTransformation(projectId string, dbAlias string, tableName string, transformRules module_model.TransformRules, realStore ModuleStoreI) (err error)
	GetTableTransformation(projectId string, dbAlias string, tableName string) (transformRules module_model.TransformRules, err error)
	GetTableSecurityRule(projectId string, dbAlias string, tableName string) (transformRules module_model.SecurityRules, err error)
	DropSchemaTable(projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (err error)
	RemoveSchemaTable(projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (tables map[string]interface{}, err error)
	SaveMyQuery(projectId string, queryName string, queryType string, dbAlias string, query string, vars map[string]interface{}, realStore ModuleStoreI, cols string, securityRule security_rule.SecurityRule) error
	RemoveMyQuery(projectId string, queryName string, realStore ModuleStoreI) error
	GetMyQuery(projectId string, queryName string) (myquery *module_model.MyQuery, err error)
	GetMyQueries(projectId string, queryType string) (myqueries map[string]module_model.MyQuery, err error)
	AddSchemaJoin(projectId string, dbAlias string, tj *module_model.TableJoins, realStore ModuleStoreI) (tables map[string]interface{}, err error)
	RemoveSchemaJoin(projectId string, dbAlias string, tj *module_model.TableJoins, realStore ModuleStoreI) (tables map[string]interface{}, err error)
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

func (ms *ModuleStore) SaveProject(projectId string, realStore ModuleStoreI, persist bool) error {
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

func (ms *ModuleStore) RemoveProject(projectId string, realStore ModuleStoreI) error {
	if _, ok := ms.Projects[projectId]; ok {
		delete(ms.Projects, projectId)
		log.Print("SaveStore called from RemoveProject")
		return realStore.SaveStore("", realStore)
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}

func (ms *ModuleStore) GetProjectConfig(projectId string) (*module_model.Project, error) {
	if _, ok := ms.Projects[projectId]; ok {
		//log.Println(store.Projects[projectId])
		return ms.Projects[projectId], nil
	} else {
		return nil, errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}

func (ms *ModuleStore) GetProjectConfigObject(projectId string) (pc module_model.ProjectConfig, err error) {
	if _, ok := ms.Projects[projectId]; ok {
		//log.Println(store.Projects[projectId])
		return ms.Projects[projectId].ProjectConfig, nil
	} else {
		return pc, errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}

func (ms *ModuleStore) GetProjectList() []map[string]interface{} {
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

func (ms *ModuleStore) SetDataSourceConnections() (err error) {
	for _, prj := range ms.Projects {
		log.Print(prj.ProjectId)
		for _, datasource := range prj.DataSources {
			log.Print(datasource.DbName)
			log.Print(datasource.DbConfig)
			i := ds.GetSqlMaker(datasource.DbName)
			log.Print(i)
			if i != nil {
				err = i.CreateConn(datasource)
				if err != nil {
					log.Print(err)
				}
			} else {
				err = errors.New(fmt.Sprint(datasource.DbName, " not found"))
				log.Print(err.Error())
			}
		}
	}
	log.Print("exiting SetDataSourceConnections")
	return nil
}

func (ms *ModuleStore) SaveProjectConfig(projectId string, projectConfig module_model.ProjectConfig, realStore ModuleStoreI) error {
	err := ms.checkProjectExists(projectId)
	if err != nil {
		return err
	}

	ms.Projects[projectId].ProjectConfig = projectConfig

	return realStore.SaveStore("", realStore)
}

func (ms *ModuleStore) SaveDataSource(projectId string, datasource *module_model.DataSource, realStore ModuleStoreI) error {
	err := ms.checkProjectExists(projectId)
	if err != nil {
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

	err = sqlMaker.CreateConn(datasource)
	if err != nil {
		//return err
		log.Print(err.Error())
	}

	return realStore.SaveStore("", realStore)
}

func (ms *ModuleStore) RemoveDataSource(projectId string, dbAlias string, realStore ModuleStoreI) error {
	err := ms.checkProjectDataSourceExists(projectId, dbAlias)
	if err != nil {
		return err
	}
	delete(ms.Projects[projectId].DataSources, dbAlias)
	return realStore.SaveStore("", realStore)
}

func (ms *ModuleStore) GetDataSource(projectId string, dbAlias string) (datasource *module_model.DataSource, err error) {
	err = ms.checkProjectDataSourceExists(projectId, dbAlias)
	if err != nil {
		return nil, err
	}
	return ms.Projects[projectId].DataSources[dbAlias], nil
}

func (ms *ModuleStore) GetDataSources(projectId string) (datasources map[string]*module_model.DataSource, err error) {
	err = ms.checkProjectExists(projectId)
	if err != nil {
		return nil, err
	}
	return ms.Projects[projectId].DataSources, nil
}

func (ms *ModuleStore) UpdateSchemaTables(projectId string, dbAlias string, realStore ModuleStoreI) (datasource *module_model.DataSource, err error) {
	var tmpList []string
	log.Println("inside UpdateSchemaTables")
	err = ms.checkProjectDataSourceExists(projectId, dbAlias)
	if err != nil {
		return nil, err
	}

	datasource = ms.Projects[projectId].DataSources[dbAlias]
	sr := ds.GetSqlMaker(datasource.DbName)
	err = sr.GetTableList(sr.GetTableMetaDataSQL(), datasource, sr)
	if err != nil {
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
func (ms *ModuleStore) AddSchemaTable(projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (tables map[string]interface{}, err error) {
	err = ms.checkProjectDataSourceExists(projectId, dbAlias)
	if err != nil {
		return nil, err
	}
	datasource := ms.Projects[projectId].DataSources[dbAlias]
	log.Print(tableName)
	for k, _ := range datasource.OtherTables {
		log.Print(k)
	}
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
				log.Print(&tj)
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
			return nil, err
		} else {
			return map[string]interface{}{"SchemaTables": datasource.SchemaTables, "OtherTables": datasource.OtherTables}, nil
		}
	} else {
		return nil, errors.New(fmt.Sprintf(tableName, " not found to add in the schema"))
	}
}
func (ms *ModuleStore) RemoveSchemaTable(projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (tables map[string]interface{}, err error) {
	err = ms.checkProjectDataSourceExists(projectId, dbAlias)
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
			return nil, err
		} else {
			return map[string]interface{}{"SchemaTables": datasource.SchemaTables, "OtherTables": datasource.OtherTables}, nil
		}
	} else {
		return nil, errors.New(fmt.Sprintf(tableName, " not found to add in the schema"))
	}
}

func (ms *ModuleStore) AddSchemaJoin(projectId string, dbAlias string, tj *module_model.TableJoins, realStore ModuleStoreI) (tables map[string]interface{}, err error) {
	err = ms.checkProjectDataSourceExists(projectId, dbAlias)
	if err != nil {
		return nil, err
	}
	datasource := ms.Projects[projectId].DataSources[dbAlias]
	datasource.AddTableJoins(tj)
	err = realStore.SaveStore("", realStore)
	if err != nil {
		return nil, err
	} else {
		return map[string]interface{}{"TableJoins": datasource.TableJoins}, nil
	}
}
func (ms *ModuleStore) RemoveSchemaJoin(projectId string, dbAlias string, tj *module_model.TableJoins, realStore ModuleStoreI) (tables map[string]interface{}, err error) {
	err = ms.checkProjectDataSourceExists(projectId, dbAlias)
	if err != nil {
		return nil, err
	}
	datasource := ms.Projects[projectId].DataSources[dbAlias]
	datasource.RemoveTableJoins(tj)
	err = realStore.SaveStore("", realStore)
	if err != nil {
		return nil, err
	} else {
		return map[string]interface{}{"TableJoins": datasource.TableJoins}, nil
	}
}

func (ms *ModuleStore) SaveMyQuery(projectId string, queryName string, queryType string, dbAlias string, query string, vars map[string]interface{}, realStore ModuleStoreI, cols string, securityRule security_rule.SecurityRule) error {
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
		log.Print(myquery)
		if ms.Projects[projectId].MyQueries == nil {
			ms.Projects[projectId].MyQueries = make(map[string]*module_model.MyQuery)
		}
		ms.Projects[projectId].MyQueries[queryName] = &myquery
		return realStore.SaveStore("", realStore)
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " not found"))
	}
}

func (ms *ModuleStore) RemoveMyQuery(projectId string, queryName string, realStore ModuleStoreI) error {
	if _, ok := ms.Projects[projectId]; ok {
		if ms.Projects[projectId].MyQueries == nil {
			return errors.New(fmt.Sprint("Query ", queryName, " not found"))
		}
		if _, ok = ms.Projects[projectId].MyQueries[queryName]; ok {
			delete(ms.Projects[projectId].MyQueries, queryName)
			return realStore.SaveStore("", realStore)
		} else {
			return errors.New(fmt.Sprint("Query ", queryName, " not found"))
		}
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " not found"))
	}
}

func (ms *ModuleStore) GetMyQuery(projectId string, queryName string) (myquery *module_model.MyQuery, err error) {
	if _, ok := ms.Projects[projectId]; ok {
		if ms.Projects[projectId].MyQueries == nil {
			return nil, errors.New(fmt.Sprint("Query ", queryName, " not found"))
		}
		if myquery, ok = ms.Projects[projectId].MyQueries[queryName]; ok {
			return myquery, nil
		} else {
			return nil, errors.New(fmt.Sprint("Query ", queryName, " not found"))
		}
	} else {
		return nil, errors.New(fmt.Sprint("Project ", projectId, " not found"))
	}
}

func (ms *ModuleStore) GetMyQueries(projectId string, queryType string) (myqueries map[string]module_model.MyQuery, err error) {
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
		return nil, errors.New(fmt.Sprint("Project ", projectId, " not found"))
	}
}

func (ms *ModuleStore) SaveSchemaTable(projectId string, dbAlias string, tableName string, tableObj map[string]module_model.TableColsMetaData, realStore ModuleStoreI) (err error) {

	tableExists := false
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if _, ok := db.SchemaTables[tableName]; ok {
				tableExists = true
				log.Println("table exists in Schema table - to alter")
			} else if _, ok := db.OtherTables[tableName]; ok {
				tableExists = true
				log.Println("table exists in Other table - to alter")
			}
			if tableExists {
				//alter table
				err = errors.New("Alter table not implemented as yet")
			} else {
				//create table
				sr := ds.GetSqlMaker(db.DbName)
				query, err := sr.MakeCreateTableSQL(tableName, tableObj)
				if err != nil {
					return err
				}
				res, err := sr.ExecutePreparedQuery(query, db)
				if err != nil {
					return err
				}
				log.Println(res)
				//TODO to change store
			}
		} else {
			return errors.New(fmt.Sprint("Datasource ", dbAlias, " not found"))
		}
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " not found"))
	}
	return err
}

func (ms *ModuleStore) SaveTableSecurity(projectId string, dbAlias string, tableName string, securityRules module_model.SecurityRules, realStore ModuleStoreI) (err error) {
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if _, ok := db.SchemaTables[tableName]; ok {
				if db.SchemaTablesSecurity == nil {
					db.SchemaTablesSecurity = make(map[string]module_model.SecurityRules)
				}
				db.SchemaTablesSecurity[tableName] = securityRules
			} else {
				return errors.New(fmt.Sprint("Table ", tableName, " not found"))
			}
		} else {
			return errors.New(fmt.Sprint("Datasource ", dbAlias, " not found"))
		}
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " not found"))
	}
	if err != nil {
		return err
	}
	return realStore.SaveStore("", realStore)
}

func (ms *ModuleStore) SaveTableTransformation(projectId string, dbAlias string, tableName string, transformRules module_model.TransformRules, realStore ModuleStoreI) (err error) {
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if _, ok := db.SchemaTables[tableName]; ok {
				if db.SchemaTablesTransformation == nil {
					db.SchemaTablesTransformation = make(map[string]module_model.TransformRules)
				}
				db.SchemaTablesTransformation[tableName] = transformRules
			} else {
				return errors.New(fmt.Sprint("Table ", tableName, " not found"))
			}
		} else {
			return errors.New(fmt.Sprint("Datasource ", dbAlias, " not found"))
		}
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " not found"))
	}
	return realStore.SaveStore("", realStore)
}

func (ms *ModuleStore) GetTableTransformation(projectId string, dbAlias string, tableName string) (transformRules module_model.TransformRules, err error) {
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if _, ok := db.SchemaTables[tableName]; ok {
				transformRules = db.SchemaTablesTransformation[tableName]
			} else if _, ok := prj.MyQueries[tableName]; ok {
				//do nothing as there are no transform rule feature for my query TODO check feasibility
			} else {
				return transformRules, errors.New(fmt.Sprint("Table ", tableName, " not found"))
			}
		} else {
			return transformRules, errors.New(fmt.Sprint("Datasource ", dbAlias, " not found"))
		}
	} else {
		return transformRules, errors.New(fmt.Sprint("Project ", projectId, " not found"))
	}
	return
}

func (ms *ModuleStore) GetTableSecurityRule(projectId string, dbAlias string, tableName string) (securityRules module_model.SecurityRules, err error) {
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if _, ok := db.SchemaTables[tableName]; ok {
				securityRules = db.SchemaTablesSecurity[tableName]
			} else if _, ok := prj.MyQueries[tableName]; ok {
				securityRules.Query = prj.MyQueries[tableName].SecurityRule
			} else {
				return securityRules, errors.New(fmt.Sprint("Table ", tableName, " not found"))
			}
		} else {
			return securityRules, errors.New(fmt.Sprint("Datasource ", dbAlias, " not found"))
		}
	} else {
		return securityRules, errors.New(fmt.Sprint("Project ", projectId, " not found"))
	}
	return
}

func (ms *ModuleStore) DropSchemaTable(projectId string, dbAlias string, tableName string, realStore ModuleStoreI) (err error) {
	tableExists := false
	if prj, ok := ms.Projects[projectId]; ok {
		if db, ok := prj.DataSources[dbAlias]; ok {
			if _, ok := db.SchemaTables[tableName]; ok {
				tableExists = true
				delete(db.SchemaTables, tableName)
				log.Println("table exists in Schema table - to alter")
			} else if _, ok := db.OtherTables[tableName]; ok {
				tableExists = true
				delete(db.OtherTables, tableName)
				log.Println("table exists in Other table - to alter")
			}
			if tableExists {
				//drop table
				sr := ds.GetSqlMaker(db.DbName)
				query, err := sr.MakeDropTableSQL(tableName)
				if err != nil {
					return err
				}
				res, err := sr.ExecutePreparedQuery(query, db)
				if err != nil {
					return err
				}
				log.Println(res)
				//TODO to change store
			} else {
				err = errors.New(fmt.Sprint("Table ", tableName, " does not exists"))
			}
		} else {
			return errors.New(fmt.Sprint("Datasource ", dbAlias, " not found"))
		}
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " not found"))
	}
	return err
}
