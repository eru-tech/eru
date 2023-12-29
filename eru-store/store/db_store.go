package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type DbStore struct {
	Store
	DbType         string    `json:"db_type"`
	UpdateTime     time.Time `json:"update_time"`
	StoreTableName string    `json:"store_table_name"`
	storeType      string
	conStr         string
	Con            *sqlx.DB `json:"-"`
	ConStatus      bool     `json:"-"`
}

type Queries struct {
	Query string
	Vals  []interface{}
}

func getStoreDbPath() string {
	dbString := os.Getenv("STORE_DB_PATH")
	logs.Logger.Debug(fmt.Sprint("dbString = ", dbString))
	return dbString
}
func (store *DbStore) SetDbType(dbtype string) {
	store.DbType = strings.ToLower(dbtype)
}

func (store *DbStore) GetDbType() string {
	return store.DbType
}

func (store *DbStore) SetStoreTableName(tablename string) {
	store.StoreTableName = strings.ToLower(tablename)
}

func (store *DbStore) GetStoreByteArray(dbString string) (b []byte, err error) {
	//TODO to implement this function
	logs.Logger.Debug("GetStoreByteArray - Start")
	if dbString == "" {
		dbString = getStoreDbPath()
		if dbString == "" {
			err := errors.New("No value found for environment variable STORE_DB_PATH")
			logs.Logger.Error(err.Error())
			return nil, err
		}
	}
	logs.Logger.Info("Creating DB connection for GetStoreByteArray")
	db, err := sqlx.Open(store.DbType, dbString)
	if err != nil {
		logs.Logger.Error(err.Error())
		return nil, err
	}
	defer db.Close()
	logs.Logger.Info(fmt.Sprint("db connection succesfull - fetch config from ", store.StoreTableName))
	//rows, err := db.Queryx(fmt.Sprint("select replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(config::text,'MyQueries','my_queries'),'ProjectId','project_id'),'DataSources','data_sources'),'QueryName','query_name'),'Query','query'),'Vars','vars'),'queryType','query_type'),'DBAlias','db_alias'),'DbAlias','db_alias'),'ReadWrite','read_write'),'Cols','cols'),'SecurityRule','security_rule'),'DbType','db_type'),'DbName','db_name'),'DbConfig','db_config'),'SchemaTables','schema_tables'),'OtherTables','other_tables'),'schema_tablesSecurity','schema_tables_security'),'schema_tablesTransformation','schema_tables_transformation'),'TableJoins','table_joins'),'ConStatus','con_status'),'Dbsecurity_rules','db_security_rules'),'Table1Name','table1_name'),'Table1cols','table1_cols'),'Table2Name','table2_name'),'Table2cols','table2_cols'),'IsActive','is_active'),'IsCustom','is_custom'),'ComplexCondition','complex_condition'),'TblSchema','tbl_schema'),'TblName','tbl_name'),'ColName','col_name'),'DataType','data_type'),'OwnDataType','own_data_type'),'PrimaryKey','primary_key'),'IsUnique','is_unique'),'PkConstraintName','pk_constraint_name'),'UqConstraintName','uq_constraint_name'),'IsNullable','is_nullable'),'ColPosition','col_position'),'DefaultValue','default_value'),'AutoIncrement','auto_increment'),'CharMaxLength','char_max_length'),'NumericPrecision','numeric_precision'),'NumericScale','numeric_scale'),'DatetimePrecision','datetime_precision'),'FkConstraintName','fk_constraint_name'),'FkDeleteRule','fk_delete_rule'),'FkTblSchema','fk_tbl_schema'),'FkTblName','fk_tbl_name'),'FkColName','fk_col_name'),'ColumnMasking','column_masking'),'Create','create'),'Drop','drop'),'Alter','alter'),'Insert','insert'),'Select','select'),'Update','update'),'Delete','delete'),'TransformInput','transform_input'),'TransformOutput','transform_output'),'RuleType','rule_type'),'ApplyOn','apply_on'),'Rules','rules'),'CustomRule','custom_rule'),'ForceColumnValues','force_column_values'),'RemoveColumnValues','remove_column_values'),'ComplexScript','complex_script'),'RuleRank','rule_rank'),'Host','host'),'Port','port'),'User','user'),'Password','password'),'DefaultDB','default_db'),'DefaultSchema','default_schema'),'DriverConfig','driver_config'),'Otherdb_config','other_db_config'),'MaxOpenConns','max_open_conns'),'MaxIdleConns','max_idle_conns'),'ConnMaxLifetime','conn_max_lifetime'),'RowLimit','row_limit'),'queryTimeOut','query_time_out'),'updateTime','update_time'),'StoreTableName','store_table_name'),'SecretManager','secret_manager'),'Secrets','secrets'),'Envvars','env_vars'),'Vars','vars'),'Key','key'),'Value','value'),'AND','and'),'OR','or'),'Variable1','variable1'),'Variable2','variable2'),'Operator','operator'),'ErrorMsg','error_msg'),'RuleType','rule_type'),'CustomRule','custom_rule')::jsonb config, create_date from ", store.StoreTableName, " limit 1"))
	//rows, err := db.Queryx(fmt.Sprint("select replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(config::text, 'DbType','db_type'),'UpdateTime','update_time'),'StoreTableName','store_table_name'),'Variables','variables'),'Auth','auth'),'Gateways','gateways'),'MessageTemplates','message_templates'),'ProjectId','project_id'),'Channel','channel'),'GatewayMethod','gateway_method'),'Allocation','allocation'),'GatewayName','gateway_name'),'GatewayType','gateway_type'),'GatewayUrl','gateway_url'),'QueryParams','query_params'),'templateId','template_id'),'TemplateId','Template_id'),'TemplateName','template_name'),'TemplateText','template_text'),'TemplateType','template_type'),'Hooks','hooks'),'SRC','src'),'SRCF','srcf'),'SVCF','svcf'),'SWEF','swef'),'AdminHost','admin_host'),'AdminPort','admin_port'),'AdminScheme','admin_scheme'),'authURL','auth_url'),'HydraClients','hydra_clients'),'PublicHost','public_host'),'PublicPort','public_port'),'PublicScheme','public_scheme'),'TokenURL','token_url'),'LoginMethod','login_method'),'Kratos','kratos'),'Hydra','hydra'),'TokenHeaderKey','token_header_key'),'authType','auth_type'),'authName','auth_name'),'UpdateTime','update_time'),'ProjectId','project_id'),'DbType','db_type'),'Routes','routes'),'FuncGroups','func_groups'),'ConditionFailAction','condition_fail_action'),'ConditionFailMessage','condition_fail_message'),'Condition','condition'),'IsPublic','is_public'),'AsyncMessage','async_message'),'Async','async'),'LoopVariable','loop_variable'),'LoopInParallel','loop_in_parallel'),'RouteName','route_name'),'RouteCategoryName','route_category_name'),'TargetHosts','target_hosts'),'MatchType','match_type'),'RewriteUrl','rewrite_url'),'AllowedHosts','allowed_hosts'),'AllowedMethods','allowed_methods'),'RequiredHeaders','required_headers'),'EnableCache','enable_cache'),'RequestHeaders','request_headers'),'QueryParams','query_params'),'FormData','form_data'),'FileData','file_data'),'ResponseHeaders','response_headers'),'TransformRequest','transform_request'),'TransformResponse','transform_response'),'TokenSecretKey','token_secret_key'),'RemoveParams','remove_params'),'OnError','on_error'),'RedirectUrl','redirect_url'),'FinalRedirectUrl','final_redirect_url'),'RedirectScheme','redirect_scheme'),'RedirectParams','_redirect_params'),'Redirect','redirect'),'Allocation','allocation'),'Scheme','scheme'),'Method','method'),'Port','port'),'Host','host'),'IsTemplate','is_template'),'Key','key'),'Value','value'),'FileName','file_name'),'FileVarName','file_var_name'),'FileContent','file_content'),'FuncCategoryName','func_category_name'),'FuncGroupName','func_group_name'),'FuncSteps','func_steps'),'FunctionName','function_name'),'QueryName','query_name'),'QueryOutputEncode','query_output_encode'),'QueryOutput','query_output'),'ApiPath','api_path'),'Api','api'),'Path','path'),'Url','url'),'StoreTableName','store_table_name'),'\"Vars\"','\"vars\"'),'EnvVars','env_vars'),'Secrets','secrets') ::jsonb config, create_date from ", store.StoreTableName, " limit 1"))
	//rows, err := db.Queryx(fmt.Sprint("select replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(config::text,'UpdateTime','update_time'),'ProjectId','project_id'),'DbType','db_type'),'Routes','routes'),'FuncGroups','func_groups'),'ConditionFailAction','condition_fail_action'),'ConditionFailMessage','condition_fail_message'),'Condition','condition'),'IsPublic','is_public'),'AsyncMessage','async_message'),'Async','async'),'LoopVariable','loop_variable'),'LoopInParallel','loop_in_parallel'),'RouteName','route_name'),'RouteCategoryName','route_category_name'),'TargetHosts','target_hosts'),'MatchType','match_type'),'RewriteUrl','rewrite_url'),'AllowedHosts','allowed_hosts'),'AllowedMethods','allowed_methods'),'RequiredHeaders','required_headers'),'EnableCache','enable_cache'),'RequestHeaders','request_headers'),'QueryParams','query_params'),'FormData','form_data'),'FileData','file_data'),'ResponseHeaders','response_headers'),'TransformRequest','transform_request'),'TransformResponse','transform_response'),'TokenSecretKey','token_secret_key'),'RemoveParams','remove_params'),'OnError','on_error'),'RedirectUrl','redirect_url'),'FinalRedirectUrl','final_redirect_url'),'RedirectScheme','redirect_scheme'),'RedirectParams','_redirect_params'),'Redirect','redirect'),'Allocation','allocation'),'Scheme','scheme'),'Method','method'),'Port','port'),'Host','host'),'IsTemplate','is_template'),'Key','key'),'Value','value'),'FileName','file_name'),'FileVarName','file_var_name'),'FileContent','file_content'),'FuncCategoryName','func_category_name'),'FuncGroupName','func_group_name'),'FuncSteps','func_steps'),'FunctionName','function_name'),'QueryName','query_name'),'QueryOutputEncode','query_output_encode'),'QueryOutput','query_output'),'ApiPath','api_path'),'Api','api'),'Path','path'),'Url','url'),'StoreTableName','store_table_name'),'\"Vars\"','\"vars\"'),'EnvVars','env_vars'),'Secrets','secrets')::jsonb config, create_date from ", store.StoreTableName, " limit 1"))
	rows, err := db.Queryx(fmt.Sprint("select replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(config::text,'AuthorizerName','authorizer_name'),'TokenHeaderKey','token_header_key'),'SecretAlgo','secret_algo'),'JwkUrl','jwk_url'),'Audience','audience'),'Issuer','issuer'),'RuleRank','rule_rank'),'RuleName','rule_name'),'Hosts','hosts'),'Paths','paths'),'Headers','headers'),'Addheaders','add_headers'),'Params','params'),'Methods','methods'),'SourceIP','source_ip'),'Targethosts','target_hosts'),'AuthorizerName','authorizer_name'),'AuthorizerException','authorizer_exception'),'authorizer_exceptionIP','authorizer_exception_ip'),'Key','key'),'Value','value'),'IsTemplate','is_template'),'MatchType','match_type'),'Path','path'),'Host','host'),'Port','port'),'Method','method'),'Scheme','scheme'),'Allocation','allocation'),'DbType','db_type'),'Variables','variables'),'UpdateTime','update_time'),'Authorizers','authorizers'),'ListenerRules','listener_rules'),'StoreTableName','store_table_name')::jsonb config, create_date from ", store.StoreTableName, " limit 1"))

	if err != nil {
		logs.Logger.Error(err.Error())
		return nil, err
	}
	logs.Logger.Info("config fetched succesfully")
	mapping := make(map[string]interface{})
	var storeData interface{}
	for rows.Next() {
		err = rows.MapScan(mapping)
		if err != nil {
			logs.Logger.Error(err.Error())
			return nil, err
		}
		storeData = mapping["config"]
		storeUpdateTime := mapping["create_date"]
		store.UpdateTime = storeUpdateTime.(time.Time)
	}
	if storeData == nil {
		err = errors.New("no config data retrived from db")
		logs.Logger.Error(err.Error())
		return nil, err
	}
	logs.Logger.Info("config loaded successfully")
	return storeData.([]byte), err
}

