package types

// Threat 威胁信息
type Threat struct {
	Type       string                 `json:"type"`
	SubType    string                 `json:"sub_type"`
	Severity   string                 `json:"severity"`
	Message    string                 `json:"message"`
	Parameter  string                 `json:"parameter"`
	Value      string                 `json:"value"`
	RuleID     string                 `json:"rule_id"`
	RuleName   string                 `json:"rule_name"`
	SourceIP   string                 `json:"source_ip"`
	Details    map[string]interface{} `json:"details"`
}