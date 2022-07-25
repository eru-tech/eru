package ds

import (
	"fmt"
	"github.com/eru-tech/eru/eru-ql/module_model"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"log"
	"reflect"
)

type MysqlSqlMaker struct {
	SqlMaker
}

func (mr *MysqlSqlMaker) GetTableMetaDataSQL() string {
	return mysqlTableMetaDataSQL
}

func (mr *MysqlSqlMaker) CreateConn(dataSource *module_model.DataSource) error {
	connString := fmt.Sprint(dataSource.DbConfig.User, ":", dataSource.DbConfig.Password, "@tcp(", dataSource.DbConfig.Host, ":", dataSource.DbConfig.Port, ")/", dataSource.DbConfig.DefaultSchema)
	log.Print(connString)
	db, err := sqlx.Open("mysql", connString)
	if err != nil {
		log.Print(err)
		return err
	}
	dataSource.Con = db
	return nil
}
func (mr *MysqlSqlMaker) CheckMe() {
	log.Print("I am MysqlSqlMaker changed  removed")
	mr.ChildChange = "changed by MysqlSqlMaker"
	//log.Print(mr)
}

func (pr *MysqlSqlMaker) AddLimitSkipClause(query string, limit int, skip int, globalLimit int) (newQuery string) {
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

func (mr *MysqlSqlMaker) executeQuery(query string, datasource *module_model.DataSource) (res map[string]interface{}, err error) {
	log.Print("executeQuery of MysqlSqlMaker called")
	//rows, e := datasource.Con.Query(query)
	rows, e := datasource.Con.Queryx(query)
	if e != nil {
		log.Print(e)
	}
	//log.Print(rows.Columns())
	//cols,ee := rows.Columns()
	//log.Print(ee)
	defer rows.Close()
	mapping := make(map[string]interface{})
	for rows.Next() {
		e = rows.MapScan(mapping)
		if e != nil {
			log.Print(e)
		}
		colsType, ee := rows.ColumnTypes()
		if ee != nil {
			log.Print(e)
		}
		for _, colType := range colsType {
			log.Print(colType)
			log.Print(colType.Name())
			log.Print(colType.DatabaseTypeName())
			log.Print(mapping[colType.Name()])
			log.Print(reflect.TypeOf(mapping[colType.Name()]))
		}
	}
	return nil, nil
}
func (mr *MysqlSqlMaker) getDataTypeMapping(dataType string) string {
	if mysqlDataTypeMapping[dataType] == "" {
		return "NotSupported"
	} else {
		return mysqlDataTypeMapping[dataType]
	}
}

var mysqlTableMetaDataSQL = ""

var mysqlDataTypeMapping = map[string]string{}
