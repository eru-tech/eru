package module_model

import (
	"context"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-security-rule/security_rule"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jmoiron/sqlx"
	"time"
)

const (
	RULETYPE_NONE     = "none"
	RULETYPE_ALWAYS   = "always"
	RULETYPE_CUSTOM   = "custom"
	RULEPREFIX_TOKEN  = "token"
	RULEPREFIX_DOCS   = "docs"
	RULEPREFIX_NONE   = "none"
	QUERY_TYPE_INSERT = "insert"
	QUERY_TYPE_UPDATE = "update"
	QUERY_TYPE_DELETE = "delete"
	QUERY_TYPE_SELECT = "select"
)

type ModuleProjectI interface {
	CompareProject(ctx context.Context, compareProject Project) (StoreCompare, error)
}

type StoreCompare struct {
	DeleteQueries       []string
	NewQueries          []string
	MismatchQuries      map[string]interface{}
	DeleteDataSources   []string
	NewDataSources      []string
	MismatchDataSources map[string]interface{}
}

type Project struct {
	ProjectId       string                 `eru:"required"`
	DataSources     map[string]*DataSource //DB alias is the key
	MyQueries       map[string]*MyQuery    //queryName is key
	ProjectSettings ProjectSettings
}
type ProjectSettings struct {
	AesKey    string
	ClaimsKey string
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
	QueryName    string
	Query        string
	Vars         map[string]interface{}
	QueryType    string
	DBAlias      string
	ReadWrite    string
	Cols         string
	SecurityRule security_rule.SecurityRule
}

type DataSource struct {
	DbAlias                    string                                  `eru:"required"`
	DbType                     string                                  `eru:"required"`
	DbName                     string                                  `eru:"required"`
	DbConfig                   DbConfig                                `eru:"required"`
	SchemaTables               map[string]map[string]TableColsMetaData //tableName is the key
	OtherTables                map[string]map[string]TableColsMetaData `json:"-"` //tableName is the key
	SchemaTablesSecurity       map[string]SecurityRules
	SchemaTablesTransformation map[string]TransformRules
	TableJoins                 map[string]*TableJoins
	Con                        *sqlx.DB `json:"-"`
	ConStatus                  bool
	DbSecurityRules            SecurityRules
}

type TableJoins struct {
	Table1Name       string
	Table1Cols       []string
	Table2Name       string
	Table2Cols       []string
	IsActive         bool
	IsCustom         bool
	ComplexCondition map[string]interface{}
}

type TableColsMetaData struct {
	TblSchema         string `eru:"required"`
	TblName           string `eru:"required"`
	ColName           string `eru:"required"`
	DataType          string
	OwnDataType       string `eru:"required"`
	PrimaryKey        bool   `eru:"required"`
	IsUnique          bool   `eru:"required"`
	PkConstraintName  string
	UqConstraintName  string
	IsNullable        bool `eru:"required"`
	ColPosition       int
	DefaultValue      string
	AutoIncrement     bool
	CharMaxLength     int
	NumericPrecision  string
	NumericScale      int
	DatetimePrecision int
	FkConstraintName  string
	FkDeleteRule      string
	FkTblSchema       string
	FkTblName         string
	FkColName         string
	ColumnMasking     ColumnMasking
}

type ColumnMasking struct {
	MaskingType string
	MaskingRule string
	CustomRule  security_rule.CustomRule
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
	Create security_rule.SecurityRule
	Drop   security_rule.SecurityRule
	Alter  security_rule.SecurityRule
	Insert security_rule.SecurityRule
	Update security_rule.SecurityRule
	Delete security_rule.SecurityRule
	Select security_rule.SecurityRule
	Query  security_rule.SecurityRule
}
type TransformRules struct {
	TransformInput  TransformRule
	TransformOutput TransformRule
}
type TransformRule struct {
	RuleType string
	ApplyOn  []string
	Rules    []TransformRuleDetail
}

type TransformRuleDetail struct {
	CustomRule         security_rule.CustomRule
	ForceColumnValues  map[string]string
	RemoveColumnValues []string
	ComplexScript      string
	RuleRank           int
}

type DbConfig struct {
	Host          string       `eru:"required"`
	Port          string       `eru:"required"`
	User          string       `eru:"required"`
	Password      string       `eru:"required"`
	DefaultDB     string       `eru:"required"`
	DefaultSchema string       `eru:"required"`
	DriverConfig  DriverConfig `eru:"required"`
	OtherDbConfig OtherDbConfig
}

type DriverConfig struct {
	MaxOpenConns    int           `eru:"required"`
	MaxIdleConns    int           `eru:"required"`
	ConnMaxLifetime time.Duration `eru:"required"`
}

type OtherDbConfig struct {
	RowLimit       int
	QueryTimeOut   int
	AllowDropTable bool
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

func (prj *Project) CompareProject(ctx context.Context, compareProject Project) (StoreCompare, error) {
	storeCompare := StoreCompare{}
	for _, mq := range prj.MyQueries {
		var diffR utils.DiffReporter
		qFound := false
		for _, cq := range compareProject.MyQueries {
			if mq.QueryName == cq.QueryName {
				qFound = true
				if !cmp.Equal(mq, cq, cmp.Reporter(&diffR)) {
					if storeCompare.MismatchQuries == nil {
						storeCompare.MismatchQuries = make(map[string]interface{})
					}
					storeCompare.MismatchQuries[mq.QueryName] = diffR.Output()
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
				if !cmp.Equal(md, cd, cmpopts.IgnoreFields(DataSource{}, "Con"), cmpopts.IgnoreFields(TableColsMetaData{}, "ColPosition"), cmp.Reporter(&diffR)) {
					if storeCompare.MismatchDataSources == nil {
						storeCompare.MismatchDataSources = make(map[string]interface{})
					}
					storeCompare.MismatchDataSources[md.DbAlias] = diffR.Output()
				}
				break
			}
		}
		if !dsFound {
			storeCompare.DeleteQueries = append(storeCompare.DeleteDataSources, md.DbAlias)
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

	}
	return storeCompare, nil
}
