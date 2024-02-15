package module_model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-secret-manager/sm"
	"github.com/eru-tech/eru/eru-security-rule/security_rule"
	"github.com/eru-tech/eru/eru-store/store"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jmoiron/sqlx"
	"time"
)

const (
	RULETYPE_NONE          = "none"
	RULETYPE_ALWAYS        = "always"
	RULETYPE_CUSTOM        = "custom"
	RULEPREFIX_TOKEN       = "token"
	RULEPREFIX_DOCS        = "docs"
	RULEPREFIX_NONE        = "none"
	QUERY_TYPE_INSERT      = "insert"
	QUERY_TYPE_UPDATE      = "update"
	QUERY_TYPE_DELETE      = "delete"
	QUERY_TYPE_SELECT      = "select"
	COLUMN_MASKING_NONE    = "none"
	COLUMN_MASKING_ENCRYPT = "encrypt"
	COLUMN_MASKING_HASH    = "hash"
	MAKE_JSON_ARRAY_FN     = "$make_json_array_fn"
)

type ModuleProjectI interface {
	CompareProject(ctx context.Context, compareProject ExtendedProject) (StoreCompare, error)
}

type StoreCompare struct {
	store.StoreCompare
	DeleteQueries               []string               `json:"delete_queries"`
	NewQueries                  []string               `json:"new_queries"`
	MismatchQueries             map[string]interface{} `json:"mismatch_queries"`
	DeleteDataSources           []string               `json:"delete_data_sources"`
	NewDataSources              []string               `json:"new_data_sources"`
	MismatchDataSources         map[string]interface{} `json:"mismatch_data_sources"`
	DeleteTables                []string               `json:"delete_tables"`
	NewTables                   []string               `json:"new_tables"`
	MismatchTables              map[string]interface{} `json:"mismatch_tables"`
	DeleteJoins                 []string               `json:"delete_joins"`
	NewJoins                    []string               `json:"new_joins"`
	MismatchJoins               map[string]interface{} `json:"mismatch_joins"`
	DeleteTableSecurity         []string               `json:"delete_table_security"`
	NewTableSecurity            []string               `json:"new_table_security"`
	MismatchTableSecurity       map[string]interface{} `json:"mismatch_table_security"`
	DeleteTableTransformation   []string               `json:"delete_table_transformation"`
	NewTableTransformation      []string               `json:"new_table_transformation"`
	MismatchTableTransformation map[string]interface{} `json:"mismatch_table_transformation"`
}

type ExtendedProject struct {
	Project
	Variables     store.Variables `json:"variables"`
	SecretManager sm.SmStoreI     `json:"secret_manager"`
}

type Project struct {
	ProjectId       string                 `json:"project_id" eru:"required"`
	DataSources     map[string]*DataSource `json:"data_sources"` //DB alias is the key
	MyQueries       map[string]*MyQuery    `json:"my_queries"`   //queryName is key
	ProjectSettings ProjectSettings        `json:"project_settings"`
}
type ProjectSettings struct {
	ClaimsKey string `json:"claims_key" eru:"required"`
}

/*
	type AesKey struct {
		Key string
		//Bits int
	}

	type TokenSecret struct {
		HeaderKey  string
		SecretAlgo string
		SecretKey  string
		JwkUrl     string
		Audience   []string
		Issuer     []string
	}
*/
type MyQuery struct {
	QueryName    string                     `json:"query_name"`
	Query        string                     `json:"query"`
	Vars         map[string]interface{}     `json:"vars"`
	QueryType    string                     `json:"query_type"`
	DBAlias      string                     `json:"db_alias"`
	ReadWrite    string                     `json:"read_write"`
	Cols         string                     `json:"cols"`
	SecurityRule security_rule.SecurityRule `json:"security_rule"`
}

