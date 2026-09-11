package utiltiy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
)

// A user says "make this look like the invoice page", or "copy the header from
// the one I built yesterday". The agent needs two things it cannot get from the
// request: the list of pages that exist, and the JSON of one of them.
//
// These wrap the processo page actions so the model asks for a page by name
// instead of by an id nobody told it, and so the org/process ids stay out of the
// model's hands.

const (
	ListPagesToolName = "list_pages"
	GetPageToolName   = "get_page"
)

// MaxReferencePageBytes caps one fetched page. A reference page is read for its
// patterns, not copied wholesale, and a 120KB page would crowd out the page
// being built.
const MaxReferencePageBytes = 60000

var pageLibraryActions = []tools.ToolAction{
	{ActionName: ListPagesToolName, Description: "List the pages that exist, with their ids and names"},
	{ActionName: GetPageToolName, Description: "Fetch one existing page's JSON by id"},
}

func ListPagesToolSchema() eru_models.JSONSchema {
	return eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"name_contains": {
				Type:        "string",
				Description: "Optional filter: only pages whose name or id contains this text, matched case-insensitively. Use it when the user named a page (\"the invoice page\") to narrow a long list.",
			},
		},
	}
}

func GetPageToolSchema() eru_models.JSONSchema {
	return eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"page_id": {
				Type:        "string",
				Description: "The id of the page to fetch, as returned by list_pages. Never invent one.",
			},
		},
		Required: []string{"page_id"},
	}
}

func ListPagesToolDescription() string {
	return "List the pages that already exist in this workspace, with their ids and names. " +
		"Call it when the user refers to another page - \"like the invoice page\", \"the one I built yesterday\" - " +
		"and did not give you its id, so you can find the id before fetching it."
}

func GetPageToolDescription() string {
	return "Fetch an existing page's JSON by id, to read how something was built there: a layout, a header, " +
		"an event wiring, a grid configuration. Use it as a reference to imitate - it is NOT the page you are " +
		"editing, and you must never return it as your answer."
}

// PageLibraryTool answers "what pages exist" and "what does that page look
// like" by delegating to the configured processo tool.
type PageLibraryTool struct {
	tools.Tool
	// ListDelegate and GetDelegate are the configured tool actions that do the
	// work. They are wired per request by the agent that offers this tool.
	ListDelegate tools.Tooling `json:"-"`
	GetDelegate  tools.Tooling `json:"-"`
	// OrgId and ProcessId scope the lookup. The processo page actions take them
	// explicitly, so they are filled from the agent's execution context rather
	// than asked of the model.
	OrgId     string `json:"org_id,omitempty"`
	ProcessId string `json:"process_id,omitempty"`
}

func (plTool *PageLibraryTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(pageLibraryActions))
	for i, action := range pageLibraryActions {
		infos[i] = tools.ActionInfo{Name: action.ActionName, Description: action.Description}
	}
	return infos
}

func (plTool *PageLibraryTool) GetActions() []tools.ToolAction { return pageLibraryActions }

func (plTool *PageLibraryTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("PageLibraryTool MakeFromJson - Start")
	if err := json.Unmarshal(*rj, &plTool); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (plTool *PageLibraryTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug(fmt.Sprint("PageLibraryTool Execute - Start : ", actionName))
	switch actionName {
	case ListPagesToolName:
		return plTool.listPages(ctx, projectId, tenantId, params)
	case GetPageToolName:
		return plTool.getPage(ctx, projectId, tenantId, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
}

func (plTool *PageLibraryTool) scope() map[string]interface{} {
	return map[string]interface{}{
		"org_id":     plTool.OrgId,
		"process_id": plTool.ProcessId,
	}
}

func (plTool *PageLibraryTool) listPages(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	if plTool.ListDelegate == nil {
		return nil, false, fmt.Errorf("%s is not available: no tool in this workspace offers fetch_pages", ListPagesToolName)
	}
	result, _, err := plTool.ListDelegate.Execute(ctx, projectId, tenantId, "fetch_pages", plTool.scope())
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint(ListPagesToolName, " failed: ", err.Error()))
		return nil, false, fmt.Errorf("the page list could not be fetched: %w", err)
	}

	pages := pageSummaries(extractRows(result))
	if filter, _ := params["name_contains"].(string); strings.TrimSpace(filter) != "" {
		pages = filterPages(pages, filter)
		if len(pages) == 0 {
			return map[string]interface{}{
				"pages": []pageSummary{},
				"note":  fmt.Sprintf("no page name or id contains %q - call again without name_contains to see every page", filter),
			}, false, nil
		}
	}
	out := map[string]interface{}{"pages": pages, "count": len(pages)}
	if len(pages) == 0 {
		out["note"] = "this workspace has no other pages to reference"
	}
	return out, false, nil
}

func (plTool *PageLibraryTool) getPage(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	if plTool.GetDelegate == nil {
		return nil, false, fmt.Errorf("%s is not available: no tool in this workspace offers fetch_page", GetPageToolName)
	}
	pageId, _ := params["page_id"].(string)
	pageId = strings.TrimSpace(pageId)
	if pageId == "" {
		return nil, false, errors.New("get_page needs a \"page_id\" - call list_pages first to find it")
	}

	request := plTool.scope()
	request["page_id"] = pageId
	result, _, err := plTool.GetDelegate.Execute(ctx, projectId, tenantId, "fetch_page", request)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint(GetPageToolName, " failed for ", pageId, ": ", err.Error()))
		return nil, false, fmt.Errorf("page %q could not be fetched: %w", pageId, err)
	}

	page := firstPageDefinition(result)
	if page == nil {
		return map[string]interface{}{
			"page_id": pageId,
			"note":    "that page id returned nothing - check the id against list_pages",
		}, false, nil
	}

	encoded, merr := json.Marshal(page)
	if merr == nil && len(encoded) > MaxReferencePageBytes {
		// Read for its patterns, not copied: the structure is what matters, so
		// the detail is dropped rather than the request failing.
		return map[string]interface{}{
			"page_id":   pageId,
			"structure": pageStructure(page),
			"note": fmt.Sprintf("this page is %d bytes, too large to return whole. Its structure is above - "+
				"component ids and types only. Ask for it again only if you need a specific component's properties, "+
				"and say which one.", len(encoded)),
		}, false, nil
	}

	return map[string]interface{}{
		"page_id": pageId,
		"page":    page,
		"note": "This is a REFERENCE page. Imitate what the user asked for from it - a layout, a header, an event " +
			"wiring - and never return it as your answer or reuse its component ids in the page you are building.",
	}, false, nil
}

