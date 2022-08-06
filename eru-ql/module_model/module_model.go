package module_model

import (
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-security-rule/security_rule"
	"github.com/jmoiron/sqlx"
	"log"
	"time"
)

const (
	RULETYPE_NONE    = "none"
	RULETYPE_ALWAYS  = "always"
	RULETYPE_CUSTOM  = "custom"
	RULEPREFIX_TOKEN = "token"
)

type ModuleProjectI interface {
}

type Project struct {
	ProjectId     string                 `eru:"required"`
	DataSources   map[string]*DataSource //DB alias is the key
	MyQueries     map[string]*MyQuery    //queryName is key
	ProjectConfig ProjectConfig
}

type AesKey struct {
	Key string
	//Bits int
}

type ProjectConfig struct {
	AesKey         AesKey
	TokenSecret    TokenSecret
	ProjectGitRepo ProjectGitRepo
}

type TokenSecret struct {
	HeaderKey  string
	SecretAlgo string
	SecretKey  string
	JwkUrl     string
	Audience   []string
	Issuer     []string
}

type ProjectGitRepo struct {
	RepoName   string
	BranchName string
	AuthMode   string
	AuthKey    string
}

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

func (ds *DataSource) GetTableJoins(parentTableName string, childTableName string) (TableJoins, error) {
	//log.Print("inside VerifyChildTable")
	// TODO if schema is not passed with table name then compare with default schema set at datasource level
	//log.Print(parentTableName, " - ", childTableName)

	tj := TableJoins{}
	if _, ok := ds.SchemaTables[parentTableName]; !ok {
		//log.Print(parentTableName, " table not found")
		return tj, errors.New(fmt.Sprint(parentTableName, " table not found"))
	}
	if _, ok := ds.SchemaTables[childTableName]; !ok {
		//log.Print(childTableName, " table not found")
		return tj, errors.New(fmt.Sprint(childTableName, " table not found"))
	}
	tempKey := fmt.Sprint(parentTableName, "___", childTableName)
	tempKey1 := fmt.Sprint(childTableName, "___", parentTableName)

	if val, ok := ds.TableJoins[tempKey]; !ok {
		if val, ok := ds.TableJoins[tempKey1]; !ok {
			//log.Print("table joins not found for ",parentTableName," and ",childTableName)
			return tj, errors.New(fmt.Sprint("table joins not found for ", parentTableName, " and ", childTableName))
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
	log.Print(tj)
	return tj, nil
}

func (ds *DataSource) AddTableJoins(tj *TableJoins) {
	tempKey := fmt.Sprint(tj.Table1Name, "___", tj.Table2Name)
	ds.TableJoins[tempKey] = tj
}
func (ds *DataSource) RemoveTableJoins(tj *TableJoins) {
	tempKey := fmt.Sprint(tj.Table1Name, "___", tj.Table2Name)
	delete(ds.TableJoins, tempKey)
}

func (tj *TableJoins) GetOnClause() (res map[string]interface{}) {
	onClause := make(map[string]interface{})
	log.Print("len(tj.Table1Cols)  ==== ", len(tj.Table1Cols))
	for i := 0; i < len(tj.Table1Cols); i++ {
		k := fmt.Sprint(tj.Table1Name, ".", tj.Table1Cols[i])
		log.Print(k)
		kk := fmt.Sprint(tj.Table2Name, ".", tj.Table2Cols[i])
		log.Print(kk)
		onClause[k] = kk
	}
	if tj.ComplexCondition != nil {
		for k, v := range tj.ComplexCondition {
			onClause[k] = v
		}
	}
	res = make(map[string]interface{})
	res["on"] = onClause
	log.Print(res)

	return res
}

func (ds *DataSource) CreateTable(tableName string, tableObj map[string]TableColsMetaData) (err error) {

	return
}
