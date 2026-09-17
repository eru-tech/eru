package eru_studio

import (
	"context"
	"sync"
)

// RepairState is a repair turn in flight.
//
// The ordinary retry throws the rejected answer away and asks for the whole page
// again. A repair keeps it: the page the model just produced becomes the base,
// and the next answer is a patch against it that fixes only what was wrong. That
// turns a twenty-five second rewrite of seventy components into a diff touching
// one - and it stops the rewrite from introducing something new, which is how a
// second attempt sometimes ended up worse than the first.
//
// It is mutable state held in the request context because the retry loop that
// triggers it and the resolution that consumes it are on opposite sides of a
// generic agent that must not know either exists.
type RepairState struct {
	mu     sync.Mutex
	active bool
	base   map[string]interface{}
	// nested carries the pages the rejected answer produced, so a repair that
	// only fixes the root page does not silently drop them.
	nested []*NestedPage
}

func NewRepairState() *RepairState { return &RepairState{} }

// Begin makes page the base the next answer is a diff against.
func (r *RepairState) Begin(page map[string]interface{}, nested []*NestedPage) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active = true
	r.base = page
	r.nested = nested
}

func (r *RepairState) Active() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.active
}

func (r *RepairState) Base() map[string]interface{} {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.base
}

// CarriedPages are the nested pages from the rejected answer, to be kept unless
// the repair replaces them.
func (r *RepairState) CarriedPages() []*NestedPage {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.nested
}

type repairKey struct{}

func WithRepairState(ctx context.Context, state *RepairState) context.Context {
	return context.WithValue(ctx, repairKey{}, state)
}

// RepairStateFrom returns the repair state for this run, or nil. Every method is
// nil-safe, so callers never have to check.
func RepairStateFrom(ctx context.Context) *RepairState {
	if state, ok := ctx.Value(repairKey{}).(*RepairState); ok {
		return state
	}
	return nil
}
