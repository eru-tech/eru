package ds

import (
	"context"
	"errors"
	"strings"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	common_types "github.com/eru-tech/eru/eru-ql/common_types"
	"github.com/eru-tech/eru/eru-ql/module_model"
)

const (
	CatalogTypeS3Tables = "s3tables"
)

type IcebergSqlMaker struct {
	SqlMaker
}

func (ib *IcebergSqlMaker) CreateConn(ctx context.Context, dataSource *module_model.DataSource) error {
	logs.WithContext(ctx).Debug("CreateConn - Start")
	// Load a REST catalog (S3 Tables endpoint)

	if dataSource.IcebergConfig.CatalogType != CatalogTypeS3Tables {
		return errors.New("currently only s3tables is supported")
	}
	if dataSource.IcebergConfig.CatalogType == CatalogTypeS3Tables {
		return ib.S3TablesCreateConn(ctx, dataSource)
	}

	return nil
}

func (ib *IcebergSqlMaker) getDataTypeMapping(ctx context.Context, dataType string) string {
	logs.WithContext(ctx).Debug("getDataTypeMapping - Start")
	if icebergDataTypeMapping[strings.ToLower(dataType)] == "" {
		return "NotSupported"
	} else {
		return icebergDataTypeMapping[strings.ToLower(dataType)]
	}
}

func (ib *IcebergSqlMaker) getErutoDBDataTypeMapping(ctx context.Context, dataType string) string {
	logs.WithContext(ctx).Debug("getErutoDBDataTypeMapping - Start")
	if icebergErutoDBDataTypeMapping[dataType] == "" {
		return "NotSupported"
	} else {
		return icebergErutoDBDataTypeMapping[dataType]
	}
}
func (ib *IcebergSqlMaker) MakeCreateTableSQL(ctx context.Context, tableName string, tableObj map[string]common_types.TableColsMetaData) (string, error) {
	logs.WithContext(ctx).Debug("MakeCreateTableSQL - Start")
	return "", nil
}

func (ib *IcebergSqlMaker) ExecutePreparedQuery(ctx context.Context, query string, datasource *module_model.DataSource) (res map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("ExecutePreparedQuery - Start")
	logs.WithContext(ctx).Info(query)
	datasource.SqlEngine.ExecuteQuery(ctx, query)
	return nil, nil
}

var icebergDataTypeMapping = map[string]string{
	"tinyint":   "SmallInteger",
	"smallint":  "SmallInteger",
	"mediumint": "Integer",
	"int":       "Integer",
	"integer":   "Integer",
	"bigint":    "BigInteger",
}
var icebergErutoDBDataTypeMapping = map[string]string{
	"SmallInteger": "tinyint",
	"Integer":      "int",
	"BigInteger":   "bigint",
}
