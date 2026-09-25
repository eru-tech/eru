package eval

import agents "github.com/eru-tech/eru/eru-ai/agents"

// Scenarios are the runs we already know the right answer to, because we watched
// each of them go wrong.
//
// Every fixture here is a defect that reached a user: an attachment field
// invented its storage, a four-step plan quietly ran one step, a dashboard bound
// eleven components to queries it never ran and reported them as "confirmed".
// None of these broke a validator, which is exactly why they need to be checked
// here instead.
func Scenarios() Suite {
	return Suite{
		{
			Name:  "attachment_without_storage_asks",
			Agent: "processo_builder",
			// A namespace of its own, and one that is never created: this scenario
			// asserts the agent asks and writes NOTHING, so test_ask_* stays absent
			// for ever and every run tests creation-with-unknown-storage.
			//
			// It shared test_scan_* with the writing scenario until that scenario
			// filled those names in with a storage already set - at which point
			// there was nothing left to ask about and the agent correctly stopped
			// asking, failing this fixture 5/5 for behaving perfectly.
			Prompt: `On the existing entity "test", add a field named test_ask_{{sample}}, ` +
				`label "Test Scan", datatype attachment, on the bd tab.`,
			// ~12s a run, and it is the sharpest ask-vs-guess signal we have.
			Samples: 5,
			// Without the entity the agent asks BECAUSE IT IS MISSING, and this
			// fixture - which asserts exactly "asked, wrote nothing" - passes
			// 3/3 while testing nothing at all.
			RequiresPresent: []string{"test"},
			Why: "storage_name has no default worth guessing. The agent invented \"default\" twice, " +
				"which saves without error and leaves a field pointing at storage that does not exist.",
			Expect: []Expectation{
				// Its correct ending is a pause, not a completion - and until the
				// loop named its exits, those were the same green tick.
				StoppedBecause(agents.StopAskedUser),
				Asked(),
				NeverCalled("save_field"),
			},
		},
		{
			Name:  "attachment_with_storage_writes",
			Agent: "processo_builder",
			Prompt: `On the existing entity "test", add a field named test_scan_{{sample}}, label "Test Scan", ` +
				`datatype attachment, on the bd tab, using storage sample_storage.`,
			Samples:         5,
			RequiresPresent: []string{"test"},
			Why: "The other half of the same rule: told the storage, it must get on with it rather " +
				"than asking anyway.",
			// Outcome, not action. Asserting that save_field was CALLED failed on
			// the second run of the day and was right to: the field was already
			// there from the first, so the correct behaviour was to leave it
			// alone, which an action assertion cannot tell from doing nothing.
			// What the user wants is that the field is an attachment pointed at
			// the storage they named - true whether this run wrote it or found
			// it already correct.
			Expect: []Expectation{
				DidNotAsk(),
				// Not Called("save_field"): on a second run the field is already
				// there and the right move is to leave it. But a write that DOES
				// happen must carry the storage the user named, never an invented
				// default - that claim holds however many times this is run.
				IfCalledWith("save_field", "field.storage_name", "sample_storage"),
				RanToCompletion(),
				ReportsNo("action", "failed"),
			},
			ExpectOutcomeFor: func(sample string) []OutcomeExpectation {
				field := "test_scan_" + sample
				return []OutcomeExpectation{
					EntityFieldHas("test", field, "datatype", "attachment"),
					EntityFieldHas("test", field, "storage_name", "sample_storage"),
				}
			},
		},
		{
			Name:  "capture_request_builds_model_and_page",
			Agent: "processo",
			Prompt: "we need to capture business card details with capability to capture multiple " +
				"addresses and contat details",
			// Minutes a run, and each one leaves entities behind that block the
			// next sample - see RequiresAbsent. More samples need a reset first.
			Samples: 1,
			Why: "A requirement phrased as data still needs a page, and every step after the one " +
				"that asked a question must still run. The first version of this built one entity " +
				"of three and no page at all, and said nothing about it.",
			// Nothing here asserts that a write happened. Twice now an action
			// assertion has failed a correct run: first "creates three entities",
			// when the right move was to extend the three already there, and then
			// "calls save_field", when the model was complete and there was
			// nothing left to write. What the user wants is the destination, and
			// the destination is the same either way.
			Expect: []Expectation{
				RanToCompletion(),
				ReportsNo("action", "failed"),
			},
			// Topics, not names. The same prompt produced "Address" one day and
			// "business_card_address" the next; both are right, and a fixture
			// naming one fails the other. Matched against what this run ADDED,
			// so the long-standing "Contacts" entity cannot satisfy it.
			// Only meaningful against a workspace that does not already have these.
			// Run twice over, the right behaviour is to add nothing, which no
			// outcome assertion can tell apart from having done nothing wrong.
			RequiresAbsent: []string{"business_card"},
			ExpectOutcome: []OutcomeExpectation{
				CoversTopics("address", "contact"),
			},
		},
		{
			Name: "dashboard_probes_every_query_it_binds",
			// Addressed to the page agent itself, not the orchestrator.
			//
			// Run through the orchestrator, this scenario scored green while
			// seeing nothing: the reply carried only structured_output, though
			// the log showed run_query had run four times. The page agent's own
			// tool calls do not reach the orchestrator's traces, so an assertion
			// about them passed for want of evidence rather than because the
			// behaviour was right. Asking the agent directly makes its calls its
			// own reply's traces, and the assertion mean what it says.
			Agent: "eru_studio",
			// Its tool calls used to appear in neither its traces nor its metrics,
			// and this fixture reported BLOCKED. They are now recorded at the tool
			// boundary with their arguments, so the probe can be checked properly.
			NeedsToolVisibility: true,
			Prompt: "build a management dashboard with a row of KPI tiles and a line chart of " +
				"disbursement and repayment, using the saved queries db_tiles_rff, db_tiles_fin, " +
				"db_disb and db_tiles_os",
			// Ten minutes and eight probes a run.
			Samples: 1,
			Why: "A query's response shape is only knowable by running it, and a wrong path renders " +
				"a blank component while the query answers 200. The dashboard bound eleven " +
				"components without a single run_query and reported \"all columns bound from " +
				"confirmed run_query results\".",
			Expect: []Expectation{
				Called("run_query"),
				// Not CalledBefore("run_query", "save_page"): the page agent hands
				// pages back as artifacts and save_page runs later, only if the user
				// accepts them. The ordering check would pass by never firing, which
				// reads as confidence and is worth less than nothing.
				// Restored: the tool record carries query_name for every probe, so
				// this compares what was bound against what was actually run.
				EveryBoundQueryWasProbed("run_query", "query_name"),
				// Probing is not the point; binding to what was probed is. A run
				// passed everything above while shipping a dashboard where not one
				// component showed a number: the tiles carried the row path in
				// their value paths, the charts carried a "result" key that is not
				// in the answer, and the retry dropped the tile bindings
				// altogether rather than correcting them. All three are visible
				// only in the components.
				PagesBindSoundly(),
				BindsEveryQuery("db_tiles_rff", "db_tiles_fin", "db_disb", "db_tiles_os"),
				// A page whose headings are the database's column names is not
				// finished, however sound its bindings are.
				GridsLabelTheirColumns(),
				// The gate that now judges this during the run, checked two ways:
				// that it reached a verdict at all - a silent gate is
				// indistinguishable from a satisfied one - and that the page was
				// not handed over with an objection still standing.
				TheQualityGateRan(),
				PagesAreWorthShipping(),
				RanToCompletion(),
				ReportsNo("action", "failed"),
			},
		},

		// ---- HELD OUT ----
		//
		// Measured, never tuned against. See heldout.go for the rule that goes
		// with them, and for what the gap between the two rates means.
		//
		// All three are eru_studio, and all three are authoring with no writes.
		// That is a real limit and it is better stated than discovered: this
		// held-out set says nothing about processo_builder or the orchestrator,
		// so a gap of zero here is evidence about page building and about
		// nothing else.
		//
		// Each one exercises a rule the validator has carried for weeks with no
		// fixture behind it, which is the nearest thing to untouched ground this
		// codebase has: the behaviour was specified, never measured, and never
		// iterated against.
		{
			Name:    "board_grid_gets_a_card_page",
			HeldOut: true,
			Agent:   "eru_studio",
			Prompt: "build a page showing financiers as a kanban board grouped by status, " +
				"with each card showing the financier name, short name and limit amount",
			Samples: 1,
			Why: "A board draws every record as a card, so the card IS the design. Left without one it falls back " +
				"to a built-in card the user cannot change - a page that looks finished and cannot be edited. " +
				"CodeMountBoardCardUnset has enforced this since the rule table was written; nothing has ever measured it.",
			Expect: []Expectation{
				// Without this the fixture passes by building a table.
				ProducesComponent("grid", map[string]string{"view_mode": "board"}),
				// The board rule lives in the catalog and reaches this through
				// GenericRules, so card_page_id being set is checked here by the
				// same declaration the validator rejects on.
				PagesBindSoundly(),
				RanToCompletion(),
				ReportsNo("action", "failed"),
			},
		},
		{
			// MOVED OUT OF THE HELD-OUT SET on 2026-09-24, under the rule in
			// heldout.go: it failed on the first held-out run, the product was
			// changed because of it, and a fixture cannot measure generalisation
			// twice. Its replacement is entity_page_names_an_entity_that_exists.
			//
			// What it found, in one run: the agent was told twice that the query
			// does not exist and bound a chart to it anyway. The cause was in the
			// harness, not the prompt - run_query recorded "not found" as a TOOL
			// FAILURE, and a failed action is unenforceable, so the first missing
			// query switched off "do not bind a query you have not probed" for
			// the rest of the turn. Ledger.RecordMissingQuery now keeps the two
			// apart and CodeQueryMissing rejects the binding outright.
			Name:  "unknown_query_is_not_invented",
			Agent: "eru_studio",
			Prompt: "build a dashboard with a line chart of monthly collections using the saved query " +
				"db_monthly_collections_summary",
			Samples: 1,
			Why: "The query does not exist. Binding to it anyway produces a component that renders empty behind a 200 - " +
				"the failure mode this whole suite was started over - and the agent has every means to know: " +
				"list_queries answers, and run_query says not found. The right answer is to say so, not to guess.",
			Expect: []Expectation{
				NoComponentSets("query", "db_monthly_collections_summary"),
				// NOT RanToCompletion. Asking is a correct ending here, and the
				// agent now does: told the query does not exist, it stops and
				// says so instead of inventing a binding. Requiring completion
				// would fail it for doing the right thing - the same mistake as
				// asserting a paused run is a finished one.
				ReportsNo("action", "failed"),
			},
		},
		{
			// The held-out set was three eru_studio authoring scenarios while the
			// tune set carried the two hardest writes, and the reported gap came
			// out INVERTED - held-out easier than tune - which the report
			// correctly called a broken split rather than good news. A
			// generalisation gap between sets of different difficulty measures
			// the difference in difficulty.
			//
			// This is the counterweight: a processo_builder WRITE, held out.
			Name:    "field_on_a_missing_entity_is_refused",
			HeldOut: true,
			Agent:   "processo_builder",
			Prompt: `On the entity "warehouse_shipments", add a field named ref_no, ` +
				`label "Reference", datatype textbox, on the bd tab.`,
			Samples: 3,
			// The entity does not exist and never will; the scenario is about what
			// the agent does when asked to build on nothing.
			RequiresAbsent: []string{"warehouse_shipments"},
			Why: "Asked to add a field to an entity that is not there, the agent can ask, or it can create the " +
				"entity it was never asked for. The second is how a data model acquires entities nobody designed - " +
				"and it reads as helpfulness, so nobody questions it.",
			Expect: []Expectation{
				// Either answer is acceptable as long as it did not invent the
				// entity: asking is ideal, reporting the problem is fine.
				NeverCalled("save_entity"),
				ReportsNo("action", "failed"),
			},
		},
		{
			Name:    "entity_page_names_an_entity_that_exists",
			HeldOut: true,
			Agent:   "eru_studio",
			Prompt:  "build a page listing all warehouse shipments in a grid, with their reference and status",
			Samples: 1,
			Why: "There is no warehouse shipment entity in this workspace. The metadata lookup answers, and answers that it " +
				"is not there - so binding a grid to an invented entity name is the same class of fault as binding to a " +
				"query that does not exist, reached through a different lookup. The replacement for the fixture that " +
				"caught the query version: same question, untouched code path.",
			Expect: []Expectation{
				// Whatever it builds, it must not claim a binding to something the
				// metadata never returned.
				PagesBindSoundly(),
				RanToCompletion(),
				ReportsNo("action", "failed"),
			},
		},
		{
			Name:    "entity_page_binds_the_entity_not_the_table",
			HeldOut: true,
			Agent:   "eru_studio",
			Prompt:  "build a page listing financiers in a grid, showing their name, short name and limit amount",
			Samples: 1,
			Why: "Entity \"fn\" lives in table \"scf_fn\" and the metadata answer carries both, so the model picks whichever " +
				"looks more like an identifier. Bound to the table the grid renders nothing. entityBindingIssues catches it " +
				"during a run; this asks whether the agent needs catching.",
			Expect: []Expectation{
				ProducesComponent("grid", nil),
				NoComponentSets("entity_name", "scf_fn"),
				PagesBindSoundly(),
				RanToCompletion(),
				ReportsNo("action", "failed"),
			},
		},
	}
}
