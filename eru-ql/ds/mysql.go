package ds

import (
	"context"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/module_model"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type MysqlSqlMaker struct {
	SqlMaker
}

func (mr *MysqlSqlMaker) GetTableMetaDataSQL(ctx context.Context) string {
	return mysqlTableMetaDataSQL
}

func (mr *MysqlSqlMaker) CreateConn(ctx context.Context, dataSource *module_model.DataSource) error {
	logs.WithContext(ctx).Debug("CreateConn - Start")
	connString := fmt.Sprint(dataSource.DbConfig.User, ":", dataSource.DbConfig.Password, "@tcp(", dataSource.DbConfig.Host, ":", dataSource.DbConfig.Port, ")/", dataSource.DbConfig.DefaultSchema)
	db, err := sqlx.Open("mysql", connString)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	dataSource.Con = db
	return nil
}
func (mr *MysqlSqlMaker) CheckMe(ctx context.Context) {
	logs.WithContext(ctx).Info("I am MysqlSqlMaker changed  removed")
	mr.ChildChange = "changed by MysqlSqlMaker"
}

func (pr *MysqlSqlMaker) AddLimitSkipClause(ctx context.Context, query string, limit int, skip int, globalLimit int) (newQuery string) {
	logs.WithContext(ctx).Debug("AddLimitSkipClause - Start")
	strSkip := ""
	if skip > 0 {
		strSkip = fmt.Sprint(skip, " , ")
	}
	if limit > 0 {
		newQuery = fmt.Sprint(query, " limit ", strSkip, limit)
	} else {
		newQuery = fmt.Sprint(query, " limit ", strSkip, globalLimit)
	}
	return newQuery
}

func (mr *MysqlSqlMaker) executeQuery(ctx context.Context, query string, datasource *module_model.DataSource) (res map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("executeQuery - Start")
	//rows, e := datasource.Con.Query(query)
	rows, e := datasource.Con.Queryx(query)
	if e != nil {
		logs.WithContext(ctx).Error(e.Error())
	}
	defer rows.Close()
	mapping := make(map[string]interface{})
	for rows.Next() {
		e = rows.MapScan(mapping)
		if e != nil {
			logs.WithContext(ctx).Error(e.Error())
		}
		colsType, ee := rows.ColumnTypes()
		if ee != nil {
			logs.WithContext(ctx).Error(e.Error())
		}
		_ = colsType
	}
	return nil, nil
}
func (mr *MysqlSqlMaker) getDataTypeMapping(ctx context.Context, dataType string) string {
	logs.WithContext(ctx).Debug("getDataTypeMapping - Start")
	if mysqlDataTypeMapping[dataType] == "" {
		return "NotSupported"
	} else {
		return mysqlDataTypeMapping[dataType]
	}
}

var mysqlTableMetaDataSQL = ""

var mysqlDataTypeMapping = map[string]string{}
