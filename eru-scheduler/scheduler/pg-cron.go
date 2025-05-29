package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/jmoiron/sqlx"
)

type PgCronScheduler struct {
	Scheduler
	DataSource DataSource `json:"data_source" eru:"required"`
}

type DataSource struct {
	DbConfig  DbConfig `json:"db_config" eru:"required"`
	Con       *sqlx.DB `json:"-"`
	ConStatus bool     `json:"con_status"`
}
type DbConfig struct {
	Host          string       `json:"host" eru:"required"`
	Port          string       `json:"port" eru:"required"`
	User          string       `json:"user" eru:"required"`
	Password      string       `json:"password" eru:"required"`
	DefaultDB     string       `json:"default_db" eru:"required"`
	DefaultSchema string       `json:"default_schema" eru:"required"`
	DriverConfig  DriverConfig `json:"driver_config" eru:"required"`
}

type DriverConfig struct {
	MaxOpenConns    int           `json:"max_open_conns" eru:"required"`
	MaxIdleConns    int           `json:"max_idle_conns" eru:"required"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime" eru:"required"`
}

func (pgCronScheduler *PgCronScheduler) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &pgCronScheduler)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (pgCronScheduler *PgCronScheduler) GetConn() *sqlx.DB {
	logs.WithContext(context.Background()).Debug("GetConStatus - Start")
	return pgCronScheduler.DataSource.Con
}

func (pgCronScheduler *PgCronScheduler) SetConn(con *sqlx.DB) {
	logs.WithContext(context.Background()).Debug("SetConnection - Start")
	pgCronScheduler.DataSource.Con = con
	if pgCronScheduler.DataSource.Con != nil {
		pgCronScheduler.DataSource.ConStatus = true
	}
}

func (pgCronScheduler *PgCronScheduler) Init(ctx context.Context, rj *json.RawMessage) (err error) {
	logs.WithContext(ctx).Debug("Init - Start")

	var schClone PgCronScheduler
	err = schClone.MakeFromJson(ctx, rj)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}

	db, err := sqlx.Open("postgres", schClone.getStoreDbPath())
	if err != nil {
		logs.Logger.Error(err.Error())
		pgCronScheduler.DataSource.ConStatus = false
		return err
	}
	_, err = db.Queryx("select 1")
	if err != nil {
		pgCronScheduler.DataSource.ConStatus = false
		logs.Logger.Error(err.Error())
		return err
	}
	logs.Logger.Info("dummy query success - setting con as true")
	pgCronScheduler.DataSource.Con = db
	pgCronScheduler.DataSource.ConStatus = true
	return nil
}

func (pgCronScheduler *PgCronScheduler) getStoreDbPath() string {
	return fmt.Sprint("postgres://", pgCronScheduler.DataSource.DbConfig.User, ":", pgCronScheduler.DataSource.DbConfig.Password, "@", pgCronScheduler.DataSource.DbConfig.Host, ":", pgCronScheduler.DataSource.DbConfig.Port, "/", pgCronScheduler.DataSource.DbConfig.DefaultDB, "?sslmode=disable")
}

func (pgCronScheduler *PgCronScheduler) Schedule(ctx context.Context, scheduleJobName string, scheduleCommand string, scheduleCron string) (err error) {
	logs.WithContext(ctx).Debug("ScheduleCronJob - Start")

	logs.WithContext(ctx).Info(fmt.Sprint("scheduleJobName: ", scheduleJobName))
	logs.WithContext(ctx).Info(fmt.Sprint("scheduleCommand: ", scheduleCommand))
	logs.WithContext(ctx).Info(fmt.Sprint("scheduleCron: ", scheduleCron))

	var insertQueries []*models.Queries
	insertQueryFuncAsync := models.Queries{}
	query := fmt.Sprint("SELECT cron.schedule('", scheduleJobName, "','", scheduleCron, "', $$ ", scheduleCommand, "; $$);")
	logs.WithContext(ctx).Info(fmt.Sprint("query: ", query))
	insertQueryFuncAsync.Query = query
	insertQueryFuncAsync.Rank = 1
	insertQueries = append(insertQueries, &insertQueryFuncAsync)
	_, insertOutputErr := utils.ExecuteDbSave(ctx, pgCronScheduler.GetConn(), insertQueries)
	if insertOutputErr != nil {
		logs.WithContext(ctx).Error(insertOutputErr.Error())
		return insertOutputErr
	}
	return nil
}
