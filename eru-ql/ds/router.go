package ds

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync/atomic"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/jmoiron/sqlx"
)

type RouterHint struct {
	UseWriter bool
}

type useWriterCtxKey struct{}

// WithUseWriter returns a context that forces SELECTs to the main writer.
func WithUseWriter(ctx context.Context, useWriter bool) context.Context {
	if !useWriter {
		return ctx
	}
	return context.WithValue(ctx, useWriterCtxKey{}, true)
}

func hintFromCtx(ctx context.Context) RouterHint {
	if v, ok := ctx.Value(useWriterCtxKey{}).(bool); ok && v {
		return RouterHint{UseWriter: true}
	}
	return RouterHint{}
}

const (
	StrategyRoundRobin = "round_robin"
	StrategyWeighted   = "weighted"
	StrategyRandom     = "random"
	StrategyMainOnly   = "main_only"
)

func WriteCon(ctx context.Context, ds *module_model.DataSource) (*sqlx.DB, error) {
	if ds == nil || ds.Con == nil {
		return nil, errors.New("write connection not available")
	}
	return ds.Con, nil
}

// ReadCon picks a connection for a SELECT per ds.ReadPolicy.
// The returned *ReadDbConfig is non-nil only when a replica was chosen — pass it to
// MarkReadFailed on query error to flip that replica to unhealthy.
func ReadCon(ctx context.Context, ds *module_model.DataSource, hint RouterHint) (*sqlx.DB, *module_model.ReadDbConfig, error) {
	if ds == nil {
		return nil, nil, errors.New("nil datasource")
	}
	if !hint.UseWriter {
		hint = hintFromCtx(ctx)
	}
	policy := ds.ReadPolicy
	if hint.UseWriter || len(ds.ReadDbConfigs) == 0 || policy.Strategy == "" || policy.Strategy == StrategyMainOnly {
		if ds.Con == nil {
			return nil, nil, errors.New("main connection not available")
		}
		return ds.Con, nil, nil
	}

	type candidate struct {
		db      *sqlx.DB
		replica *module_model.ReadDbConfig
		weight  int
	}

	var sm SqlMakerI
	var candidates []candidate
	for i, replica := range ds.ReadDbConfigs {
		if !replica.ConStatus || replica.Con == nil {
			if sm == nil {
				sm = GetSqlMaker(ds.DbName)
			}
			if sm == nil {
				continue
			}
			if rerr := sm.ConnectReadReplica(ctx, ds, i); rerr != nil {
				logs.WithContext(ctx).Debug("read replica reconnect skipped: " + rerr.Error())
				continue
			}
		}
		w := replica.Weight
		if w <= 0 {
			w = 1
		}
		candidates = append(candidates, candidate{db: replica.Con, replica: replica, weight: w})
	}

	if policy.IncludeMainInReads && ds.Con != nil {
		w := policy.MainWeight
		if w <= 0 {
			w = 1
		}
		candidates = append(candidates, candidate{db: ds.Con, replica: nil, weight: w})
	}

	if len(candidates) == 0 {
		if policy.FailoverToMain && ds.Con != nil {
			return ds.Con, nil, nil
		}
		return nil, nil, errors.New("no read connections available")
	}

	var pick candidate
	switch policy.Strategy {
	case StrategyRandom:
		pick = candidates[rand.Intn(len(candidates))]
	case StrategyWeighted:
		total := 0
		for _, c := range candidates {
			total += c.weight
		}
		seq := atomic.AddUint64(&ds.ReadCounter, 1) - 1
		idx := int(seq % uint64(total))
		cum := 0
		pick = candidates[len(candidates)-1]
		for _, c := range candidates {
			cum += c.weight
			if idx < cum {
				pick = c
				break
			}
		}
	default: // StrategyRoundRobin and any unknown value
		seq := atomic.AddUint64(&ds.ReadCounter, 1) - 1
		pick = candidates[int(seq%uint64(len(candidates)))]
	}
	if pick.replica != nil {
		logs.WithContext(ctx).Info(fmt.Sprint("replica ", pick.replica.Name, " used"))
	} else {
		logs.WithContext(ctx).Info("main connection used")
	}
	return pick.db, pick.replica, nil
}

// MarkReadFailed flips a replica to unhealthy after a query/connection error so subsequent
// picks skip it (and trigger a reconnect attempt). No-op when replica is nil (main writer).
func MarkReadFailed(ctx context.Context, replica *module_model.ReadDbConfig) {
	if replica == nil {
		return
	}
	replica.ConStatus = false
	if replica.Con != nil {
		_ = replica.Con.Close()
		replica.Con = nil
	}
}
