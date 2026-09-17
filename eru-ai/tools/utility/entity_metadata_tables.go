package utiltiy

import (
	"sort"
	"strings"
)

// The metadata query does not return rows of entity/field pairs. It returns ONE
// row with one column, "tables", holding a JSONB array shaped like the database
// it describes:
//
//	tables: [
//	  { table_name: "scf_ba", entity_name: "ba", table_label: "Bank Accounts", table_desc: "...",
//	    columns: [
//	      { column_name: "entity_id",   data_type: "string" },
//	      { column_name: "entity_data", data_type: "jsonb",
//	        json_schema: [ { key_name: "an", key_label: "Account Number",
//	                         data_type: "string", key_desc: "...",
//	                         nullable: "true", unique: "true",
//	                         foreign_key: { foreign_table, foreign_column } } ] } ] } ]
//
// The entity's name is entity_name. table_name is the PHYSICAL table, which is
// the process name and the entity name joined ("scf" + "_" + "fn"), and is what
// something generating SQL needs - deriving one from the other is guesswork, so
// both are carried through. Its fields are the json_schema of the entity_data
// column; the two columns themselves (entity_id, entity_data) are storage, not
// fields anyone binds a form to.
//
// This was missed when the tool was written: it looked for flat rows with an
// entity column and a field column, found neither, and reported zero entities
// for a tenant with 61 of them. The agent then had nothing to bind to and named
// fields from the user's wording instead, which is exactly the failure the tool
// exists to prevent - and it did so silently, because "no entities" is a valid
// answer for a tenant that genuinely has none.

const (
	tablesColumn     = "tables"
	entityDataColumn = "entity_data"
	entityIdColumn   = "entity_id"
)

// tableRows finds the "tables" array in whatever envelope the result arrived in.
func tableRows(result map[string]interface{}) []interface{} {
	if result == nil {
		return nil
	}
	if tables, ok := result[tablesColumn].([]interface{}); ok {
		return tables
	}
	keys := make([]string, 0, len(result))
	for key := range result {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		switch nested := result[key].(type) {
		case []interface{}:
			for _, item := range nested {
				if row, ok := item.(map[string]interface{}); ok {
					if tables := tableRows(row); len(tables) > 0 {
						return tables
					}
				}
			}
		case map[string]interface{}:
			if tables := tableRows(nested); len(tables) > 0 {
				return tables
			}
		}
	}
	return nil
}

// groupTables turns the tables shape into entities, newest-style first. It
// returns ok=false when the result is not in that shape, so the caller can fall
// back to the flat-row grouping rather than report an empty tenant.
func groupTables(result map[string]interface{}, wanted []string) (entities []metadataEntity, total int, truncated bool, ok bool) {
	tables := tableRows(result)
	if len(tables) == 0 {
		return nil, 0, false, false
	}

	keep := map[string]bool{}
	for _, name := range wanted {
		keep[strings.ToLower(strings.TrimSpace(name))] = true
	}

	entities = []metadataEntity{}
	for _, raw := range tables {
		table, isObject := raw.(map[string]interface{})
		if !isObject {
			continue
		}
		tableName, _ := table["table_name"].(string)
		name, _ := table["entity_name"].(string)
		if strings.TrimSpace(name) == "" {
			// Older results carried only the table name. Falling back to it
			// keeps them readable, even though it is the wrong name to bind to.
			name = tableName
		}
		if strings.TrimSpace(name) == "" {
			continue
		}
		label, _ := table["table_label"].(string)
		if len(keep) > 0 && !keep[strings.ToLower(name)] && !keep[strings.ToLower(tableName)] &&
			!matchesLoosely(keep, name, tableName, label) {
			continue
		}

		entity := metadataEntity{Name: name}
		if strings.TrimSpace(tableName) != "" && tableName != name {
			entity.Table = tableName
		}
		if strings.TrimSpace(label) != "" {
			entity.Label = label
		}

		fields := []metadataField{}
		for _, rawColumn := range asList(table["columns"]) {
			column, isObject := rawColumn.(map[string]interface{})
			if !isObject {
				continue
			}
			if columnName, _ := column["column_name"].(string); columnName == entityIdColumn {
				continue
			}
			for _, rawField := range asList(column["json_schema"]) {
				field, isObject := rawField.(map[string]interface{})
				if !isObject {
					continue
				}
				keyName, _ := field["key_name"].(string)
				if strings.TrimSpace(keyName) == "" {
					continue
				}
				built := metadataField{Name: keyName}
				if label, _ := field["key_label"].(string); strings.TrimSpace(label) != "" {
					built.Label = label
				}
				if dataType, _ := field["data_type"].(string); strings.TrimSpace(dataType) != "" {
					built.Type = dataType
				}
				// The query aliases the entity's `mandatory` flag as
				// "nullable", so the value means mandatory despite the name.
				built.Mandatory = isTrue(field["nullable"])
				fields = append(fields, built)
			}
		}

		if len(fields) == 0 && len(keep) == 0 {
			// An entity with no bindable fields is noise in a list of sixty.
			continue
		}

		// Every entity is always named, even when the budget for fields has run
		// out. Dropping whole entities to stay under the cap is how a model ends
		// up certain an entity does not exist - it asked, and it was not in the
		// answer. Listing it without its fields tells the truth and tells the
		// model what to ask for next.
		if total+len(fields) > MaxEntityMetadataFields {
			truncated = true
			entity.FieldsOmitted = len(fields)
			entities = append(entities, entity)
			continue
		}
		entity.Fields = fields
		total += len(fields)
		entities = append(entities, entity)
	}

	sort.Slice(entities, func(i, j int) bool { return entities[i].Name < entities[j].Name })
	return entities, total, truncated, true
}

// matchesLoosely lets a caller ask for "financiers" or "fn" and still find
// "scf_fn". The entity names carry a process prefix the user never types.
func matchesLoosely(keep map[string]bool, name string, tableName string, label string) bool {
	candidates := []string{strings.ToLower(name), strings.ToLower(tableName), strings.ToLower(label)}
	for wanted := range keep {
		if wanted == "" {
			continue
		}
		singular := strings.TrimSuffix(wanted, "s")
		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}
			if strings.Contains(candidate, wanted) || (singular != wanted && strings.Contains(candidate, singular)) {
				return true
			}
		}
	}
	return false
}

func asList(value interface{}) []interface{} {
	list, _ := value.([]interface{})
	return list
}

func isTrue(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "t", "y", "yes", "1":
			return true
		}
	}
	return false
}
