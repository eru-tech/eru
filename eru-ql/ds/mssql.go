package ds

import (
	"context"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"strings"
)

type MssqlSqlMaker struct {
	SqlMaker
}

func (pr *MssqlSqlMaker) GetTableMetaDataSQL(ctx context.Context) string {
	return mssqlTableMetaDataSQL
}
func (mr *MssqlSqlMaker) CreateConn(ctx context.Context, dataSource *module_model.DataSource) error {
	return errors.New("CreateConn not implemented")
}
func (mr *MssqlSqlMaker) CheckMe(ctx context.Context) {
	logs.WithContext(ctx).Info("I am MssqlSqlMaker changed  removed")
	mr.ChildChange = "changed by MysqlRead"
}

func (mr *MssqlSqlMaker) AddLimitSkipClause(ctx context.Context, query string, limit int, skip int, globalLimit int) (newQuery string) {
	logs.WithContext(ctx).Debug("AddLimitSkipClause - Start")
	if limit == 0 {
		limit = globalLimit
	}

	if skip == 0 {
		if mr.DistinctResults {
			newQuery = strings.Replace(query, "distinct ", fmt.Sprint("distinct top ", limit, " "), 1)
		} else {
			newQuery = strings.Replace(query, "select ", fmt.Sprint("select top ", limit), 1)
		}

	} else {
		orderClause := ""
		if mr.SortClause == "" {
			orderClause = " order by (select null) "
		}
		newQuery = fmt.Sprint(query, orderClause, " offset ", skip, " rows fetch next ", limit, " rows only")
	}
	return newQuery
}

func (mr *MssqlSqlMaker) getDataTypeMapping(ctx context.Context, dataType string) string {
	logs.WithContext(ctx).Debug("getDataTypeMapping - Start")
	if mssqlDataTypeMapping[dataType] == "" {
		return "NotSupported"
	} else {
		return mssqlDataTypeMapping[dataType]
	}
}

var mssqlTableMetaDataSQL = ""
var mssqlDataTypeMapping = map[string]string{}
