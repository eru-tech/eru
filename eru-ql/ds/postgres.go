package ds

import (
	"context"
	"errors"
	"fmt"

	"strings"

	"github.com/antlr4-go/antlr/v4"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	parser "github.com/eru-tech/eru/eru-ql/ds/parser"

	//eru_utils "github.com/eru-tech/eru/eru-utils"

	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var dbBlockedWords = []string{"--"}
var dbBlockedRegex = []string{}

type PostgresSqlMaker struct {
	SqlMaker
}

// Listener to traverse the parse tree
type Listener struct {
	*parser.BasePostgreSQLParserListener
	Columns       []string
	inFinalSelect bool // Flag to track if we're in the final SELECT
}

/* func (l *Listener) EnterColumnref(ctx *parser.ColumnrefContext) {
	// Check if this is the final SELECT (not part of a CTE)
	if ctx.GetParent() != nil {
		logs.WithContext(context.Background()).Info(fmt.Sprint(ctx.GetText()))
		 if _, ok := ctx.GetParent().(*parser.Relation_expr_opt_aliasContext); ok {
			l.inFinalSelect = true

		}
	}
} */

//func (l *Listener) EnterEveryRule(ctx antlr.ParserRuleContext) {
//	fmt.Print(ctx.GetRuleIndex(), " : ", ctx.GetText())
//}

func (pr *PostgresSqlMaker) GetBlockedWords() []string {
	return append(blockedWords, dbBlockedWords...)
}
func (pr *PostgresSqlMaker) GetBlockedRegex() []string {
	return append(blockedRegex, dbBlockedRegex...)
}
func (pr *PostgresSqlMaker) GetMakeJsonArrayFn() (string, error) {
	return " ?| array ", nil
}
func (pr *PostgresSqlMaker) GetMakeJsonArrayFnStr() (string, error) {
	return " ? ", nil
}

func (pr *PostgresSqlMaker) ExtractTableNames(ctx context.Context, query string) (resTablesInQuery module_model.TablesInQuery) {
	logs.WithContext(ctx).Debug("ExtractTableNames - Start")
	// Create an input stream
	//query = "select * from abc a left join xyz b on 1=1"
	is := antlr.NewInputStream(query)

	// Create the lexer and token stream
	lexer := parser.NewPostgreSQLLexer(is)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	// Create the parser
	p := parser.NewPostgreSQLParser(stream)

	// Parse the input (starting rule depends on your grammar, e.g., statement)
	tree := p.Root()
	//fmt.Println(tree.ToStringTree(nil, p))

	//tables := extractTableNames(tree)
	aliases := extractAliasNames(tree)
	tablesInQuery := extractTableAliasNames(tree, query)
	reaTablesInQuery := module_model.TablesInQuery{}
	// Walk the parse tree with a custom listener
	//listener := &Listener{}
	//antlr.ParseTreeWalkerDefault.Walk(listener, tree)

	//var filteredArray []string
	// Only keep table names that don't exist in aliases
	for _, table := range tablesInQuery.Tables {
		aliasFound := false
		for _, alias := range aliases {
			if strings.TrimSpace(table.TableName) == strings.TrimSpace(alias) {
				aliasFound = true
				break
			}
		}
		if !aliasFound {
			reaTablesInQuery.Tables = append(reaTablesInQuery.Tables, table)
		}
	}
	//tables = eru_utils.UniqueStrings(filteredArray)
	//logs.WithContext(ctx).Info(fmt.Sprint("Tables : ", tables))
	return reaTablesInQuery
}
func (pr *PostgresSqlMaker) DefaultSchemaName() string {
	return "public."
}
func extractTableNames(tree antlr.Tree) (tableNames []string) {
	switch ctx := tree.(type) {
	case *parser.Relation_exprContext:
		tableNames = append(tableNames, ctx.GetText())
	default:
		for i := 0; i < tree.GetChildCount(); i++ {
			tableNames = append(tableNames, extractTableNames(tree.GetChild(i))...)
		}
	}
	return tableNames
}
func extractAliasNames(tree antlr.Tree) (aliases []string) {
	switch ctx := tree.(type) {
	case *parser.Common_table_exprContext:
		aliases = append(aliases, strings.Split(ctx.GetText(), "as(")[0])
	default:
		for i := 0; i < tree.GetChildCount(); i++ {
			aliases = append(aliases, extractAliasNames(tree.GetChild(i))...)
		}
	}
	return aliases
}

func extractTableAliasNames(tree antlr.Tree, query string) (tablesInQuery module_model.TablesInQuery) {

	tablesInQuery = module_model.TablesInQuery{}
	switch ctx := tree.(type) {
	case *parser.Table_refContext:
		startIndex := 0
		stopIndex := 0
		tn := ""
		alias := ""
		for _, v := range ctx.GetChildren() {
			switch ctx1 := v.(type) {
			case *parser.Relation_exprContext:
				startIndex = ctx1.GetStart().GetStart()
				stopIndex = ctx1.GetStop().GetStop() + 1
				tn = ctx1.GetText()
			case *parser.Alias_clauseContext:
				stopIndex = ctx1.GetStop().GetStop() + 1
				alias = ctx1.GetText()

			default:
				for i := 0; i < tree.GetChildCount(); i++ {
					childTablesInQuery := extractTableAliasNames(tree.GetChild(i), query)
					for _, v := range childTablesInQuery.Tables {
						aliasFound := false
						for _, vv := range tablesInQuery.Tables {
							if vv.TableKey == v.TableKey {
								aliasFound = true
								break
							}
						}
						if !aliasFound {
							tablesInQuery.Tables = append(tablesInQuery.Tables, v)
						}
					}
				}
			}
		}
		if tn != "" {
			if alias == "" {
				alias = tn
			}
			tmpStopIndex := stopIndex + 5
			if tmpStopIndex > len(query) {
				tmpStopIndex = len(query)
			}

			tablesInQuery.Tables = append(tablesInQuery.Tables, module_model.TableInQuery{AliasName: alias, TableName: tn, TableKey: query[startIndex:stopIndex], TableKeyPrefix: query[startIndex-5 : startIndex], TableKeySuffix: query[stopIndex:tmpStopIndex]})
		}
	default:
		for i := 0; i < tree.GetChildCount(); i++ {
			childTablesInQuery := extractTableAliasNames(tree.GetChild(i), query)
			for _, v := range childTablesInQuery.Tables {
				aliasFound := false
				for _, vv := range tablesInQuery.Tables {
					if vv.TableKey == v.TableKey {
						aliasFound = true
						break
					}
				}
				if !aliasFound {
					tablesInQuery.Tables = append(tablesInQuery.Tables, v)
				}
			}
		}
	}
	return
}

func (pr *PostgresSqlMaker) GetPreparedQueryPlaceholder(ctx context.Context, rowCount int, colCount int, single bool) string {
	logs.WithContext(ctx).Debug("GetPreparedQueryPlaceholder - Start")
	var rowArray []string
	startNo := 0
	if single {
		return fmt.Sprint("$", colCount)
	}
	for r := 1; r <= rowCount; r++ {
		var colArray []string
		for c := 1; c <= colCount; c++ {
			startNo++
			colArray = append(colArray, fmt.Sprint("$", startNo))
		}
		rowArray = append(rowArray, fmt.Sprint("(", strings.Join(colArray, " , "), ")"))
	}
	return strings.Join(rowArray, " , ")
}

func (pr *PostgresSqlMaker) GetTableMetaDataSQL(ctx context.Context) string {
	logs.WithContext(ctx).Debug("GetTableMetaDataSQL - Start")
	return postgresTableMetaDataSQL
}

func (pr *PostgresSqlMaker) MakeCreateTableSQL(ctx context.Context, tableName string, tableObj map[string]module_model.TableColsMetaData) (string, error) {
	logs.WithContext(ctx).Debug("MakeCreateTableSQL - Start")
	var cols []string
	var fks []string
	pkCon := make(map[string][]string)
	uqCon := make(map[string][]string)
	pkConName := fmt.Sprint("pk_", strings.Replace(tableName, ".", "___", 1))
	for _, v := range tableObj {
		dt := "serial"
		//pk := ""
		//uk := ""
		nl := ""
		if !v.IsNullable {
			nl = " not null "
		}
		if v.IsUnique && !v.PrimaryKey {
			uqCon[v.UqConstraintName] = append(uqCon[v.UqConstraintName], v.ColName)
		}
		if !v.PrimaryKey {
			dt = pr.getErutoDBDataTypeMapping(ctx, v.OwnDataType)
			if dt == "NotSupported" {
				return "", errors.New(fmt.Sprint("Unsupported Datatype : ", v.OwnDataType))
			}
		} else {
			pkCon[pkConName] = append(pkCon[pkConName], v.ColName)
			//pk = " primary key "
			nl = ""
		}

		switch dt {
		case "numeric":
			dt = fmt.Sprint(dt, " (", v.NumericPrecision, ")")
		case "character", "character varying":
			dt = fmt.Sprint(dt, " (", v.CharMaxLength, ")")
		case "timestamp without time zone", "timestamp with time zone", "time with time zone":
			dt = fmt.Sprint(dt, " [", v.DatetimePrecision, "]")
		}

		if v.FkTblName != "" {
			fks = append(fks, fmt.Sprint("constraint fk_", v.TblName, v.ColName, " foreign key (", v.ColName, ") references ", v.FkTblSchema, ".", v.FkTblName, "(", v.FkColName, ")"))
		}
		cols = append(cols, fmt.Sprint(v.ColName, " ", dt, nl))
	}
	var pk []string
	for k, v := range pkCon {
		pk = append(pk, fmt.Sprint("constraint ", k, " primary key (", strings.Join(v, " , "), ")"))
	}
	if len(pk) > 0 {
		cols = append(cols, strings.Join(pk, " , "))
	}
	var uq []string
	for k, v := range uqCon {
		uq = append(uq, fmt.Sprint("constraint ", k, " unique (", strings.Join(v, " , "), ")"))
	}
	if len(uq) > 0 {
		cols = append(cols, strings.Join(uq, " , "))
	}
	if len(fks) > 0 {
		cols = append(cols, strings.Join(fks, " , "))
	}

	query := fmt.Sprint("create table ", tableName, " (", strings.Join(cols, " , "), " )")
	return query, nil
}

func (pr *PostgresSqlMaker) MakeDropTableSQL(ctx context.Context, tableName string) (string, error) {
	logs.WithContext(ctx).Debug("MakeDropTableSQL - Start")
	return fmt.Sprint("drop table ", tableName), nil
}

func (pr *PostgresSqlMaker) CreateConn(ctx context.Context, dataSource *module_model.DataSource) error {
	logs.WithContext(ctx).Debug("CreateConn - Start")
	connString := fmt.Sprint("postgres://", dataSource.DbConfig.User, ":", dataSource.DbConfig.Password, "@", dataSource.DbConfig.Host, ":", dataSource.DbConfig.Port, "/", dataSource.DbConfig.DefaultDB, "?sslmode=disable")
	db, err := sqlx.Open("postgres", connString)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		dataSource.ConStatus = false
		return err
	}
	logs.WithContext(ctx).Info("db connection was successfully done for fetch dummy query")
	_, err = db.Queryx("select 1")
	if err != nil {
		dataSource.ConStatus = false
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	logs.WithContext(ctx).Info("dummy query success - setting con as true")
	dataSource.Con = db
	dataSource.ConStatus = true
	return nil
}

func (pr *PostgresSqlMaker) CheckMe(ctx context.Context) {
	logs.WithContext(ctx).Info("I am PostgresSqlMaker")
	pr.ChildChange = "changed by PostgresSqlMaker"
}
func (pr *PostgresSqlMaker) MakeJsonColumn(jsonField string, jsonKey string) string {
	return fmt.Sprint(jsonField, "->>'", jsonKey, "'")
}
func (pr *PostgresSqlMaker) AddLimitSkipClause(ctx context.Context, query string, limit int, skip int, globalLimit int) (newQuery string) {
	logs.WithContext(ctx).Debug("AddLimitSkipClause - Start")
	if limit > 0 {
		newQuery = fmt.Sprint(query, " limit ", limit)
	} else {
		newQuery = fmt.Sprint(query, " limit ", globalLimit)
	}
	if skip > 0 {
		newQuery = fmt.Sprint(newQuery, " offset ", skip)
	}
	return newQuery
}

func (pr *PostgresSqlMaker) getDataTypeMapping(ctx context.Context, dataType string) string {
	logs.WithContext(ctx).Debug("getDataTypeMapping - Start")
	if postgresDataTypeMapping[strings.ToLower(dataType)] == "" {
		return "NotSupported"
	} else {
		return postgresDataTypeMapping[strings.ToLower(dataType)]
	}
}

func (pr *PostgresSqlMaker) getErutoDBDataTypeMapping(ctx context.Context, dataType string) string {
	logs.WithContext(ctx).Debug("getErutoDBDataTypeMapping - Start")
	if postgresErutoDBDataTypeMapping[dataType] == "" {
		return "NotSupported"
	} else {
		return postgresErutoDBDataTypeMapping[dataType]
	}
}

const postgresTableMetaDataSQL = `select CAST(c.table_schema as VARCHAR) TblSchema,
       CAST(c.table_name as VARCHAR) TblName,
       CAST(c.column_name as VARCHAR) ColName,
       CAST(c.data_type as VARCHAR) DataType,
	   '' OwnDataType,
	   CAST(CASE WHEN pk.constraint_type is not null then 'true' else 'false' end as VARCHAR) PrimaryKey,
		CAST(CASE WHEN uq.constraint_type is not null then 'true' else 'false' end as VARCHAR) IsUnique,
		CAST(COALESCE(pk.constraint_name,'') as VARCHAR) PkConstraintName,
		CAST(COALESCE(uq.constraint_name,'') as VARCHAR) UqConstraintName,
	   CAST(CASE WHEN CAST(c.is_nullable as VARCHAR) = 'YES' THEN 'true' ELSE 'false' END as VARCHAR) IsNullable,       
       c.ordinal_position ColPosition,
       CAST(SPLIT_PART(REPLACE(COALESCE(c.column_default,''),'''',''), '::', 1) as VARCHAR) DefaultValue,
       CAST(CASE WHEN UPPER(c.column_default) like 'NEXTVAL%' then 'true' else 'false' end as VARCHAR) AutoIncrement,
       COALESCE(c.character_maximum_length,-1) CharMaxLength,
       COALESCE(c.numeric_precision,0)||','||COALESCE(c.numeric_scale,0) NumericPrecision,
       COALESCE(c.numeric_scale,0) NumericScale,
       COALESCE(c.datetime_precision,0) DatetimePrecision,
       CAST(COALESCE(fk.constraint_name,'') as VARCHAR) FkConstraintName,
       CAST(COALESCE(fk.delete_rule,'') as VARCHAR) FkDeleteRule,
       CAST(COALESCE(fk.foreign_table_schema,'')  as VARCHAR) FkTblSchema,
       CAST(COALESCE(fk.foreign_table_name,'') as VARCHAR) FkTblName,
       CAST(COALESCE(fk.foreign_column_name,'') as VARCHAR) FkColName
FROM information_schema.columns c
LEFT JOIN (select tc.constraint_type, tc.constraint_name , tc.table_schema, tc.table_name, kcu.column_name from information_schema.table_constraints tc
INNER JOIN information_schema.key_column_usage kcu 
            	ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema) pk on pk.constraint_type='PRIMARY KEY' and pk.table_schema=c.table_schema
				and pk.table_name=c.table_name and pk.column_name=c.column_name
LEFT JOIN (select tc.constraint_type, tc.constraint_name , tc.table_schema, tc.table_name, kcu.column_name from information_schema.table_constraints tc
INNER JOIN information_schema.key_column_usage kcu 
            	ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema) uq on uq.constraint_type='UNIQUE' and uq.table_schema=c.table_schema
				and uq.table_name=c.table_name and uq.column_name=c.column_name				
LEFT JOIN (SELECT tc.table_schema, tc.constraint_name, tc.table_name, kcu.column_name,
			ccu.table_schema foreign_table_schema, ccu.table_name foreign_table_name,
			ccu.column_name foreign_column_name, rc.delete_rule delete_rule
            FROM information_schema.table_constraints tc
            INNER JOIN information_schema.key_column_usage kcu 
            	ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
            INNER JOIN information_schema.constraint_column_usage ccu
				ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
			INNER JOIN  information_schema.referential_constraints rc
            	ON rc.constraint_name = tc.constraint_name AND rc.constraint_schema = tc.table_schema
			WHERE tc.constraint_type = 'FOREIGN KEY' ) fk
ON fk.table_name = c.table_name AND fk.column_name = c.column_name AND fk.table_schema = c.table_schema
WHERE  c.table_schema not in ('information_schema','pg_catalog')
ORDER BY c.ordinal_position`

//erudevsh

var postgresDataTypeMapping = map[string]string{
	"smallint":                    "SmallInteger",
	"integer":                     "Integer",
	"int4":                        "Integer",
	"int8":                        "Integer",
	"int16":                       "Integer",
	"int32":                       "Integer",
	"int64":                       "Integer",
	"int":                         "Integer",
	"bigint":                      "BigInteger",
	"numeric":                     "Decimal",
	"real":                        "Float",
	"double precision":            "Float",
	"varchar":                     "Varchar",
	"character varying":           "Varchar",
	"character":                   "Char",
	"text":                        "String",
	"timestamp":                   "DateTime",
	"timestamp without time zone": "DateTime",
	"timestamp with time zone":    "DateTimeWithZone",
	"date":                        "Date",
	"time without time zone":      "Time",
	"time with time zone":         "TimeWithZone",
	"boolean":                     "Boolean",
	"bool":                        "Boolean",
	"json":                        "JSON",
	"jsonb":                       "JSON"}

var postgresErutoDBDataTypeMapping = map[string]string{
	"SmallInteger":     "smallint",
	"Integer":          "integer",
	"BigInteger":       "bigint",
	"Decimal":          "numeric",
	"Float":            "double precision",
	"Varchar":          "character",
	"Char":             "character varying",
	"String":           "text",
	"DateTime":         "timestamp without time zone",
	"DateTimeWithZone": "timestamp with time zone",
	"Date":             "date",
	"Time":             "time with time zone",
	"TimeWithZone":     "time with time zone",
	"Boolean":          "boolean",
	"JSON":             "jsonb",
}