func (store *DbStore) LoadStore(dbString string, ms StoreI) (err error) {
	logs.Logger.Debug("LoadStore - Start")
	if dbString == "" {
		dbString = getStoreDbPath()
	}
	logs.Logger.Info("Creating DB connection for Load DB store")
	db, err := sqlx.Open(store.DbType, dbString)
	defer db.Close()
	if err != nil {
		logs.Logger.Error(err.Error())
		return err
	}
	rows, err := db.Queryx(fmt.Sprint("select * from ", store.StoreTableName, " limit 1"))
	if err != nil {
		logs.Logger.Error(err.Error())
		return err
	}
	mapping := make(map[string]interface{})
	for rows.Next() {
		err = rows.MapScan(mapping)
		if err != nil {
			logs.Logger.Error(err.Error())
			return err
		}
		storeData := mapping["config"]
		storeUpdateTime := mapping["create_date"]
		// Marshalling the store
		//store = new(FileStore)
		err = json.Unmarshal(storeData.([]byte), ms)
		if err != nil {
			logs.Logger.Error(err.Error())
			return err
		}
		store.UpdateTime = storeUpdateTime.(time.Time)
		logs.Logger.Info(fmt.Sprint("storeUpdateTime == ", storeUpdateTime))
	}
	if err != nil {
		logs.Logger.Error(err.Error())
		return err
	}
	return nil
}