type DataSource struct {
	DbAlias                    string                                  `json:"db_alias" eru:"required"`
	DbType                     string                                  `json:"db_type" eru:"required"`
	DbName                     string                                  `json:"db_name" eru:"required"`
	DbConfig                   DbConfig                                `json:"db_config" eru:"required"`
	SchemaTables               map[string]map[string]TableColsMetaData `json:"schema_tables"`         //tableName is the key
	OtherTables                map[string]map[string]TableColsMetaData `json:"other_tables" json:"-"` //tableName is the key
	SchemaTablesSecurity       map[string]SecurityRules                `json:"schema_tables_security"`
	SchemaTablesTransformation map[string]TransformRules               `json:"schema_tables_transformation"`
	TableJoins                 map[string]*TableJoins                  `json:"table_joins"`
	Con                        *sqlx.DB                                `json:"-"`
	ConStatus                  bool                                    `json:"con_status"`
	DbSecurityRules            SecurityRules                           `json:"db_security_rules"`
}

type TableJoins struct {
	Table1Name       string                 `json:"table1_name"`
	Table1Cols       []string               `json:"table1_cols"`
	Table2Name       string                 `json:"table2_name"`
	Table2Cols       []string               `json:"table2_cols"`
	IsActive         bool                   `json:"is_active"`
	IsCustom         bool                   `json:"is_custom"`
	ComplexCondition map[string]interface{} `json:"complex_condition"`
}

type TableColsMetaData struct {
	TblSchema         string        `json:"tbl_schema" eru:"required"`
	TblName           string        `json:"tbl_name" eru:"required"`
	ColName           string        `json:"col_name" eru:"required"`
	DataType          string        `json:"data_type"`
	OwnDataType       string        `json:"own_data_type" eru:"required"`
	PrimaryKey        bool          `json:"primary_key" eru:"required"`
	IsUnique          bool          `json:"is_unique" eru:"required"`
	PkConstraintName  string        `json:"pk_constraint_name"`
	UqConstraintName  string        `json:"uq_constraint_name"`
	IsNullable        bool          `json:"is_nullable" eru:"required"`
	ColPosition       int           `json:"col_position"`
	DefaultValue      string        `json:"default_value"`
	AutoIncrement     bool          `json:"auto_increment"`
	CharMaxLength     int           `json:"char_max_length"`
	NumericPrecision  string        `json:"numeric_precision"`
	NumericScale      int           `json:"numeric_scale"`
	DatetimePrecision int           `json:"datetime_precision"`
	FkConstraintName  string        `json:"fk_constraint_name"`
	FkDeleteRule      string        `json:"fk_delete_rule"`
	FkTblSchema       string        `json:"fk_tbl_schema"`
	FkTblName         string        `json:"fk_tbl_name"`
	FkColName         string        `json:"fk_col_name"`
	ColumnMasking     ColumnMasking `json:"column_masking"`
}

type ColumnMasking struct {
	MaskingType string `json:"masking_type"`
}

/*
	type CustomRule struct {
		AND []CustomRuleDetails `json:",omitempty"`
		OR  []CustomRuleDetails `json:",omitempty"`
	}

	type CustomRuleDetails struct {
		DataType  string              `json:",omitempty"`
		Variable1 string              `json:",omitempty"`
		Variable2 string              `json:",omitempty"`
		Operator  string              `json:",omitempty"`
		ErrorMsg  string              `json:",omitempty"`
		AND       []CustomRuleDetails `json:",omitempty"`
		OR        []CustomRuleDetails `json:",omitempty"`
	}

	type SecurityRule struct {
		RuleType   string
		CustomRule CustomRule
	}
*/
type SecurityRules struct {
	Create security_rule.SecurityRule `json:"create"`
	Drop   security_rule.SecurityRule `json:"drop"`
	Alter  security_rule.SecurityRule `json:"alter"`
	Insert security_rule.SecurityRule `json:"insert"`
	Update security_rule.SecurityRule `json:"update"`
	Delete security_rule.SecurityRule `json:"delete"`
	Select security_rule.SecurityRule `json:"select"`
	Query  security_rule.SecurityRule `json:"query"`
}
type TransformRules struct {
	TransformInput  TransformRule `json:"transform_input"`
	TransformOutput TransformRule `json:"transform_output"`
}
type TransformRule struct {
	RuleType string                `json:"rule_type"`
	ApplyOn  []string              `json:"apply_on"`
	Rules    []TransformRuleDetail `json:"rules"`
}

