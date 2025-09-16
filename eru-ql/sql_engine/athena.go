package sqlengine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/athena/types"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	common_types "github.com/eru-tech/eru/eru-ql/common_types"
)

const (
	AuthTypeSecret = "SECRET"
	AuthTypeIAM    = "IAM"
)

type AthenaSQLEngine struct {
	SQLEngine
	session        *athena.Client
	Region         string `json:"region" eru:"required"`
	Authentication string `json:"authentication" eru:"required"`
	Key            string `json:"key" eru:"required"`
	Secret         string `json:"secret" eru:"required"`
	workgroup      string
	outputS3       string
	database       string
}

func (athenaSQLEngine *AthenaSQLEngine) Init(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("Init - Start")
	awsConf, awsConfErr := config.LoadDefaultConfig(ctx,
		config.WithRegion(athenaSQLEngine.Region),
	)
	if awsConfErr != nil {
		err = awsConfErr
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	if athenaSQLEngine.Authentication == AuthTypeSecret {
		awsConf.Credentials = credentials.NewStaticCredentialsProvider(
			athenaSQLEngine.Key,
			athenaSQLEngine.Secret,
			"", // a token will be created when the session is used.
		)
	} else if athenaSQLEngine.Authentication == AuthTypeIAM {
		logs.WithContext(ctx).Info("connecting AWS Athena with IAM role")
		// do nothing - no new attributes to set in config
	}
	athenaSQLEngine.session = athena.NewFromConfig(awsConf)

	return err

}
func (athenaSQLEngine *AthenaSQLEngine) ExecuteQuery(ctx context.Context, query string) (output []map[string]interface{}, err error) {
	if athenaSQLEngine.session == nil {
		err = athenaSQLEngine.Init(ctx)
		if err != nil {
			return
		}
	}
	start, err := athenaSQLEngine.session.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString: aws.String(query),
		QueryExecutionContext: &types.QueryExecutionContext{
			Database: aws.String(athenaSQLEngine.database),
		},
		ResultConfiguration: &types.ResultConfiguration{OutputLocation: aws.String(athenaSQLEngine.outputS3)},
		WorkGroup:           aws.String(athenaSQLEngine.workgroup),
	})
	if err != nil {
		err = logs.Err(ctx, err, "failed to start query execution")
		return nil, err
	}

	// Poll
	var state string
	for {
		desc, err := athenaSQLEngine.session.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{
			QueryExecutionId: start.QueryExecutionId,
		})
		if err != nil {
			return nil, err
		}
		state = string(desc.QueryExecution.Status.State)
		if state == "SUCCEEDED" {
			break
		}
		if state == "FAILED" || state == "CANCELLED" {
			return nil, fmt.Errorf("athena query failed: %s", state)
		}
		time.Sleep(2 * time.Second)
	}

	// 3. Fetch results
	resOut, err := athenaSQLEngine.session.GetQueryResults(ctx, &athena.GetQueryResultsInput{
		QueryExecutionId: start.QueryExecutionId,
	})
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("failed to get query results: %v", err))
		return nil, err
	}

	for _, row := range resOut.ResultSet.Rows {
		for _, col := range row.Data {
			logs.WithContext(ctx).Info(fmt.Sprintf("%s\t", aws.ToString(col.VarCharValue)))
		}
	}
	return nil, nil
}

func (athenaSQLEngine *AthenaSQLEngine) MakeCreateTableSQL(ctx context.Context, tableName string, tableObj map[string]common_types.TableColsMetaData) (string, error) {
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
			dt = athenaSQLEngine.getErutoDBDataTypeMapping(ctx, v.OwnDataType)
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
	logs.WithContext(ctx).Info(query)
	return query, nil
}
func (athenaSQLEngine *AthenaSQLEngine) getDataTypeMapping(ctx context.Context, dataType string) string {
	logs.WithContext(ctx).Debug("getDataTypeMapping - Start")
	if athenaDataTypeMapping[strings.ToLower(dataType)] == "" {
		return "NotSupported"
	} else {
		return athenaDataTypeMapping[strings.ToLower(dataType)]
	}
}

