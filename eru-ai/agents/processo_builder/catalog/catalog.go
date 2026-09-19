// Package catalog carries processo's entity and field model as the agent sees
// it: which datatypes exist, which keys each one writes, what a name may look
// like, which names are reserved.
//
// The file it reads is generated from the processo app by
// scripts/export-processo-catalog.mjs and refreshed by sync.sh. Nothing here is
// hand-written, so the prompt the model reads and the checks its answer is held
// to come from one source and cannot disagree.
//
// Required-ness is deliberately absent. It lives on the Go param structs in
// eru-ai/tools/eru/processo.go, which are what actually rejects a bad payload.
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

//go:embed processo_catalog.json
var catalogJSON []byte

const CatalogFile = "processo_catalog.json"

type Source struct {
	Repo        string   `json:"repo"`
	AppPath     string   `json:"app_path"`
	Files       []string `json:"files"`
	Fingerprint string   `json:"fingerprint"`
}

type Datatype struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// FormControl is one control of the screen that authors this object. It is not
// the payload - it is what a person is offered - and it is here so the agent can
// answer "is that something the UI can even express".
type FormControl struct {
	Key      string      `json:"key"`
	Default  interface{} `json:"default,omitempty"`
	Required bool        `json:"required,omitempty"`
}

type EntitySpec struct {
	NamePattern      string        `json:"name_pattern"`
	FormControls     []FormControl `json:"form_controls"`
	SystemFieldNames []string      `json:"system_field_names"`
}

type FieldSpec struct {
	NamePattern string `json:"name_pattern"`
	// CommonKeys are written for every datatype.
	CommonKeys []string `json:"common_keys"`
	// ConditionalKeys are written on something other than the datatype - a
	// derived-from-child field, a date's allowed days, the dropdown source.
	ConditionalKeys []string `json:"conditional_keys"`
	// PerDatatype is the key set a datatype writes on top of CommonKeys.
	PerDatatype map[string][]string `json:"per_datatype"`
	// PerDatatypeNotOffered are cases the save builder still handles that the
	// editor no longer offers. They must never be emitted.
	PerDatatypeNotOffered []string                     `json:"per_datatype_not_offered"`
	Enums                 map[string]map[string]string `json:"enums"`
	FormControls          []FormControl                `json:"form_controls"`
}

type Catalog struct {
	CatalogVersion string     `json:"catalog_version"`
	Source         Source     `json:"source"`
	Note           string     `json:"note"`
	Entity         EntitySpec `json:"entity"`
	Field          FieldSpec  `json:"field"`
	Datatypes      []Datatype `json:"datatypes"`

	offered      map[string]Datatype
	allowedKeys  map[string]map[string]bool
	systemFields map[string]bool
	notOffered   map[string]bool
	fieldNameRe  *regexp.Regexp
	entityNameRe *regexp.Regexp
}

var (
	loadOnce sync.Once
	loaded   *Catalog
	loadErr  error
)

// Get returns the embedded catalog. It panics only if the embedded file is
// unparseable, which a build can never produce - Load is there for callers that
// want the error.
func Get() *Catalog {
	c, err := Load()
	if err != nil {
		panic(fmt.Sprintf("processo catalog: %v", err))
	}
	return c
}

func Load() (*Catalog, error) {
	loadOnce.Do(func() {
		loaded, loadErr = Parse(catalogJSON)
	})
	return loaded, loadErr
}

func Parse(raw []byte) (*Catalog, error) {
	c := &Catalog{}
	if err := json.Unmarshal(raw, c); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", CatalogFile, err)
	}
	if len(c.Datatypes) == 0 {
		return nil, fmt.Errorf("%s lists no datatypes", CatalogFile)
	}
	c.index()
	return c, nil
}

// goPattern turns the JavaScript literal the extractor captured (`/^[a-z_]+$/`)
// into something regexp.Compile accepts. A pattern that will not compile is
// dropped rather than fatal: a missing name check is a worse agent, a panic at
// load is a dead service.
func goPattern(js string) *regexp.Regexp {
	trimmed := strings.TrimSpace(js)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "/") {
		if end := strings.LastIndex(trimmed, "/"); end > 0 {
			trimmed = trimmed[1:end]
		}
	}
	re, err := regexp.Compile(trimmed)
	if err != nil {
		return nil
	}
	return re
}

