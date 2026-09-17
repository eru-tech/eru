package eru_studio

import (
	"context"
	"sort"
	"strings"
	"sync"
)

// Ledger records the lookups an agent actually made while answering one request.
//
// The prompt asks the model to read the entity metadata before it names a form
// field, and for a long time nothing checked whether it did. A page that binds
// to invented field names looks correct and binds to nothing, and the user only
// finds out when they open it. The ledger turns that instruction into something
// the validator can enforce: the page may claim bindings only if the lookup that
// justifies them actually happened.
type Ledger struct {
	mu sync.Mutex
	// called counts successful calls per tool action.
	called map[string]int
	// offered names the actions this run had available at all.
	offered map[string]bool
	// broken names actions that were offered but failed when called. A lookup
	// that cannot run must not become a requirement the model can never satisfy.
	broken map[string]string
	// entityNames are the entity names the metadata lookup returned, and
	// tableToEntity maps each physical table back to its entity. A page binds to
	// the entity; binding to the table renders nothing, and the two look alike
	// enough that being told the difference in prose has not been enough.
	entityNames   map[string]bool
	tableToEntity map[string]string
}

func NewLedger() *Ledger {
	return &Ledger{
		called:        map[string]int{},
		offered:       map[string]bool{},
		broken:        map[string]string{},
		entityNames:   map[string]bool{},
		tableToEntity: map[string]string{},
	}
}

// RecordEntities notes what the metadata lookup actually returned, so a page can
// be checked against it rather than against what the model remembers.
func (l *Ledger) RecordEntities(names []string, tables map[string]string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, name := range names {
		if name != "" {
			l.entityNames[name] = true
		}
	}
	for table, entity := range tables {
		if table != "" && entity != "" {
			l.tableToEntity[table] = entity
		}
	}
}

// KnowsEntities reports whether any entity was ever returned. Without that there
// is nothing to check a page against, and guessing would be worse than silence.
func (l *Ledger) KnowsEntities() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entityNames) > 0
}

// EntityForTable returns the entity a physical table belongs to, if the name
// given is a table rather than an entity.
func (l *Ledger) EntityForTable(name string) (string, bool) {
	if l == nil {
		return "", false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entityNames[name] {
		return "", false
	}
	entity, ok := l.tableToEntity[name]
	return entity, ok
}

// Offer records that an action was available to the model this run.
func (l *Ledger) Offer(action string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.offered[action] = true
}

// Record marks a successful call. Tools call this on their own behalf, so it is
// a no-op on a nil ledger - a tool used outside a studio run has nowhere to
// report to and should not have to know that.
func (l *Ledger) Record(action string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.called[action]++
}

// RecordFailure marks an action as attempted and broken.
func (l *Ledger) RecordFailure(action string, reason string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.broken[action] = reason
}

func (l *Ledger) Called(action string) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.called[action] > 0
}

// Enforceable reports whether it is fair to require this action: it was offered,
// and it has not failed when tried.
func (l *Ledger) Enforceable(action string) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.offered[action] {
		return false
	}
	_, failed := l.broken[action]
	return !failed
}

// Calls is the per-action tally, for the run metrics.
func (l *Ledger) Calls() map[string]int {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]int, len(l.called))
	for action, n := range l.called {
		out[action] = n
	}
	return out
}

type ledgerKey struct{}

func WithLedger(ctx context.Context, ledger *Ledger) context.Context {
	return context.WithValue(ctx, ledgerKey{}, ledger)
}

// LedgerFrom returns the ledger for this run, or nil. Every Ledger method is
// nil-safe, so callers never have to check.
func LedgerFrom(ctx context.Context) *Ledger {
	if ledger, ok := ctx.Value(ledgerKey{}).(*Ledger); ok {
		return ledger
	}
	return nil
}

// BindingClaim is one component saying it binds to a real entity field.
type BindingClaim struct {
	ComponentId string
	Identifier  string
	EntityName  string
}

func (b BindingClaim) key() string { return b.ComponentId + "|" + b.Identifier + "|" + b.EntityName }

// BindingClaims finds every component that claims to bind to real data.
//
// "identifier" is the claim: the prompt tells the model to leave it off when it
// is inventing a field name, so a page that sets it is asserting the field
// exists. "entity_name" is the same assertion at the component or page level.
func BindingClaims(page map[string]interface{}) []BindingClaim {
	claims := []BindingClaim{}
	if len(page) == 0 {
		return claims
	}
	if entity, _ := page["entity_name"].(string); strings.TrimSpace(entity) != "" {
		pageId, _ := page["id"].(string)
		claims = append(claims, BindingClaim{ComponentId: pageId, EntityName: entity})
	}
	walkComponents(page, func(component map[string]interface{}, _ string) {
		id, _ := component["id"].(string)
		identifier := firstStringProperty(component, "identifier")
		entity := firstStringProperty(component, "entity_name")
		if identifier == "" && entity == "" {
			return
		}
		claims = append(claims, BindingClaim{ComponentId: id, Identifier: identifier, EntityName: entity})
	})
	sort.Slice(claims, func(i, j int) bool { return claims[i].key() < claims[j].key() })
	return claims
}

// IntroducedBindings is what this edit newly claims, ignoring what the page
// already claimed. A patch that moves a button must not be held to account for
// bindings someone else wrote last week.
func IntroducedBindings(basePage, resolved map[string]interface{}) []BindingClaim {
	existing := map[string]bool{}
	for _, claim := range BindingClaims(basePage) {
		existing[claim.key()] = true
	}
	out := []BindingClaim{}
	for _, claim := range BindingClaims(resolved) {
		if existing[claim.key()] {
			continue
		}
		out = append(out, claim)
	}
	return out
}

// firstStringProperty reads a property from whichever breakpoint sets it, base
// first: a value the model put under "base" and a value it put under "md" are
// the same claim as far as binding goes.
func firstStringProperty(component map[string]interface{}, key string) string {
	properties, ok := component["properties"].(map[string]interface{})
	if !ok {
		return ""
	}
	if base, ok := properties["base"].(map[string]interface{}); ok {
		if value, _ := base[key].(string); strings.TrimSpace(value) != "" {
			return value
		}
	}
	names := make([]string, 0, len(properties))
	for breakpoint := range properties {
		if breakpoint != "base" {
			names = append(names, breakpoint)
		}
	}
	sort.Strings(names)
	for _, breakpoint := range names {
		bag, ok := properties[breakpoint].(map[string]interface{})
		if !ok {
			continue
		}
		if value, _ := bag[key].(string); strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
