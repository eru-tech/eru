package eru_studio

import (
	"context"
	"sync"
)

// A notice is something the client should know about the request it made, when
// the edit itself still stands. It rides back on the envelope's warnings, which
// is where a client already looks for "this worked, but read this".
//
// It exists so a disagreement about bookkeeping - a revision that no longer
// describes the page, a claim the client cannot back up - does not have to be
// either silent or fatal. Failing the run throws away an edit the user asked
// for over a hash; saying nothing lets the client keep drifting.
type Notices struct {
	mu    sync.Mutex
	items []string
}

type noticesKey struct{}

func WithNotices(ctx context.Context, notices *Notices) context.Context {
	return context.WithValue(ctx, noticesKey{}, notices)
}

// NoticesFrom always returns a usable collector, so a caller never has to guard
// against a context that was not set up.
func NoticesFrom(ctx context.Context) *Notices {
	if notices, ok := ctx.Value(noticesKey{}).(*Notices); ok && notices != nil {
		return notices
	}
	return &Notices{}
}

func NewNotices() *Notices {
	return &Notices{}
}

func (n *Notices) Add(text string) {
	if n == nil || text == "" {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.items = append(n.items, text)
}

func (n *Notices) All() []string {
	if n == nil {
		return nil
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.items...)
}