type TransformRuleDetail struct {
	CustomRule         security_rule.CustomRule `json:"custom_rule"`
	ForceColumnValues  map[string]string        `json:"force_column_values"`
	RemoveColumnValues []string                 `json:"remove_column_values"`
	ComplexScript      string                   `json:"complex_script"`
	RuleRank           int                      `json:"rule_rank"`
}

type DbConfig struct {
	Host          string        `json:"host" eru:"required"`
	Port          string        `json:"port" eru:"required"`
	User          string        `json:"user" eru:"required"`
	Password      string        `json:"password" eru:"required"`
	DefaultDB     string        `json:"default_db" eru:"required"`
	DefaultSchema string        `json:"default_schema" eru:"required"`
	DriverConfig  DriverConfig  `json:"driver_config" eru:"required"`
	OtherDbConfig OtherDbConfig `json:"other_db_config"`
}

type DriverConfig struct {
	MaxOpenConns    int           `json:"max_open_conns" eru:"required"`
	MaxIdleConns    int           `json:"max_idle_conns" eru:"required"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime" eru:"required"`
}

type OtherDbConfig struct {
	RowLimit     int `json:"row_limit"`
	QueryTimeOut int `json:"query_time_out"`
}

type QueryResultMaker struct {
	QueryLevel    int
	QuerySubLevel []int
	MainTableName string
	MainAliasName string
	Tables        [][]Tables
	SQLQuery      string
}

type MutationResultMaker struct {
	MainTableName   string
	MainAliasName   string
	MutationRecords []MutationRecord
	MutationReturn  MutationReturn
	QueryType       string
	SingleTxn       bool
	OpenTxn         bool
	CloseTxn        bool
	TxnFlag         bool
	IsNested        bool
	DBQuery         string
	PreparedQuery   bool
}

type MutationRecord struct {
	Cols            string
	NonNestedCols   string
	NonNestedValues []interface{}
	UpdatedCols     string
	ColsPlaceholder string
	Values          []interface{}
	ChildRecords    map[string][]MutationRecord
	TableJoins      map[string]TableJoins
	DBQuery         string
}

type MutationReturn struct {
	ReturnError      bool
	ReturnDoc        bool
	ReturnErrorAlias string
	ReturnDocAlias   string
	ReturnFields     string
}

// tables used in query
type Tables struct {
	Name     string
	Nested   bool
	SqlQuery string
}

func (ePrj *ExtendedProject) UnmarshalJSON(b []byte) error {
	logs.Logger.Info("UnMarshal ExtendedProject - Start")
	ctx := context.Background()
	var ePrjMap map[string]*json.RawMessage
	err := json.Unmarshal(b, &ePrjMap)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}

	projectId := ""
	if _, ok := ePrjMap["project_id"]; ok {
		if ePrjMap["project_id"] != nil {
			err = json.Unmarshal(*ePrjMap["project_id"], &projectId)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			ePrj.ProjectId = projectId
		}
	}

	var ps ProjectSettings
	if _, ok := ePrjMap["project_settings"]; ok {
		if ePrjMap["project_settings"] != nil {
			err = json.Unmarshal(*ePrjMap["project_settings"], &ps)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			ePrj.ProjectSettings = ps
		}
	}

	var vars store.Variables
	if _, ok := ePrjMap["variables"]; ok {
		if ePrjMap["variables"] != nil {
			err = json.Unmarshal(*ePrjMap["variables"], &vars)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			ePrj.Variables = vars
		}
	}

	var ds map[string]*DataSource
	if _, ok := ePrjMap["data_sources"]; ok {
		if ePrjMap["data_sources"] != nil {
			err = json.Unmarshal(*ePrjMap["data_sources"], &ds)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			ePrj.DataSources = ds
		}
	}

	var mq map[string]*MyQuery
	if _, ok := ePrjMap["my_queries"]; ok {
		if ePrjMap["my_queries"] != nil {
			err = json.Unmarshal(*ePrjMap["my_queries"], &mq)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			ePrj.MyQueries = mq
		}
	}

	var smObj map[string]*json.RawMessage
	var smJson *json.RawMessage
	if _, ok := ePrjMap["secret_manager"]; ok {
		if ePrjMap["secret_manager"] != nil {
			err = json.Unmarshal(*ePrjMap["secret_manager"], &smObj)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			err = json.Unmarshal(*ePrjMap["secret_manager"], &smJson)
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
				err = smI.MakeFromJson(ctx, smJson)
				if err == nil {
					ePrj.SecretManager = smI
				} else {
					return err
				}
			} else {
				logs.WithContext(ctx).Info("ignoring secret manager as sm_store_type attribute not found")
			}
		} else {
			logs.WithContext(ctx).Info("secret manager attribute is nil")
		}
	} else {
		logs.WithContext(ctx).Info("secret manager attribute not found in store")
	}

	return nil
}

