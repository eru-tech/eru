package eru

import (
	"reflect"
	"strings"
	"testing"

	utils "github.com/eru-tech/eru/eru-utils"
)

// The variables map is the only place a caller learns how a stored default
// behaves, so the two rules that are easy to get wrong have to be in it: an
// explicitly-passed empty string beats the default, and a template default
// needs the lowercase "none." prefix.
func TestSaveQueryVariablesSchemaCarriesTheDefaultRules(t *testing.T) {
	schema := utils.StructToJSONSchema(reflect.TypeOf(EruqlSaveQueryParams{}), []string{})
	variables, present := schema.Properties["variables"]
	if !present {
		t.Fatal("variables is not in the save_query schema")
	}
	for _, want := range []string{"none.", "overrides", "DEFAULT", "graphql-only", "WITHOUT the $", "$fc never matches"} {
		if !strings.Contains(variables.Description, want) {
			t.Errorf("the variables description does not mention %q: %s", want, variables.Description)
		}
	}
}