func (store *DbStore) SaveStore(ctx context.Context, dbString string, ms StoreI) (err error) {
	logs.WithContext(ctx).Debug("SaveStore - Start")
	if dbString == "" {
		dbString = getStoreDbPath()
	}
	logs.WithContext(ctx).Info("Creating DB connection for Save DB store")
	db, err := sqlx.Open(store.DbType, dbString)
	defer db.Close()
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	tx := db.MustBegin()
	storeData, err := json.Marshal(ms)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		tx.Rollback()
		return err
	}
	strStoreData := strings.Replace(string(storeData), "'", "''", -1)
	query := fmt.Sprint("update ", store.StoreTableName, " set create_date=current_timestamp , config = '", strStoreData, "' returning create_date")
	stmt, err := tx.PreparexContext(ctx, query)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("Error in tx.PreparexContext : ", err.Error()))
		tx.Rollback()
		return err
	}
	rw, err := stmt.QueryxContext(ctx)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("Error in stmt.QueryxContext : ", err.Error()))
		tx.Rollback()
		return err
	}
	for rw.Rows.Next() {
		resDoc := make(map[string]interface{})
		err = rw.MapScan(resDoc)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("Error in rw.MapScan : ", err.Error()))
			tx.Rollback()
			return err
		}
		logs.WithContext(ctx).Info(fmt.Sprint("Old store.UpdateTime = ", store.UpdateTime))
		store.UpdateTime = resDoc["create_date"].(time.Time)
		logs.WithContext(ctx).Info(fmt.Sprint("New store.UpdateTime = ", store.UpdateTime))
	}
	err = tx.Commit()
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("Error in tx.Commit : ", err.Error()))
		tx.Rollback()
	}
	return nil
}

