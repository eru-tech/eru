package eru_studio

import (
	"context"
	"strings"
)

type contextKey string

const (
	outputModeKey contextKey = "eru_studio_output_mode"
	basePageKey   contextKey = "eru_studio_base_page"
	pageScopeKey  contextKey = "eru_studio_page_scope"
)

// PageScope is the org and process whose existing pages the agent may read.
type PageScope struct {
	OrgId     string
	ProcessId string
}

// WithPageScope records which org and process the page lookups run against.
func WithPageScope(ctx context.Context, scope PageScope) context.Context {
	return context.WithValue(ctx, pageScopeKey, scope)
}

// PageScopeFrom returns the scope for this request, zero-valued outside one.
func PageScopeFrom(ctx context.Context) PageScope {
	if ctx == nil {
		return PageScope{}
	}
	scope, _ := ctx.Value(pageScopeKey).(PageScope)
	return scope
}

// OutputModeParam is the request param a consumer sets to opt into the patch
// protocol. Without it the agent answers exactly as it always has - a bare
// EruPage - so no existing client has to change.
const OutputModeParam = "output_mode"

// InlineNestedParam controls whether the root page comes back with each nested
// page's components inlined under the component that mounts it, so a client can
// render the whole thing without resolving mounts. On by default; the nested
// pages travel separately either way, and those are what get saved.
const InlineNestedParam = "inline_nested_pages"

const inlineNestedKey contextKey = "eru_studio_inline_nested"

// WithInlineNested records the inlining choice for this request.
func WithInlineNested(ctx context.Context, inline bool) context.Context {
	return context.WithValue(ctx, inlineNestedKey, inline)
}

// InlineNestedEnabled reports whether the root page should carry its nested
// pages inline. Defaults to true: it is what the client already sends, and it
// saves a render pass.
func InlineNestedEnabled(ctx context.Context) bool {
	if ctx == nil {
		return true
	}
	if inline, ok := ctx.Value(inlineNestedKey).(bool); ok {
		return inline
	}
	return true
}

// ParseInlineNested reads the param, defaulting to true.
func ParseInlineNested(raw interface{}) bool {
	switch value := raw.(type) {
	case bool:
		return value
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "false", "0", "no":
			return false
		}
	}
	return true
}

// BaseRevisionParam is the revision of the page the client believes it is
// sending. Optional, and worth setting: it turns "the client sent the wrong
// page" from a silent revert into an error.
const BaseRevisionParam = "base_revision"

// WithOutputMode carries the negotiated output mode down to GetOutputSchema and
// ValidateOutput, which the agent interface hands only a context.
func WithOutputMode(ctx context.Context, mode string) context.Context {
	return context.WithValue(ctx, outputModeKey, mode)
}

// OutputModeFrom reports the mode negotiated for this request, defaulting to the
// full page.
func OutputModeFrom(ctx context.Context) string {
	if ctx == nil {
		return ModeFull
	}
	if mode, ok := ctx.Value(outputModeKey).(string); ok && mode != "" {
		return mode
	}
	return ModeFull
}

// WithBasePage carries the page the client already holds, so validation can
// apply a patch to it and check the page that results - not just the patch in
// isolation.
func WithBasePage(ctx context.Context, page map[string]interface{}) context.Context {
	return context.WithValue(ctx, basePageKey, page)
}

// BasePageFrom returns the page a patch applies to, or nil when the agent is
// authoring one from scratch.
func BasePageFrom(ctx context.Context) map[string]interface{} {
	if ctx == nil {
		return nil
	}
	page, _ := ctx.Value(basePageKey).(map[string]interface{})
	return page
}

// NegotiateMode turns what the consumer asked for into what the agent will do.
//
// "auto" is the useful setting for a studio client: author a page in full, then
// patch every edit after it. An unrecognised value falls back to a full page,
// because answering in a protocol the caller did not ask for is worse than
// ignoring the request.
func NegotiateMode(requested string, hasBasePage bool) string {
	switch strings.ToLower(strings.TrimSpace(requested)) {
	case ModePatch:
		return ModePatch
	case ModeAuto:
		// Deliberately NOT collapsed to ModeFull when there is no page yet.
		//
		// Whether the answer is a patch or a whole page, and whether it may carry
		// more than one page, are independent questions. Collapsing "auto" with no
		// base page to the bare-page protocol answered the second question with
		// the first, and a request to author a page from scratch - exactly when a
		// row template is most likely to be needed - lost the ability to return
		// one. The agent then correctly reported that it could not build the
		// repeatable section it had correctly decided it needed.
		return ModeAuto
	default:
		return ModeFull
	}
}

// EnvelopeEnabled reports whether the response is a page-update envelope rather
// than a bare EruPage. Only the envelope can carry a patch, a nested page, or a
// scope report, so this is a question about what the client can read - never
// about what this particular turn happens to produce.
func EnvelopeEnabled(ctx context.Context) bool {
	switch OutputModeFrom(ctx) {
	case ModePatch, ModeAuto:
		return true
	default:
		return false
	}
}

// PatchExpected reports whether the agent was asked to patch rather than left to
// choose. In auto mode the model decides per turn: a page it is authoring comes
// back whole, an edit to one it was given comes back as a patch.
func PatchExpected(ctx context.Context) bool {
	return OutputModeFrom(ctx) == ModePatch
}
