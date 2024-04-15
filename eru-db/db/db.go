package db

import (
	"context"
	"github.com/jmoiron/sqlx"
)

type DbI interface {
	GetConn() *sqlx.DB
	SetConn(*sqlx.DB)
	GetDbQuery(ctx context.Context, query string) (finalQuery string)
}

func GetDb(dbType string) DbI {
	switch dbType {
	case "POSTGRES":
		return new(DbPostgres)
	case "MYSQL":
		return new(DbMysql)
	default:
		return new(Db)
	}
}

func (db *Db) GetDbQuery(ctx context.Context, query string) (finalQuery string) {
	return query
}

type Db struct {
	Con       *sqlx.DB `json:"-"`
	ConStatus bool     `json:"-"`
}

func (db *Db) GetConn() *sqlx.DB {
	return db.Con
}

func (db *Db) SetConn(con *sqlx.DB) {
	db.Con = con
}
