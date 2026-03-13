package country

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodeToName_MapContainsMajorCountries(t *testing.T) {
	// 测试主要国家的映射
	assert.Equal(t, "China", CodeToName["CN"])
	assert.Equal(t, "United States", CodeToName["US"])
	assert.Equal(t, "Japan", CodeToName["JP"])
	assert.Equal(t, "Germany", CodeToName["DE"])
	assert.Equal(t, "United Kingdom", CodeToName["GB"])
	assert.Equal(t, "France", CodeToName["FR"])
	assert.Equal(t, "Russia", CodeToName["RU"])
	assert.Equal(t, "India", CodeToName["IN"])
	assert.Equal(t, "Brazil", CodeToName["BR"])
	assert.Equal(t, "Australia", CodeToName["AU"])
}

func TestCodeToName_MapContainsAllCodes(t *testing.T) {
	// 验证所有代码都是大写的两位字母
	for code, name := range CodeToName {
		assert.Len(t, code, 2, "Code %s should be 2 characters", code)
		assert.Equal(t, code, strings.ToUpper(code), "Code %s should be uppercase", code)
		assert.NotEmpty(t, name, "Name for code %s should not be empty", code)
	}
}

func TestCodeToName_SpecificCountries(t *testing.T) {
	testCases := []struct {
		code     string
		expected string
	}{
		{"AF", "Afghanistan"},
		{"AL", "Albania"},
		{"DZ", "Algeria"},
		{"AR", "Argentina"},
		{"CA", "Canada"},
		{"CL", "Chile"},
		{"CO", "Colombia"},
		{"EG", "Egypt"},
		{"ID", "Indonesia"},
		{"IR", "Iran"},
		{"IQ", "Iraq"},
		{"IL", "Israel"},
		{"IT", "Italy"},
		{"KR", "Korea, Republic of"},
		{"KP", "Korea, Democratic People's Republic of"},
		{"MY", "Malaysia"},
		{"MX", "Mexico"},
		{"NZ", "New Zealand"},
		{"PK", "Pakistan"},
		{"PH", "Philippines"},
		{"PL", "Poland"},
		{"SA", "Saudi Arabia"},
		{"SG", "Singapore"},
		{"ZA", "South Africa"},
		{"ES", "Spain"},
		{"SE", "Sweden"},
		{"CH", "Switzerland"},
		{"TW", "Taiwan"},
		{"TH", "Thailand"},
		{"TR", "Turkey"},
		{"UA", "Ukraine"},
		{"AE", "United Arab Emirates"},
		{"VN", "Vietnam"},
	}

	for _, tc := range testCases {
		t.Run(tc.code, func(t *testing.T) {
			assert.Equal(t, tc.expected, CodeToName[tc.code])
		})
	}
}

func TestCodeToName_EdgeCases(t *testing.T) {
	// 测试特殊地区
	assert.Equal(t, "Hong Kong", CodeToName["HK"])
	assert.Equal(t, "Macau", CodeToName["MO"])
	assert.Equal(t, "Palestine", CodeToName["PS"])
	assert.Equal(t, "Holy See (Vatican City State)", CodeToName["VA"])
}

func TestCodeToName_UnitedKingdom(t *testing.T) {
	// UK 和 GB 都应该映射到 United Kingdom
	assert.Equal(t, "United Kingdom", CodeToName["UK"])
	assert.Equal(t, "United Kingdom", CodeToName["GB"])
}

func TestGetCountryName_WithValidCode(t *testing.T) {
	testCases := []struct {
		code     string
		expected string
	}{
		{"CN", "China"},
		{"US", "United States"},
		{"JP", "Japan"},
		{"cn", "China"}, // 小写也应该工作
		{"Cn", "China"}, // 混合大小写
	}

	for _, tc := range testCases {
		result := GetCountryName(tc.code)
		assert.Equal(t, tc.expected, result, "Code: %s", tc.code)
	}
}

func TestGetCountryName_WithInvalidCode(t *testing.T) {
	// 无效代码应该返回原值
	assert.Equal(t, "XX", GetCountryName("XX"))
	assert.Equal(t, "Invalid", GetCountryName("Invalid"))
	assert.Equal(t, "", GetCountryName(""))
}

func TestGetCountryName_WithFullName(t *testing.T) {
	// 传入完整国家名应该返回原值
	assert.Equal(t, "China", GetCountryName("China"))
	assert.Equal(t, "United States", GetCountryName("United States"))
	assert.Equal(t, "Unknown Country", GetCountryName("Unknown Country"))
}

func TestCodeToName_TotalCount(t *testing.T) {
	// 验证地图包含足够多的国家代码
	assert.Greater(t, len(CodeToName), 200, "Should have more than 200 country codes")
}

func TestCodeToName_NoEmptyValues(t *testing.T) {
	// 验证没有空值
	for code, name := range CodeToName {
		assert.NotEmpty(t, name, "Country name for code %s should not be empty", code)
	}
}

func TestCodeToName_NoDuplicateCodes(t *testing.T) {
	// 验证没有重复的代码（map 本身保证）
	seen := make(map[string]bool)
	for code := range CodeToName {
		assert.False(t, seen[code], "Duplicate code: %s", code)
		seen[code] = true
	}
}
