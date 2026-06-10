package detectors

import (
	"net/http"
	"strings"

	"prerender-shield/internal/firewall/types"
)

type UserAgentDetector struct {
	name string
}

func NewUserAgentDetector() *UserAgentDetector {
	return &UserAgentDetector{name: "UserAgent"}
}

func (d *UserAgentDetector) Name() string {
	return d.name
}

var maliciousUAKeywords = []string{
	"nikto", "nmap", "nessus", "openvas", "w3af", "wpscan",
	"sqlmap", "burp", "acunetix", "netsparker", "appscan",
	"arachni", "gobuster", "dirbuster", "wfuzz", "masscan",
	"zmap", "hydra", "medusa", "hashcat", "johntheripper",
}

func (d *UserAgentDetector) Detect(req *http.Request) ([]types.Threat, error) {
	ua := req.Header.Get("User-Agent")
	if ua == "" {
		return []types.Threat{{
			Type:     "user_agent",
			SubType:  "Missing User-Agent",
			Severity: "low",
			Message:  "Request without User-Agent header",
			RuleID:   "ua-001",
			RuleName: "Missing User-Agent",
		}}, nil
	}

	uaLower := strings.ToLower(ua)
	for _, keyword := range maliciousUAKeywords {
		if strings.Contains(uaLower, keyword) {
			return []types.Threat{{
				Type:     "user_agent",
				SubType:  "Malicious Scanner",
				Severity: "high",
				Message:  "Known malicious scanner detected: " + keyword,
				Value:    ua,
				RuleID:   "ua-002",
				RuleName: "Malicious Scanner",
			}}, nil
		}
	}

	return nil, nil
}
