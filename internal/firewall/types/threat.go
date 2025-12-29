package types

// Threat 威胁信息
type Threat struct {
	Type       string
	SubType    string
	Severity   string
	Message    string
	Parameter  string
	Value      string
	RuleID     string
	RuleName   string
}