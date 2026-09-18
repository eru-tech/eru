package catalog

// Code names what is wrong, independently of how it is worded. Two things need
// that separation: the retry path, which compares a page's pre-existing issues
// against the ones an edit introduced and must not be fooled by a reworded
// message, and the eval harness, which counts issues by kind across runs.
//
// The message stays free to change - it is what the model reads. The code is
// what the code reads, so it is append-only: renaming one silently changes the
// meaning of every recorded run.
type Code string

const (
	CodePageUnknownKey        Code = "page_unknown_key"
	CodePageMissingKey        Code = "page_missing_key"
	CodePageComponentsNotList Code = "page_components_not_list"

	CodeComponentNotObject          Code = "component_not_object"
	CodeComponentMissingId          Code = "component_missing_id"
	CodeComponentDuplicateId        Code = "component_duplicate_id"
	CodeComponentMissingType        Code = "component_missing_type"
	CodeComponentUnknownType        Code = "component_unknown_type"
	CodeComponentDeprecatedType     Code = "component_deprecated_type"
	CodeComponentUnknownKey         Code = "component_unknown_key"
	CodeComponentMissingProperties  Code = "component_missing_properties"
	CodeComponentMissingStyles      Code = "component_missing_styles"
	CodeComponentChildrenNotAllowed Code = "component_children_not_allowed"
	CodePageHostChildrenIds         Code = "page_host_children_ids"
	CodeChildrenIdsNotList          Code = "children_ids_not_list"
	CodeChildrenNotList             Code = "children_not_list"

	CodePropertiesNotObject           Code = "properties_not_object"
	CodePropertiesNotResponsive       Code = "properties_not_responsive"
	CodePropertiesUnknownBreakpoint   Code = "properties_unknown_breakpoint"
	CodePropertiesBreakpointNotObject Code = "properties_breakpoint_not_object"
	CodePropertyUnknown               Code = "property_unknown"
	CodePropertyValueNotAllowed       Code = "property_value_not_allowed"

	CodeStylesNotObject         Code = "styles_not_object"
	CodeStylesUnknownKey        Code = "styles_unknown_key"
	CodeStylesUnknownBreakpoint Code = "styles_unknown_breakpoint"

	CodeEventsNotList                Code = "events_not_list"
	CodeEventNotObject               Code = "event_not_object"
	CodeEventUnknownKey              Code = "event_unknown_key"
	CodeEventMissingAction           Code = "event_missing_action"
	CodeEventUnknownAction           Code = "event_unknown_action"
	CodeEventActionMissingField      Code = "event_action_missing_field"
	CodeEventActionMissingFieldNames Code = "event_action_missing_field_names"
	CodeEventMissingEvent            Code = "event_missing_event"
	CodeEventNotEmitted              Code = "event_not_emitted"
	CodeEventMissingId               Code = "event_missing_id"

	CodeValidationRulesNotList       Code = "validation_rules_not_list"
	CodeValidationRuleNotObject      Code = "validation_rule_not_object"
	CodeValidationRuleUnknownKey     Code = "validation_rule_unknown_key"
	CodeValidationRuleMissingType    Code = "validation_rule_missing_type"
	CodeValidationRuleUnknownType    Code = "validation_rule_unknown_type"
	CodeValidationRuleMissingMessage Code = "validation_rule_missing_message"

	CodeStateNotList           Code = "state_not_list"
	CodeStateVariableNotObject Code = "state_variable_not_object"
	CodeStateUnknownKey        Code = "state_unknown_key"
	CodeStateMissingKey        Code = "state_missing_key"

	// Mounts and nested pages. Raised by the nesting checks rather than the
	// component validator, but they are the same kind of fact about a page.
	CodeMountPageRefUnset     Code = "mount_page_ref_unset"
	CodeMountBoardCardUnset   Code = "mount_board_card_unset"
	CodeMountTargetMissing    Code = "mount_target_missing"
	CodeNestedPageMissingId   Code = "nested_page_missing_id"
	CodeNestedPageUnmounted   Code = "nested_page_unmounted"
	CodeNestedPagesUnreadable Code = "nested_pages_unreadable"
	CodeNestedPageNoMountedAt Code = "nested_page_no_mounted_at"

	// Envelope-level complaints about the shape of the answer itself.
	CodeEnvelopeFullMissingPage Code = "envelope_full_missing_page"
	CodeEnvelopeUnknownMode     Code = "envelope_unknown_mode"
	CodeEnvelopeFullDropsPage   Code = "envelope_full_drops_page"
	CodeFilterWithoutDefault    Code = "filter_without_default"

	CodeDuplicateStateFieldName Code = "duplicate_state_field_name"
	CodePatchUnreadable         Code = "patch_unreadable"
	CodePatchEmpty              Code = "patch_empty"
	CodePatchScopeViolation     Code = "patch_scope_violation"
	CodePatchNotApplicable      Code = "patch_not_applicable"
	CodePatchUnknownChild       Code = "patch_unknown_child"
	CodeReferencePageNotRead    Code = "reference_page_not_read"

	// Preflight: the page claims a binding the agent never looked up.
	CodeBindingsWithoutMetadata Code = "bindings_without_metadata"

	// Layout: mistakes that render as a page nobody would have drawn on purpose.
	// They are not schema errors - every one of these pages is valid JSON that
	// the renderer accepts and then draws badly.
	CodeLayoutSqueezedPane   Code = "layout_squeezed_pane"
	CodeLayoutWidthsOverflow Code = "layout_widths_overflow"
	CodeLayoutFixedColumns   Code = "layout_fixed_columns"

	// Planning: what is wrong with the design before any of it is drawn.
	CodePlanNoRoot        Code = "plan_no_root"
	CodePlanNoPages       Code = "plan_no_pages"
	CodePlanPageNoId      Code = "plan_page_no_id"
	CodePlanDuplicateId   Code = "plan_duplicate_id"
	CodePlanPageUnmounted Code = "plan_page_unmounted"

	// A page bound to the physical table instead of the entity.
	CodeEntityIsATableName Code = "entity_is_a_table_name"

	// Page chrome decorated against the theme rather than with it.
	CodeChromeGradient        Code = "chrome_gradient"
	CodeChromeHardCodedColour Code = "chrome_hard_coded_colour"

	// A tabs component whose children do not match its tabs.
	CodeTabsChildNotAPanel Code = "tabs_child_not_a_panel"
	CodeTabsPanelCount     Code = "tabs_panel_count"
)