func (store *DbStore) getStoreDBConnStr() (string, error) {
	logs.Logger.Debug("getStoreDBConnStr - Start")
	dbConStr := os.Getenv("storedb")
	if dbConStr == "" {
		err := errors.New(fmt.Sprint("storedb environment variable not found"))
		logs.Logger.Error(err.Error())
		return "", err
	}
	return dbConStr, nil
}

func (store *DbStore) CreateConn() error {
	logs.Logger.Debug("CreateConn - Start")
	connString := getStoreDbPath()
	db, err := sqlx.Open(store.DbType, connString)
	if err != nil {
		logs.Logger.Error(err.Error())
		store.ConStatus = false
		return err
	}
	logs.Logger.Info("db connection was successfully done for fetch dummy query")
	_, err = db.Queryx("select 1")
	if err != nil {
		store.ConStatus = false
		logs.Logger.Error(err.Error())
		return err
	}
	logs.Logger.Info("dummy query success - setting con as true")
	store.Con = db
	store.ConStatus = true
	return nil
}

func (store *DbStore) GetConn() *sqlx.DB {
	logs.Logger.Debug("CreateConn - Start")
	return store.Con
}

func (store *DbStore) ExecuteDbFetch(ctx context.Context, query Queries) (output []map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("ExecuteDbFetch - Start")

	db := store.GetConn()
	if db == nil {
		err = store.CreateConn()
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
		db = store.GetConn()
	}

	rows, err := db.Queryx(query.Query, query.Vals...)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	mapping := make(map[string]interface{})
	colsType, ee := rows.ColumnTypes()
	if ee != nil {
		return nil, ee
	}
	for rows.Next() {
		innerResultRow := make(map[string]interface{})
		ee = rows.MapScan(mapping)
		if ee != nil {
			return nil, ee
		}
		for _, colType := range colsType {
			if colType.DatabaseTypeName() == "NUMERIC" && mapping[colType.Name()] != nil {
				f := 0.0
				if reflect.TypeOf(mapping[colType.Name()]).String() == "[]uint8" {
					f, err = strconv.ParseFloat(string(mapping[colType.Name()].([]byte)), 64)
					mapping[colType.Name()] = f
				} else if reflect.TypeOf(mapping[colType.Name()]).String() == "float64" {
					f = mapping[colType.Name()].(float64)
					mapping[colType.Name()] = f
				}
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return nil, err
				}
			} else if (colType.DatabaseTypeName() == "JSONB" || colType.DatabaseTypeName() == "JSON") && mapping[colType.Name()] != nil {
				bytesToUnmarshal := mapping[colType.Name()].([]byte)
				var v interface{}
				err = json.Unmarshal(bytesToUnmarshal, &v)
				if err != nil {
					return nil, err
				}
				mapping[colType.Name()] = &v
			}
			innerResultRow[colType.Name()] = mapping[colType.Name()]
		}
		output = append(output, innerResultRow)
	}
	return
}

