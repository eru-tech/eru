// Package catalog is the eru-ai side of the eru-studio component library: the
// generated description of every component type, its properties, its allowed
// property values, the events it emits and the page/component interfaces the
// Angular and Flutter renderers accept.
//
// component_catalog.json is produced by eru-studio's
// scripts/export-agent-catalog.mjs from the library sources and refreshed by
// sync.sh, so the agent's idea of the component library cannot drift from the
// renderer's. Nothing here may import the agents or tools packages - this stays
// a leaf package so both can depend on it.
package catalog

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

//go:embed component_catalog.json
var catalogFS embed.FS

const CatalogFile = "component_catalog.json"

type Option struct {
	Label string      `json:"label"`
	Value interface{} `json:"value"`
}

// Property is one entry of a component's property schema - the same declaration
// the eru-studio property panel renders.
type Property struct {
	Key         string      `json:"key"`
	Label       string      `json:"label,omitempty"`
	Type        string      `json:"type,omitempty"`
	Category    string      `json:"category,omitempty"`
	Section     string      `json:"section,omitempty"`
	Default     interface{} `json:"default_value,omitempty"`
	Description string      `json:"description,omitempty"`
	Responsive  bool        `json:"responsive,omitempty"`
	Visible     *bool       `json:"visible,omitempty"`
	Options     []Option    `json:"options,omitempty"`
	OptionType  string      `json:"option_type,omitempty"`
	Min         *float64    `json:"min,omitempty"`
	Max         *float64    `json:"max,omitempty"`
	Step        *float64    `json:"step,omitempty"`
}

// EnumValues returns the allowed values when the property is a fixed choice, and
// nil when any value is acceptable. Only static option lists constrain a value:
// an entity- or api-sourced list is resolved at runtime.
func (p Property) EnumValues() []string {
	if len(p.Options) == 0 || (p.OptionType != "" && p.OptionType != "static") {
		return nil
	}
	values := make([]string, 0, len(p.Options))
	for _, option := range p.Options {
		switch v := option.Value.(type) {
		case string:
			values = append(values, v)
		default:
			// A non-string option value (a number, an object) is not something
			// the agent can be held to a literal match on.
			return nil
		}
	}
	return values
}

type Component struct {
	Name                     string     `json:"name"`
	Category                 string     `json:"category"`
	AllowChildren            bool       `json:"allow_children"`
	Class                    string     `json:"class,omitempty"`
	Icon                     string     `json:"icon,omitempty"`
	RegistryOnly             bool       `json:"registry_only,omitempty"`
	Deprecated               bool       `json:"deprecated,omitempty"`
	AliasOf                  string     `json:"alias_of,omitempty"`
	Extraction               string     `json:"extraction,omitempty"`
	Properties               []Property `json:"properties,omitempty"`
	CommonPropertyOverrides  []Property `json:"common_property_overrides,omitempty"`
	ExcludedCommonProperties []string   `json:"excluded_common_properties,omitempty"`
	Styles                   []Property `json:"styles,omitempty"`
	ExcludedCommonStyles     []string   `json:"excluded_common_styles,omitempty"`
	Events                   []string   `json:"events,omitempty"`
}

// InterfaceField is one field of an exported renderer interface (EruPage,
// EruComponent, ComponentEventSubscription, ...).
type InterfaceField struct {
	Name        string        `json:"name"`
	Optional    bool          `json:"optional"`
	Type        string        `json:"type"`
	Values      []interface{} `json:"values,omitempty"`
	Description string        `json:"description,omitempty"`
}

type Source struct {
	Repo        string `json:"repo"`
	LibPath     string `json:"lib_path"`
	Fingerprint string `json:"fingerprint"`
}

type Catalog struct {
	CatalogVersion   string     `json:"catalog_version"`
	Source           Source     `json:"source"`
	CommonProperties []Property `json:"common_properties"`
	CommonStyles     []Property `json:"common_styles"`
	CommonEvents     []string   `json:"common_events"`
	ValueChangeEvent struct {
		Event       string `json:"event"`
		AppliesWhen string `json:"applies_when"`
	} `json:"value_change_event"`
	TypeAliases map[string][]string         `json:"type_aliases"`
	Interfaces  map[string][]InterfaceField `json:"interfaces"`
	Components  map[string]Component        `json:"components"`

	types      []string
	properties map[string]map[string]Property
	styles     map[string]map[string]Property
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
		panic(err)
	}
	return c
}