func (ds *DataSource) GetTableJoins(ctx context.Context, parentTableName string, childTableName string, otherTables map[string]string) (TableJoins, error) {
	logs.WithContext(ctx).Debug("GetTableJoins - Start")
	// TODO if schema is not passed with table name then compare with default schema set at datasource level
	tj := TableJoins{}
	if _, ok := ds.SchemaTables[parentTableName]; !ok {
		return tj, errors.New(fmt.Sprint(parentTableName, " table not found"))
	}
	if _, ok := ds.SchemaTables[childTableName]; !ok {
		return tj, errors.New(fmt.Sprint(childTableName, " table not found"))
	}
	tempKey := fmt.Sprint(parentTableName, "___", childTableName)
	tempKey1 := fmt.Sprint(childTableName, "___", parentTableName)
	if val, ok := ds.TableJoins[tempKey]; !ok {
		if val, ok := ds.TableJoins[tempKey1]; !ok {
			logs.WithContext(ctx).Info(fmt.Sprint("table joins not found for ", parentTableName, " and ", childTableName))
			newOtherTables := make(map[string]string)
			for k, _ := range otherTables {
				if k != parentTableName && k != childTableName {
					newOtherTables[k] = ""
				}
			}
			newParentTableName := ""
			finalOtherTables := make(map[string]string)
			for k, _ := range newOtherTables {
				if newParentTableName == "" {
					newParentTableName = k
				} else {
					finalOtherTables[k] = ""
				}
			}
			return ds.GetTableJoins(ctx, newParentTableName, childTableName, finalOtherTables)
			//return tj, errors.New(fmt.Sprint("table joins not found for ", parentTableName, " and ", childTableName))

		} else {
			tj = *val
			//swaping so the consumer of this function will always get child details as table 2 details and parent details as table 1 details
			tempTableName := tj.Table1Name
			tempTableCols := tj.Table1Cols
			tj.Table1Name = tj.Table2Name
			tj.Table1Cols = tj.Table2Cols
			tj.Table2Name = tempTableName
			tj.Table2Cols = tempTableCols
		}
	} else {
		tj = *val
	}
	return tj, nil
}

func (ds *DataSource) AddTableJoins(ctx context.Context, tj *TableJoins) {
	logs.WithContext(ctx).Debug("AddTableJoins - Start")
	tempKey := fmt.Sprint(tj.Table1Name, "___", tj.Table2Name)
	if ds.TableJoins == nil {
		ds.TableJoins = make(map[string]*TableJoins)
	}
	ds.TableJoins[tempKey] = tj
}
func (ds *DataSource) RemoveTableJoins(ctx context.Context, tj *TableJoins) {
	logs.WithContext(ctx).Debug("RemoveTableJoins - Start")
	tempKey := fmt.Sprint(tj.Table1Name, "___", tj.Table2Name)
	delete(ds.TableJoins, tempKey)
}

