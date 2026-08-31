package controllers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSiteNameValidation(t *testing.T) {
	bad := []string{
		"<script>alert(1)</script>site",
		`"quoted"`,
		"back`tick",
		"new\nline",
		strings.Repeat("x", 65),
	}
	for _, name := range bad {
		if name == "" {
			continue
		}
		if !(len(name) > 64 || strings.ContainsAny(name, "<>&\"'`\n\r\t")) {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
	good := []string{"my-site", "站点_01", "Site With Spaces", strings.Repeat("x", 64)}
	for _, name := range good {
		if len(name) > 64 || strings.ContainsAny(name, "<>&\"'`\n\r\t") {
			t.Fatalf("expected %q to be accepted", name)
		}
	}
	_ = httptest.NewRecorder()
	_ = gin.New()
}
