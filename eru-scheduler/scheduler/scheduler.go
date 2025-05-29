package scheduler

import (
	"context"
	"encoding/json"
	"errors"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type Scheduler struct {
	SchedulerType string `json:"scheduler_type" eru:"required"`
	SchedulerName string `json:"scheduler_name" eru:"required"`
}

type SchedulerI interface {
	Schedule(ctx context.Context, scheduleJobName string, scheduleCommand string, scheduleCron string) (err error)
	MakeFromJson(ctx context.Context, rj *json.RawMessage) error
	Init(ctx context.Context, rj *json.RawMessage) (err error)
}

func GetScheduler(schedulerType string) SchedulerI {
	switch schedulerType {
	case "pg_cron":
		return new(PgCronScheduler)
	default:
		return nil
	}
}

func (scheduler *Scheduler) Schedule(ctx context.Context,scheduleCommand string, scheduleCron string) (err error) {
	err = errors.New("method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}
func (scheduler *Scheduler) Init(ctx context.Context, rj *json.RawMessage) (err error) {
	err = errors.New("method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (scheduler *Scheduler) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &scheduler)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
