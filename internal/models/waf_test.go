package models

import (
	"errors"
	"strings"
	"testing"
)

func TestWAFRule_Validate(t *testing.T) {
	tests := []struct {
		name    string
		rule    *WAFRule
		wantErr error
	}{
		{
			name: "valid rule",
			rule: &WAFRule{
				ID:       "rule-1",
				Name:     "Block SQL Injection",
				Pattern:  `(?i)(union\s+select|select\s+.*\s+from)`,
				Action:   WAFActionBlock,
				Priority: 1,
				Enabled:  true,
			},
			wantErr: nil,
		},
		{
			name: "valid rule with allow action",
			rule: &WAFRule{
				ID:       "rule-2",
				Name:     "Allow Admin Path",
				Pattern:  `^/admin/.*`,
				Action:   WAFActionAllow,
				Priority: 10,
				Enabled:  true,
			},
			wantErr: nil,
		},
		{
			name: "valid rule with challenge action",
			rule: &WAFRule{
				ID:       "rule-3",
				Name:     "Challenge Suspicious Requests",
				Pattern:  `.*`,
				Action:   WAFActionChallenge,
				Priority: 5,
				Enabled:  false,
			},
			wantErr: nil,
		},
		{
			name: "valid rule with log action",
			rule: &WAFRule{
				ID:       "rule-4",
				Name:     "Log All Requests",
				Pattern:  `.*`,
				Action:   WAFActionLog,
				Priority: 100,
				Enabled:  true,
			},
			wantErr: nil,
		},
		{
			name: "valid rule with empty pattern",
			rule: &WAFRule{
				ID:       "rule-5",
				Name:     "Default Rule",
				Pattern:  "",
				Action:   WAFActionAllow,
				Priority: 1000,
				Enabled:  true,
			},
			wantErr: nil,
		},
		{
			name: "missing name",
			rule: &WAFRule{
				ID:       "rule-6",
				Name:     "",
				Pattern:  `.*`,
				Action:   WAFActionBlock,
				Priority: 1,
				Enabled:  true,
			},
			wantErr: ErrWAFRuleNameEmpty,
		},
		{
			name: "invalid regex pattern",
			rule: &WAFRule{
				ID:       "rule-7",
				Name:     "Invalid Pattern Rule",
				Pattern:  `[invalid(regex`,
				Action:   WAFActionBlock,
				Priority: 1,
				Enabled:  true,
			},
			wantErr: ErrPatternInvalid,
		},
		{
			name: "invalid action",
			rule: &WAFRule{
				ID:       "rule-8",
				Name:     "Invalid Action Rule",
				Pattern:  `.*`,
				Action:   "invalid_action",
				Priority: 1,
				Enabled:  true,
			},
			wantErr: ErrWAFRuleActionInvalid,
		},
		{
			name: "empty action is valid",
			rule: &WAFRule{
				ID:       "rule-9",
				Name:     "No Action Rule",
				Pattern:  `.*`,
				Action:   "",
				Priority: 1,
				Enabled:  true,
			},
			wantErr: nil,
		},
		{
			name: "complex valid regex pattern",
			rule: &WAFRule{
				ID:       "rule-10",
				Name:     "Complex XSS Pattern",
				Pattern:  `(?i)<script[^>]*>.*?</script>`,
				Action:   WAFActionBlock,
				Priority: 1,
				Enabled:  true,
			},
			wantErr: nil,
		},
		{
			name: "case sensitive invalid action",
			rule: &WAFRule{
				ID:       "rule-11",
				Name:     "Case Sensitive Action",
				Pattern:  `.*`,
				Action:   "BLOCK",
				Priority: 1,
				Enabled:  true,
			},
			wantErr: ErrWAFRuleActionInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("WAFRule.Validate() expected error %v, got nil", tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) && !strings.Contains(err.Error(), tt.wantErr.Error()) {
					t.Errorf("WAFRule.Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("WAFRule.Validate() unexpected error = %v", err)
			}
		})
	}
}