func (store *DbStore) ExecuteDbSave(ctx context.Context, queries []Queries) (output [][]map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("ExecuteDbSave - Start")
	db := store.GetConn()
	if db == nil {
		err = store.CreateConn()
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
		db = store.GetConn()
	}
	tx := db.MustBegin()
	for _, q := range queries {
		//logs.WithContext(ctx).Info(q.Query)
		//logs.WithContext(ctx).Info(fmt.Sprint(q.Vals))
		stmt, err := tx.PreparexContext(ctx, q.Query)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("Error in tx.PreparexContext : ", err.Error()))
			tx.Rollback()
			return nil, err
		}
		rw, err := stmt.QueryxContext(ctx, q.Vals...)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("Error in stmt.QueryxContext : ", err.Error()))
			tx.Rollback()
			return nil, err
		}
		var innerOutput []map[string]interface{}
		for rw.Rows.Next() {
			resDoc := make(map[string]interface{})
			err = rw.MapScan(resDoc)
			if err != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("Error in rw.MapScan : ", err.Error()))
				tx.Rollback()
				return nil, err
			}
			innerOutput = append(innerOutput, resDoc)
		}
		output = append(output, innerOutput)
	}
	err = tx.Commit()
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("Error in tx.Commit : ", err.Error()))
		tx.Rollback()
	}
	return
}
