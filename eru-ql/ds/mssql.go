package ds

import (
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"log"
	"strings"
)

type MssqlSqlMaker struct {
	SqlMaker
}

func (pr *MssqlSqlMaker) GetTableMetaDataSQL() string {
	return mssqlTableMetaDataSQL
}
func (mr *MssqlSqlMaker) CreateConn(dataSource *module_model.DataSource) error {
	return errors.New("CreateConn not implemented")
}
func (mr *MssqlSqlMaker) CheckMe() {
	log.Print("I am MysqlRead changed  removed")
	mr.ChildChange = "changed by MysqlRead"
	//log.Print(mr)
}

func (mr *MssqlSqlMaker) AddLimitSkipClause(query string, limit int, skip int, globalLimit int) (newQuery string) {
	log.Print(limit)
	if limit == 0 {
		limit = globalLimit
	}
	log.Print(limit)
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

func (mr *MssqlSqlMaker) getDataTypeMapping(dataType string) string {
	if mssqlDataTypeMapping[dataType] == "" {
		return "NotSupported"
	} else {
		return mssqlDataTypeMapping[dataType]
	}
}

var mssqlTableMetaDataSQL = ""
var mssqlDataTypeMapping = map[string]string{}
