package tools

import "sync"

type ToolCatalogEntry struct {
	ToolType     string   `json:"tool_type"`
	Category     string   `json:"category"`
	Description  string   `json:"description"`
	Actions      []ActionInfo `json:"actions"`
	OAuthEnabled bool     `json:"oauth_enabled"`
	Icon         string   `json:"icon"`
	IconType     string   `json:"icon_type"`
}

var (
	catalogMu      sync.RWMutex
	catalogEntries []ToolCatalogEntry
)

func RegisterToolCatalog(entry ToolCatalogEntry) {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	catalogEntries = append(catalogEntries, entry)
}

func GetToolCatalog() []ToolCatalogEntry {
	catalogMu.RLock()
	defer catalogMu.RUnlock()
	result := make([]ToolCatalogEntry, len(catalogEntries))
	copy(result, catalogEntries)
	return result
}
