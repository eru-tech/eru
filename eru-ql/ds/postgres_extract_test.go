package ds

import (
	"context"
	"os"
	"reflect"
	"testing"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func TestMain(m *testing.M) {
	logs.LogInit("ds-test", "test-instance")
	os.Exit(m.Run())
}

func TestExtractDMLTargetTables(t *testing.T) {
	pr := &PostgresSqlMaker{}
	ctx := context.Background()

	cases := []struct {
		name string
		sql  string
		want []string
	}{
		{
			name: "simple insert",
			sql:  `insert into org_processes (id, name) values (1, 'foo')`,
			want: []string{"org_processes"},
		},
		{
			name: "simple update",
			sql:  `update org_processes set name = 'foo' where id = 1`,
			want: []string{"org_processes"},
		},
		{
			name: "simple delete",
			sql:  `delete from org_processes where id = 1`,
			want: []string{"org_processes"},
		},
		{
			name: "update with FROM subquery — must NOT include subquery tables",
			sql:  `update org_processes set name = sub.x from (select x from foo, bar) sub where org_processes.id = sub.x`,
			want: []string{"org_processes"},
		},
		{
			name: "schema-qualified target",
			sql:  `update public.org_processes set name = 'foo' where id = 1`,
			want: []string{"public.org_processes"},
		},
		{
			name: "insert with placeholder template",
			sql:  `INSERT INTO orders (id, qty) VALUES $ColsPlaceholder RETURNING id`,
			want: []string{"orders"},
		},
		{
			name: "the user's complex update",
			sql: `update org_processes set meta_data = jsonb_set(meta_data, array['entities', elem_index::text, 'fields'], (elem->'fields')::jsonb - elemf_index) ` +
				`from (select ent.*,(posf - 1)::integer as elemf_index from (select (pos- 1)::integer as elem_index, elem from org_processes, ` +
				`jsonb_array_elements(meta_data->'entities') with ordinality arr(elem, pos) where elem->>'name' = 'prgs' and org_id = 'x' and process_id = 'y') ent, ` +
				`jsonb_array_elements(elem->'fields') with ordinality arrf(elemf, posf) where elemf->>'name' = 'la') sub where org_id = 'x' and process_id = 'y'`,
			want: []string{"org_processes"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pr.ExtractDMLTargetTables(ctx, c.sql)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}