func (tj *TableJoins) GetOnClause(ctx context.Context) (res map[string]interface{}) {
	logs.WithContext(ctx).Debug("GetOnClause - Start")
	onClause := make(map[string]interface{})
	for i := 0; i < len(tj.Table1Cols); i++ {
		k := fmt.Sprint(tj.Table1Name, ".", tj.Table1Cols[i])
		kk := fmt.Sprint(tj.Table2Name, ".", tj.Table2Cols[i])
		onClause[k] = kk
	}
	if tj.ComplexCondition != nil {
		for k, v := range tj.ComplexCondition {
			onClause[k] = v
		}
	}
	res = make(map[string]interface{})
	res["on"] = onClause

	return res
}

func (ds *DataSource) CreateTable(ctx context.Context, tableName string, tableObj map[string]TableColsMetaData) (err error) {
	logs.WithContext(ctx).Debug("CreateTable - Start")
	return
}

func (prj *ExtendedProject) CompareProject(ctx context.Context, compareProject ExtendedProject) (StoreCompare, error) {
	storeCompare := StoreCompare{}
	storeCompare.CompareVariables(ctx, prj.Variables, compareProject.Variables)
	storeCompare.CompareSecretManager(ctx, prj.SecretManager, compareProject.SecretManager)

	var diffR utils.DiffReporter
	if !cmp.Equal(prj.ProjectSettings, compareProject.ProjectSettings, cmp.Reporter(&diffR)) {
		if storeCompare.MismatchSettings == nil {
			storeCompare.MismatchSettings = make(map[string]interface{})
		}
		storeCompare.MismatchSettings["settings"] = diffR.Output()
	}

	for _, mq := range prj.MyQueries {
		var diffR utils.DiffReporter
		qFound := false
		for _, cq := range compareProject.MyQueries {
			if mq.QueryName == cq.QueryName {
				qFound = true
				if !cmp.Equal(mq, cq, cmp.Reporter(&diffR)) {
					if storeCompare.MismatchQueries == nil {
						storeCompare.MismatchQueries = make(map[string]interface{})
					}
					storeCompare.MismatchQueries[mq.QueryName] = diffR.Output()
				}
				break
			}
		}
		if !qFound {
			storeCompare.DeleteQueries = append(storeCompare.DeleteQueries, mq.QueryName)
		}
	}

	for _, cq := range compareProject.MyQueries {
		qFound := false
		for _, mq := range prj.MyQueries {
			if mq.QueryName == cq.QueryName {
				qFound = true
				break
			}
		}
		if !qFound {
			storeCompare.NewQueries = append(storeCompare.NewQueries, cq.QueryName)
		}
	}

	//compare datasources
	for _, md := range prj.DataSources {
		var diffR utils.DiffReporter
		dsFound := false
		for _, cd := range compareProject.DataSources {
			if md.DbAlias == cd.DbAlias {
				dsFound = true
				if !cmp.Equal(md, cd, cmpopts.IgnoreFields(DataSource{}, "Con", "SchemaTables", "SchemaTablesTransformation", "TableJoins"), cmpopts.IgnoreFields(TableColsMetaData{}, "ColPosition"), cmp.Reporter(&diffR)) {
					if storeCompare.MismatchDataSources == nil {
						storeCompare.MismatchDataSources = make(map[string]interface{})
					}
					storeCompare.MismatchDataSources[md.DbAlias] = diffR.Output()
				}

				for mstKey, mst := range md.SchemaTables {
					var diffSt utils.DiffReporter
					stFound := false
					for cstKey, cst := range cd.SchemaTables {
						if mstKey == cstKey {
							stFound = true
							if !cmp.Equal(mst, cst, cmpopts.IgnoreFields(TableColsMetaData{}, "ColPosition"), cmp.Reporter(&diffSt)) {
								if storeCompare.MismatchTables == nil {
									storeCompare.MismatchTables = make(map[string]interface{})
								}
								storeCompare.MismatchTables[mstKey] = diffSt.Output()
							}
							break
						}
					}
					if !stFound {
						storeCompare.DeleteTables = append(storeCompare.DeleteTables, mstKey)
					}
				}
				for cstK, _ := range cd.SchemaTables {
					sFound := false
					for mstK, _ := range md.SchemaTables {
						if mstK == cstK {
							sFound = true
							break
						}
					}
					if !sFound {
						storeCompare.NewTables = append(storeCompare.NewTables, cstK)
					}
				}

				for mstKey, mst := range md.SchemaTablesSecurity {
					var diffSt utils.DiffReporter
					stFound := false
					for cstKey, cst := range cd.SchemaTablesSecurity {
						if mstKey == cstKey {
							stFound = true
							if !cmp.Equal(mst, cst, cmp.Reporter(&diffSt)) {
								if storeCompare.MismatchTableSecurity == nil {
									storeCompare.MismatchTableSecurity = make(map[string]interface{})
								}
								storeCompare.MismatchTableSecurity[mstKey] = diffSt.Output()
							}
							break
						}
					}
					if !stFound {
						storeCompare.DeleteTableSecurity = append(storeCompare.DeleteTableSecurity, mstKey)
					}
				}
				for cstK, _ := range cd.SchemaTablesSecurity {
					sFound := false
					for mstK, _ := range md.SchemaTablesSecurity {
						if mstK == cstK {
							sFound = true
							break
						}
					}
					if !sFound {
						storeCompare.NewTableSecurity = append(storeCompare.NewTableSecurity, cstK)
					}
				}

				for mstKey, mst := range md.SchemaTablesTransformation {
					var diffSt utils.DiffReporter
					stFound := false
					for cstKey, cst := range cd.SchemaTablesTransformation {
						if mstKey == cstKey {
							stFound = true
							if !cmp.Equal(mst, cst, cmp.Reporter(&diffSt)) {
								if storeCompare.MismatchTableTransformation == nil {
									storeCompare.MismatchTableTransformation = make(map[string]interface{})
								}
								storeCompare.MismatchTableTransformation[mstKey] = diffSt.Output()
							}
							break
						}
					}
					if !stFound {
						storeCompare.DeleteTableTransformation = append(storeCompare.DeleteTableTransformation, mstKey)
					}
				}
				for cstK, _ := range cd.SchemaTablesTransformation {
					sFound := false
					for mstK, _ := range md.SchemaTablesTransformation {
						if mstK == cstK {
							sFound = true
							break
						}
					}
					if !sFound {
						storeCompare.NewTableTransformation = append(storeCompare.NewTableTransformation, cstK)
					}
				}

				for mstKey, mst := range md.TableJoins {
					var diffSt utils.DiffReporter
					stFound := false
					for cstKey, cst := range cd.TableJoins {
						if mstKey == cstKey {
							stFound = true
							if !cmp.Equal(*mst, *cst, cmp.Reporter(&diffSt)) {
								if storeCompare.MismatchJoins == nil {
									storeCompare.MismatchJoins = make(map[string]interface{})
								}
								storeCompare.MismatchJoins[mstKey] = diffSt.Output()
							}
							break
						}
					}
					if !stFound {
						storeCompare.DeleteJoins = append(storeCompare.DeleteJoins, mstKey)
					}
				}
				for cstK, _ := range cd.TableJoins {
					sFound := false
					for mstK, _ := range md.TableJoins {
						if mstK == cstK {
							sFound = true
							break
						}
					}
					if !sFound {
						storeCompare.NewJoins = append(storeCompare.NewJoins, cstK)
					}
				}

				break
			}
		}
		if !dsFound {
			storeCompare.DeleteDataSources = append(storeCompare.DeleteDataSources, md.DbAlias)
		}
	}
	for _, cd := range compareProject.DataSources {
		dFound := false
		for _, md := range prj.DataSources {
			if md.DbAlias == cd.DbAlias {
				dFound = true
				break
			}
		}
		if !dFound {
			storeCompare.NewDataSources = append(storeCompare.NewDataSources, cd.DbAlias)
		}
	}

	return storeCompare, nil
}
