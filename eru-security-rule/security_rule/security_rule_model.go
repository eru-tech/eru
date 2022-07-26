package security_rule

type CustomRule struct {
	AND []CustomRuleDetails `json:",omitempty"`
	OR  []CustomRuleDetails `json:",omitempty"`
}

type CustomRuleDetails struct {
	DataType  string              `json:",omitempty"`
	Variable1 string              `json:",omitempty"`
	Variable2 string              `json:",omitempty"`
	Operator  string              `json:",omitempty"`
	ErrorMsg  string              `json:",omitempty"`
	AND       []CustomRuleDetails `json:",omitempty"`
	OR        []CustomRuleDetails `json:",omitempty"`
}

type SecurityRule struct {
	RuleType   string
	CustomRule CustomRule
}