func (c *Catalog) index() {
	c.offered = make(map[string]Datatype, len(c.Datatypes))
	for _, dt := range c.Datatypes {
		c.offered[dt.Value] = dt
	}
	c.systemFields = make(map[string]bool, len(c.Entity.SystemFieldNames))
	for _, name := range c.Entity.SystemFieldNames {
		c.systemFields[name] = true
	}
	c.notOffered = make(map[string]bool, len(c.Field.PerDatatypeNotOffered))
	for _, name := range c.Field.PerDatatypeNotOffered {
		c.notOffered[name] = true
	}
	c.allowedKeys = make(map[string]map[string]bool, len(c.offered))
	for name := range c.offered {
		allowed := make(map[string]bool, len(c.Field.CommonKeys)+len(c.Field.ConditionalKeys)+8)
		for _, key := range c.Field.CommonKeys {
			allowed[key] = true
		}
		for _, key := range c.Field.ConditionalKeys {
			allowed[key] = true
		}
		for _, key := range c.Field.PerDatatype[name] {
			allowed[key] = true
		}
		c.allowedKeys[name] = allowed
	}
	c.fieldNameRe = goPattern(c.Field.NamePattern)
	c.entityNameRe = goPattern(c.Entity.NamePattern)
}

func (c *Catalog) Fingerprint() string { return c.Source.Fingerprint }

// DatatypeNames are the datatypes the editor offers, and the only ones an agent
// may emit.
func (c *Catalog) DatatypeNames() []string {
	names := make([]string, 0, len(c.Datatypes))
	for _, dt := range c.Datatypes {
		names = append(names, dt.Value)
	}
	sort.Strings(names)
	return names
}

func (c *Catalog) IsDatatype(name string) bool {
	_, ok := c.offered[name]
	return ok
}

// IsRetiredDatatype reports a case the save builder still handles but the editor
// no longer offers - worth a distinct message, because "unknown datatype" would
// send the model looking for a typo.
func (c *Catalog) IsRetiredDatatype(name string) bool { return c.notOffered[name] }

func (c *Catalog) Label(datatype string) string {
	if dt, ok := c.offered[datatype]; ok {
		return dt.Label
	}
	return ""
}

// KeysFor is the datatype's own keys, on top of the common set.
func (c *Catalog) KeysFor(datatype string) []string {
	keys := append([]string(nil), c.Field.PerDatatype[datatype]...)
	sort.Strings(keys)
	return keys
}

// AllowsKey reports whether a key has any business on a field of this datatype.
func (c *Catalog) AllowsKey(datatype, key string) bool {
	allowed, ok := c.allowedKeys[datatype]
	if !ok {
		return false
	}
	return allowed[key]
}

func (c *Catalog) IsSystemFieldName(name string) bool { return c.systemFields[name] }

func (c *Catalog) SystemFieldNames() []string {
	return append([]string(nil), c.Entity.SystemFieldNames...)
}

func (c *Catalog) ValidFieldName(name string) bool {
	if c.fieldNameRe == nil {
		return true
	}
	return c.fieldNameRe.MatchString(name)
}

func (c *Catalog) ValidEntityName(name string) bool {
	if c.entityNameRe == nil {
		return true
	}
	return c.entityNameRe.MatchString(name)
}

func (c *Catalog) FieldNamePattern() string  { return c.Field.NamePattern }
func (c *Catalog) EntityNamePattern() string { return c.Entity.NamePattern }

// OptionTypes are the values option_type accepts, taken from the translation map
// the editor applies. The keys of that map are the editor's own labels; the
// values are what the backend stores, and those are what an agent must send.
func (c *Catalog) OptionTypes() []string {
	seen := map[string]bool{}
	for _, m := range c.Field.Enums {
		for _, v := range m {
			seen[v] = true
		}
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// SuggestDatatype offers the nearest offered datatype to an unknown one, so a
// rejection can say "did you mean" instead of only "no".
func (c *Catalog) SuggestDatatype(unknown string) string {
	best, bestScore := "", 0
	target := strings.ToLower(strings.TrimSpace(unknown))
	for name := range c.offered {
		score := similarity(target, strings.ToLower(name))
		if score > bestScore {
			best, bestScore = name, score
		}
	}
	if bestScore < len(target)/2 {
		return ""
	}
	return best
}

func similarity(a, b string) int {
	if a == b {
		return len(a) * 2
	}
	if strings.Contains(b, a) || strings.Contains(a, b) {
		return len(a)
	}
	shared := 0
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			break
		}
		shared++
	}
	return shared
}