func Load() (*Catalog, error) {
	loadOnce.Do(func() {
		raw, err := catalogFS.ReadFile(CatalogFile)
		if err != nil {
			loadErr = fmt.Errorf("reading embedded %s: %w", CatalogFile, err)
			return
		}
		loaded, loadErr = Parse(raw)
	})
	return loaded, loadErr
}

func Parse(raw []byte) (*Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parsing component catalog: %w", err)
	}
	if len(c.Components) == 0 {
		return nil, fmt.Errorf("component catalog has no components")
	}
	c.index()
	return &c, nil
}

func (c *Catalog) index() {
	c.types = make([]string, 0, len(c.Components))
	c.properties = make(map[string]map[string]Property, len(c.Components))
	c.styles = make(map[string]map[string]Property, len(c.Components))
	for name, component := range c.Components {
		if !component.Deprecated {
			c.types = append(c.types, name)
		}
		props := make(map[string]Property)
		for _, p := range c.EffectiveProperties(name) {
			props[p.Key] = p
		}
		c.properties[name] = props
		styles := make(map[string]Property)
		for _, s := range c.EffectiveStyles(name) {
			styles[s.Key] = s
		}
		c.styles[name] = styles
	}
	sort.Strings(c.types)
}

// ComponentTypes lists every type the agent may emit, deprecated aliases
// excluded.
func (c *Catalog) ComponentTypes() []string {
	out := make([]string, len(c.types))
	copy(out, c.types)
	return out
}