func (athenaSQLEngine *AthenaSQLEngine) getErutoDBDataTypeMapping(ctx context.Context, dataType string) string {
	logs.WithContext(ctx).Debug("getErutoDBDataTypeMapping - Start")
	if athenaErutoDBDataTypeMapping[dataType] == "" {
		return "NotSupported"
	} else {
		return athenaErutoDBDataTypeMapping[dataType]
	}
}

var athenaDataTypeMapping = map[string]string{
	"BOOLEAN":   "Boolean",
	"INT":       "Integer",
	"BIGINT":    "BigInteger",
	"FLOAT":     "Float",
	"DOUBLE":    "Float",
	"DECIMAL":   "Decimal",
	"DATE":      "Date",
	"TIME":      "Time",
	"TIMESTAMP": "TimeWithZone",
	"STRING":    "Varchar",
	"VARCHAR":   "Varchar",
	"ARRAY":     "Array",
	"MAP":       "Map",
	"STRUCT":    "Struct",
	"BINARY":    "Binary",
	"UUID":      "UUID",
}
var athenaErutoDBDataTypeMapping = map[string]string{
	"Boolean":      "BOOLEAN",
	"Integer":      "INT",
	"BigInteger":   "BIGINT",
	"Float":        "FLOAT",
	"Decimal":      "DECIMAL",
	"Date":         "DATE",
	"Time":         "TIME",
	"TimeWithZone": "TIMESTAMP",
	"Varchar":      "VARCHAR",
	"Array":        "ARRAY",
	"Map":          "MAP",
	"Struct":       "STRUCT",
	"Binary":       "BINARY",
	"UUID":         "UUID",
}

/*
| Iceberg type   | Athena / Trino         | Spark SQL         | Flink SQL         | DuckDB         |
| -------------- | ---------------------- | ----------------- | ----------------- | -------------- |
| `boolean`      | `BOOLEAN`              | `BOOLEAN`         | `BOOLEAN`         | `BOOLEAN`      |
| `int`          | `INT`                  | `INT`             | `INT`             | `INTEGER`      |
| `long`         | `BIGINT`               | `BIGINT`          | `BIGINT`          | `BIGINT`       |
| `float`        | `FLOAT`                | `FLOAT`           | `FLOAT`           | `REAL`         |
| `double`       | `DOUBLE`               | `DOUBLE`          | `DOUBLE`          | `DOUBLE`       |
| `decimal(p,s)` | `DECIMAL(p,s)`         | `DECIMAL(p,s)`    | `DECIMAL(p,s)`    | `DECIMAL(p,s)` |
| `date`         | `DATE`                 | `DATE`            | `DATE`            | `DATE`         |
| `time`         | `TIME`                 | `TIME`            | `TIME`            | `TIME`         |
| `timestamp`    | `TIMESTAMP`            | `TIMESTAMP`       | `TIMESTAMP(3)`    | `TIMESTAMP`    |
| `string`       | `STRING`/`VARCHAR`     | `STRING`          | `STRING`          | `TEXT`         |
| `binary`       | `BINARY`               | `BINARY`          | `BYTES`           | `BLOB`         |
| `uuid`         | `UUID`                 | `STRING` (mapped) | `STRING` (mapped) | `UUID`         |
| `fixed(N)`     | ❌ not exposed directly | `BINARY(N)`       | `BYTES(N)`        | ❌              |
| `list<T>`      | `ARRAY<T>`             | `ARRAY<T>`        | `ARRAY<T>`        | `LIST<T>`      |
| `map<K,V>`     | `MAP<K,V>`             | `MAP<K,V>`        | `MAP<K,V>`        | `MAP<K,V>`     |
| `struct<...>`  | `STRUCT<...>`          | `STRUCT<...>`     | `ROW<...>`        | `STRUCT<...>`  |
*/
