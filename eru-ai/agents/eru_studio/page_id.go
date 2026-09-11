package eru_studio

import (
	"context"
	"strings"
)

// PageIdParam lets the client name the page this edit belongs to when it is not
// sending the page itself. A new page the user has not saved yet already has an
// id in the client; without a way to send it, the agent has to invent one, and
// what it invents becomes the page's permanent identity.
const PageIdParam = "page_id"

const pageIdentityKey contextKey = "eru_studio_page_identity"

// PageIdentity is the id the answer's root page must carry.
//
// Fixed means the id came from the request - the page in `code`, or the
// page_id param - so it is the page's existing identity and the answer must
// keep it. When it is not fixed there is no page yet, nobody has told us what
// to call it, and the id is only a fallback for an answer that names none.
type PageIdentity struct {
	Id    string
	Fixed bool
}

// WithPageIdentity records the page id for this request.
func WithPageIdentity(ctx context.Context, identity PageIdentity) context.Context {
	return context.WithValue(ctx, pageIdentityKey, identity)
}

// PageIdentityFrom returns the page id for this request, zero-valued outside one.
func PageIdentityFrom(ctx context.Context) PageIdentity {
	if ctx == nil {
		return PageIdentity{}
	}
	identity, _ := ctx.Value(pageIdentityKey).(PageIdentity)
	return identity
}

// Stamp puts the request's id on the page and reports what it replaced.
//
// A fixed id is written unconditionally: the page's identity belongs to the
// client that owns the page, not to the model, and a renamed id makes the
// client save a second page instead of updating the one the user is editing. An
// id that is not fixed is only filled in where the answer left one out, so a
// name the user asked for in the prompt survives.
func (identity PageIdentity) Stamp(page map[string]interface{}) (replaced string, changed bool) {
	if page == nil || identity.Id == "" {
		return "", false
	}
	current, _ := page["id"].(string)
	if strings.TrimSpace(current) != "" {
		if !identity.Fixed || current == identity.Id {
			return current, false
		}
	}
	page["id"] = identity.Id
	return current, true
}

// OptionEnableMultiPage is the reserved clarification option value the agent
// offers when a single-page request needs a page of its own - a repeated row, a
// side panel, a popup, a board card.
//
// It is the one clarification option the agent cannot honour itself: the output
// mode is fixed for the run, so the answer has to reach the CLIENT, which
// re-issues the same prompt with output_mode "auto". A client that leaves this
// unhandled leaves the user picking between compromises with no way to ask for
// the real thing - which is what happened when the only options offered were
// fixed slots, one entry, or a note saying nested pages would be needed.
const OptionEnableMultiPage = "enable_multi_page"