// ComponentTypesAny lists every type the renderer can load, deprecated aliases
// included - what validation of an existing page must accept.
func (c *Catalog) ComponentTypesAny() []string {
	out := make([]string, 0, len(c.Components))
	for name := range c.Components {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (c *Catalog) Component(componentType string) (Component, bool) {
	component, ok := c.Components[componentType]
	return component, ok
}

// pageHostProperties maps a component type to the properties through which it
// mounts another page. A component that mounts a page is not a container, but a
// client renders the mounted page in place, so it legitimately arrives with that
// page's components under "children" - and validation has to allow that rather
// than call it children on a leaf.
//
// The property names are checked against the exported schemas by a test, so a
// rename in the library cannot leave this behind.
var pageHostProperties = map[string][]string{
	"page_ref":   {"page"},
	"grid":       {"card_page_id"},
	"tile":       {"card_page_id"},
	"nav_outlet": {"default_page"},
}

// PageHostProperties returns the page-mounting properties of a component type.
func (c *Catalog) PageHostProperties(componentType string) []string {
	return pageHostProperties[componentType]
}

// MayHoldNestedPage reports whether a component type mounts another page, and so
// may carry that page's components inline.
func (c *Catalog) MayHoldNestedPage(componentType string) bool {
	return len(pageHostProperties[componentType]) > 0
}

// PageHostTypes lists the component types that mount another page.
func (c *Catalog) PageHostTypes() []string {
	out := make([]string, 0, len(pageHostProperties))
	for componentType := range pageHostProperties {
		out = append(out, componentType)
	}
	sort.Strings(out)
	return out
}

func (c *Catalog) IsContainer(componentType string) bool {
	component, ok := c.Components[componentType]
	return ok && component.AllowChildren
}

// Containers lists the types that may carry children.
func (c *Catalog) Containers() []string {
	out := []string{}
	for _, name := range c.types {
		if c.Components[name].AllowChildren {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func excluded(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}

// EffectiveProperties is the full property set of a component: the common
// properties it did not exclude or redefine, its overrides, then its own.
func (c *Catalog) EffectiveProperties(componentType string) []Property {
	component, ok := c.Components[componentType]
	if !ok {
		return nil
	}
	overridden := map[string]bool{}
	for _, p := range component.CommonPropertyOverrides {
		overridden[p.Key] = true
	}
	out := make([]Property, 0, len(c.CommonProperties)+len(component.Properties))
	for _, p := range c.CommonProperties {
		if excluded(component.ExcludedCommonProperties, p.Key) || overridden[p.Key] {
			continue
		}
		out = append(out, p)
	}
	out = append(out, component.CommonPropertyOverrides...)
	out = append(out, component.Properties...)
	return out
}

func (c *Catalog) EffectiveStyles(componentType string) []Property {
	component, ok := c.Components[componentType]
	if !ok {
		return nil
	}
	out := make([]Property, 0, len(c.CommonStyles)+len(component.Styles))
	for _, s := range c.CommonStyles {
		if excluded(component.ExcludedCommonStyles, s.Key) {
			continue
		}
		out = append(out, s)
	}
	return append(out, component.Styles...)
}

func (c *Catalog) Property(componentType, key string) (Property, bool) {
	props, ok := c.properties[componentType]
	if !ok {
		return Property{}, false
	}
	p, ok := props[key]
	return p, ok
}

func (c *Catalog) StyleProperty(componentType, key string) (Property, bool) {
	styles, ok := c.styles[componentType]
	if !ok {
		return Property{}, false
	}
	s, ok := styles[key]
	return s, ok
}

// Events lists every event a component can emit: its own, plus the DOM events
// every component carries, plus valueChange when the component is field-bound.
func (c *Catalog) Events(componentType string) []string {
	component, ok := c.Components[componentType]
	if !ok {
		return nil
	}
	out := append([]string{}, component.Events...)
	out = append(out, c.CommonEvents...)
	if c.ValueChangeEvent.Event != "" {
		out = append(out, c.ValueChangeEvent.Event)
	}
	sort.Strings(out)
	return out
}

// InterfaceEnum returns the literal union declared for a renderer interface
// field, e.g. InterfaceEnum("ComponentEventSubscription", "action").
func (c *Catalog) InterfaceEnum(interfaceName, fieldName string) []interface{} {
	for _, field := range c.Interfaces[interfaceName] {
		if field.Name == fieldName {
			return field.Values
		}
	}
	return nil
}

func (c *Catalog) InterfaceEnumStrings(interfaceName, fieldName string) []string {
	values := c.InterfaceEnum(interfaceName, fieldName)
	out := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// InterfaceFields returns the declared fields of a renderer interface.
func (c *Catalog) InterfaceFields(interfaceName string) []InterfaceField {
	return c.Interfaces[interfaceName]
}

func (c *Catalog) InterfaceFieldNames(interfaceName string) []string {
	fields := c.Interfaces[interfaceName]
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, f.Name)
	}
	return out
}

// EventActions are the runtime actions a component event subscription may run.
func (c *Catalog) EventActions() []string {
	return c.InterfaceEnumStrings("ComponentEventSubscription", "action")
}

// TypeAlias returns the literal union declared for a renderer type alias, e.g.
// TypeAlias("TailwindBreakpoint").
func (c *Catalog) TypeAlias(name string) []string {
	return c.TypeAliases[name]
}

// Breakpoints are the keys a responsive property or style map may use.
func (c *Catalog) Breakpoints() []string {
	if bps := c.TypeAliases["TailwindBreakpoint"]; len(bps) > 0 {
		return bps
	}
	return []string{"base", "sm", "md", "lg", "xl", "2xl"}
}

func (c *Catalog) Fingerprint() string { return c.Source.Fingerprint }

// ResolveType maps a deprecated alias onto the type the agent should use.
func (c *Catalog) ResolveType(componentType string) string {
	if component, ok := c.Components[componentType]; ok && component.AliasOf != "" {
		return component.AliasOf
	}
	return componentType
}

// SuggestType offers the closest known type for an unknown one, so a validation
// error can tell the model what to use instead.
func (c *Catalog) SuggestType(unknown string) string {
	needle := normalizeTypeName(unknown)
	if needle == "" {
		return ""
	}
	best, bestScore := "", 0
	for _, candidate := range c.types {
		score := similarity(needle, normalizeTypeName(candidate))
		if score > bestScore {
			best, bestScore = candidate, score
		}
	}
	if bestScore < len(needle)/2 {
		return ""
	}
	return best
}

func normalizeTypeName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// similarity is the length of the longest common prefix plus a bonus for a
// containment match - enough to turn "textfield" into "textbox" and "dropdown"
// into nothing.
func similarity(a, b string) int {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		i += len(b) / 2
	}
	return i
}
