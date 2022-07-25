package ds

func GetSqlMaker(dbName string) SqlMakerI {
	switch dbName {
	case "mysql":
		return new(MysqlSqlMaker)
	case "postgres":
		return new(PostgresSqlMaker)
	case "mssql":
		return new(MssqlSqlMaker)
	default:
		return nil
		//do nothing
	}
	return nil
}

func GetDbType(dbName string) string {
	switch dbName {
	case "postgres", "mysql", "mssql":
		return "sql"
	case "mongo":
		return "mongo"
	default:
		return "unknown"
	}
	return ""
}
