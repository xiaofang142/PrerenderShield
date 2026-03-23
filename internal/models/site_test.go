package models

import (
	"testing"
)

func TestSiteConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *SiteConfig
		wantErr error
	}{
		{
			name: "valid proxy mode config",
			config: &SiteConfig{
				Domain:    "example.com",
				Mode:      RenderModeProxy,
				TargetURL: "http://localhost:3000",
				Port:      8080,
			},
			wantErr: nil,
		},
		{
			name: "valid static mode config",
			config: &SiteConfig{
				Domain: "example.com",
				Mode:   RenderModeStatic,
				Port:   80,
			},
			wantErr: nil,
		},
		{
			name: "valid redirect mode config",
			config: &SiteConfig{
				Domain:    "example.com",
				Mode:      RenderModeRedirect,
				TargetURL: "https://newdomain.com",
				Port:      443,
			},
			wantErr: nil,
		},
		{
			name: "missing domain",
			config: &SiteConfig{
				Mode:      RenderModeProxy,
				TargetURL: "http://localhost:3000",
			},
			wantErr: ErrDomainEmpty,
		},
		{
			name: "invalid port - negative",
			config: &SiteConfig{
				Domain:    "example.com",
				Mode:      RenderModeProxy,
				TargetURL: "http://localhost:3000",
				Port:      -1,
			},
			wantErr: ErrPortInvalid,
		},
		{
			name: "invalid port - too large",
			config: &SiteConfig{
				Domain:    "example.com",
				Mode:      RenderModeProxy,
				TargetURL: "http://localhost:3000",
				Port:      65536,
			},
			wantErr: ErrPortInvalid,
		},
		{
			name: "invalid mode",
			config: &SiteConfig{
				Domain:    "example.com",
				Mode:      "invalid_mode",
				TargetURL: "http://localhost:3000",
			},
			wantErr: ErrModeInvalid,
		},
		{
			name: "proxy mode missing target_url",
			config: &SiteConfig{
				Domain: "example.com",
				Mode:   RenderModeProxy,
				Port:   8080,
			},
			wantErr: ErrTargetURLEmpty,
		},
		{
			name: "redirect mode missing target_url",
			config: &SiteConfig{
				Domain: "example.com",
				Mode:   RenderModeRedirect,
				Port:   8080,
			},
			wantErr: ErrTargetURLEmpty,
		},
		{
			name: "static mode without target_url is valid",
			config: &SiteConfig{
				Domain: "example.com",
				Mode:   RenderModeStatic,
				Port:   80,
			},
			wantErr: nil,
		},
		{
			name: "port zero is valid",
			config: &SiteConfig{
				Domain:    "example.com",
				Mode:      RenderModeProxy,
				TargetURL: "http://localhost:3000",
				Port:      0,
			},
			wantErr: nil,
		},
		{
			name: "max valid port",
			config: &SiteConfig{
				Domain:    "example.com",
				Mode:      RenderModeProxy,
				TargetURL: "http://localhost:3000",
				Port:      65535,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err != tt.wantErr {
				t.Errorf("SiteConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSite_EnabledInt(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected int
	}{
		{
			name:     "enabled true returns 1",
			enabled:  true,
			expected: 1,
		},
		{
			name:     "enabled false returns 0",
			enabled:  false,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site := &Site{
				ID:      "test-site",
				Domain:  "example.com",
				Name:    "Test Site",
				Enabled: tt.enabled,
			}
			result := site.EnabledInt()
			if result != tt.expected {
				t.Errorf("Site.EnabledInt() = %d, expected %d", result, tt.expected)
			}
		})
	}
}