type pageSummary struct {
	PageId string `json:"page_id"`
	Name   string `json:"name,omitempty"`
	Title  string `json:"title,omitempty"`
	Route  string `json:"route,omitempty"`
}

var pageIdKeys = []string{"page_id", "id", "pageid"}
var pageNameKeys = []string{"page_name", "name"}

// pageSummaries reduces whatever the page list returned to the fields needed to
// pick a page: its id and what it is called.
func pageSummaries(rows []map[string]interface{}) []pageSummary {
	out := make([]pageSummary, 0, len(rows))
	for _, row := range rows {
		summary := pageSummary{
			PageId: firstString(row, pageIdKeys),
			Name:   firstString(row, pageNameKeys),
			Title:  firstString(row, []string{"title"}),
			Route:  firstString(row, []string{"route"}),
		}
		// A page definition may be nested under page_def; read the name from
		// there when the row itself does not carry it.
		if summary.Name == "" || summary.Title == "" {
			if def, ok := row["page_def"].(map[string]interface{}); ok {
				if summary.Name == "" {
					summary.Name = firstString(def, pageNameKeys)
				}
				if summary.Title == "" {
					summary.Title = firstString(def, []string{"title"})
				}
				if summary.PageId == "" {
					summary.PageId = firstString(def, pageIdKeys)
				}
			}
		}
		if summary.PageId == "" && summary.Name == "" {
			continue
		}
		out = append(out, summary)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].PageId < out[j].PageId
	})
	return out
}

func filterPages(pages []pageSummary, filter string) []pageSummary {
	needle := strings.ToLower(strings.TrimSpace(filter))
	out := []pageSummary{}
	for _, page := range pages {
		haystack := strings.ToLower(page.PageId + " " + page.Name + " " + page.Title + " " + page.Route)
		if strings.Contains(haystack, needle) {
			out = append(out, page)
		}
	}
	return out
}

// firstPageDefinition digs the EruPage out of the fetch result, wherever the
// stored query wrapped it.
func firstPageDefinition(result map[string]interface{}) map[string]interface{} {
	for _, row := range extractRows(result) {
		for _, key := range []string{"page_def", "page", "definition", "config"} {
			switch value := row[key].(type) {
			case map[string]interface{}:
				return value
			case string:
				var decoded map[string]interface{}
				if json.Unmarshal([]byte(value), &decoded) == nil && len(decoded) > 0 {
					return decoded
				}
			}
		}
		// The row may itself be the page.
		if _, hasComponents := row["components"]; hasComponents {
			return row
		}
	}
	return nil
}

// pageStructure reduces a page to its component tree - ids and types only -
// which is what "build it like that page" actually needs.
func pageStructure(page map[string]interface{}) map[string]interface{} {
	var reduce func(list []interface{}) []interface{}
	reduce = func(list []interface{}) []interface{} {
		out := make([]interface{}, 0, len(list))
		for _, raw := range list {
			component, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			node := map[string]interface{}{
				"id":   component["id"],
				"type": component["type"],
			}
			if children, ok := component["children"].([]interface{}); ok && len(children) > 0 {
				node["children"] = reduce(children)
			}
			out = append(out, node)
		}
		return out
	}
	components, _ := page["components"].([]interface{})
	return map[string]interface{}{
		"id":         page["id"],
		"name":       page["name"],
		"title":      page["title"],
		"components": reduce(components),
	}
}

func (plTool *PageLibraryTool) GetSpec() tools.Tooling { return plTool }

func (plTool *PageLibraryTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &PageLibraryTool{}
	if err := json.Unmarshal(toolObjJson, newTool); err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return newTool, nil
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		Public:       false,
		ToolType:     "PageLibrary",
		Category:     "Utility",
		Description:  "Existing pages of the workspace: list them, and read one as a reference",
		Actions:      []tools.ActionInfo{{Name: ListPagesToolName}, {Name: GetPageToolName}},
		OAuthEnabled: false,
		Icon:         "",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(PageLibraryTool{}), []string{}),
	})
}
