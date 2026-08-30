package services

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/oschwald/geoip2-golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testMMDBPath = "testdata/GeoLite2-City-Test.mmdb"

// routeByHost 按 URL Host 路由到不同的假响应，隔离真实网络
func routeByHost(routes map[string]roundTripperFunc) roundTripperFunc {
	return func(r *http.Request) (*http.Response, error) {
		if h, ok := routes[r.URL.Host]; ok {
			return h(r)
		}
		return nil, fmt.Errorf("no fake route for host %s", r.URL.Host)
	}
}

func newLocalDB(t *testing.T) *geoip2.Reader {
	t.Helper()
	db, err := geoip2.Open(testMMDBPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestNewGeoIPService_BadDBPath(t *testing.T) {
	// 注意：不要在 NewGeoIPService 之后改写 client.Transport——
	// 构造器已启动后台 initServerLocation goroutine，并发写 Transport 会产生数据竞争
	service := NewGeoIPService("/nonexistent/definitely-missing.mmdb")

	assert.NotNil(t, service)
	assert.Nil(t, service.localDB)
}

func TestNewGeoIPService_ValidDBPath(t *testing.T) {
	service := NewGeoIPService(testMMDBPath)

	require.NotNil(t, service.localDB)

	// Close 应关闭本地数据库并返回 nil
	assert.NoError(t, service.Close())
}

func TestGeoIPService_LookupCountryISO_PrivateIPWithServerLocation(t *testing.T) {
	service := &GeoIPService{
		client: &http.Client{Transport: roundTripperFunc(failRT)},
	}
	service.serverLocation = &GeoLocation{CountryCode: "ZZ", Country: "Testland"}

	code, err := service.LookupCountryISO("127.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, "ZZ", code)
}

func TestGeoIPService_LookupCountryISO_FromLocalDB(t *testing.T) {
	service := &GeoIPService{
		client:  &http.Client{Transport: roundTripperFunc(failRT)},
		localDB: newLocalDB(t),
	}

	code, err := service.LookupCountryISO("81.2.69.160")
	require.NoError(t, err)
	assert.Equal(t, "GB", code)

	// 查询结果应写入缓存，第二次命中缓存
	cached, ok := service.cache.Load("81.2.69.160")
	require.True(t, ok)
	assert.Equal(t, "GB", cached.(*GeoLocation).CountryCode)
}

func TestGeoIPService_LookupCountryISO_APIError_NoServerLocation(t *testing.T) {
	service := &GeoIPService{
		client: &http.Client{Transport: roundTripperFunc(failRT)},
	}

	code, err := service.LookupCountryISO("8.8.8.8")
	require.NoError(t, err)
	assert.Equal(t, "Local", code)
}

func TestGeoIPService_LookupCountryISO_APIError_WithServerLocation(t *testing.T) {
	service := &GeoIPService{
		client: &http.Client{Transport: roundTripperFunc(failRT)},
	}
	service.serverLocation = &GeoLocation{CountryCode: "YY"}

	code, err := service.LookupCountryISO("8.8.8.8")
	require.NoError(t, err)
	assert.Equal(t, "YY", code)
}

func TestGeoIPService_QueryLocalDB(t *testing.T) {
	service := &GeoIPService{localDB: newLocalDB(t)}

	t.Run("invalid ip string", func(t *testing.T) {
		loc, err := service.queryLocalDB("not-an-ip")
		require.Error(t, err)
		assert.Nil(t, loc)
		assert.Contains(t, err.Error(), "invalid IP")
	})

	t.Run("ipv6 address on ipv4-only database", func(t *testing.T) {
		loc, err := service.queryLocalDB("2001:4860:4860::8888")
		require.Error(t, err)
		assert.Nil(t, loc)
	})

	t.Run("no country data", func(t *testing.T) {
		// 8.8.8.8 不在测试数据库内 → 空记录 → 无国家数据
		loc, err := service.queryLocalDB("8.8.8.8")
		require.Error(t, err)
		assert.Nil(t, loc)
		assert.Contains(t, err.Error(), "no country data")
	})

	t.Run("hit", func(t *testing.T) {
		loc, err := service.queryLocalDB("81.2.69.160")
		require.NoError(t, err)
		require.NotNil(t, loc)
		assert.Equal(t, "GB", loc.CountryCode)
		assert.Equal(t, "United Kingdom", loc.Country)
		assert.Equal(t, "London", loc.City)
	})
}

func TestGeoIPService_GetLocation_PrivateIPWithServerLocation(t *testing.T) {
	service := &GeoIPService{
		client: &http.Client{Transport: roundTripperFunc(failRT)},
	}
	expected := &GeoLocation{Country: "Testland", CountryCode: "ZZ", City: "Test City"}
	service.serverLocation = expected

	loc, err := service.GetLocation("10.0.0.5")
	require.NoError(t, err)
	assert.Same(t, expected, loc)
}

func TestGeoIPService_GetLocation_FromLocalDB(t *testing.T) {
	service := &GeoIPService{
		client:  &http.Client{Transport: roundTripperFunc(failRT)},
		localDB: newLocalDB(t),
	}

	loc, err := service.GetLocation("81.2.69.160")
	require.NoError(t, err)
	require.NotNil(t, loc)
	assert.Equal(t, "GB", loc.CountryCode)
}

func TestGeoIPService_GetLocation_APIError_WithServerLocation(t *testing.T) {
	service := &GeoIPService{
		client: &http.Client{Transport: roundTripperFunc(failRT)},
	}
	service.serverLocation = &GeoLocation{CountryCode: "YY"}

	loc, err := service.GetLocation("8.8.8.8")
	require.NoError(t, err)
	assert.Equal(t, "YY", loc.CountryCode)
}

func TestGeoIPService_GetLocation_APIError_NoServerLocation(t *testing.T) {
	service := &GeoIPService{
		client: &http.Client{Transport: roundTripperFunc(failRT)},
	}

	loc, err := service.GetLocation("8.8.8.8")
	require.Error(t, err, "所有 provider 失败且无本机位置时应返回错误")
	assert.Nil(t, loc)
}

func TestGeoIPService_FetchServerLocation_IPAPISuccess(t *testing.T) {
	service := &GeoIPService{
		client: &http.Client{Transport: routeByHost(map[string]roundTripperFunc{
			"ip-api.com": func(r *http.Request) (*http.Response, error) {
				return httpResp(http.StatusOK, `{"status":"success","country":"China","countryCode":"CN","city":"Beijing","lat":39.9,"lon":116.4}`)
			},
		})},
	}

	loc, err := service.fetchServerLocation()
	require.NoError(t, err)
	require.NotNil(t, loc)
	assert.Equal(t, "China", loc.Country)
	assert.Equal(t, "CN", loc.CountryCode)
	assert.Equal(t, "Beijing", loc.City)
	assert.Equal(t, 39.9, loc.Latitude)
	assert.Equal(t, 116.4, loc.Longitude)
}

func TestGeoIPService_FetchServerLocation_GeoJSFallback(t *testing.T) {
	service := &GeoIPService{
		client: &http.Client{Transport: routeByHost(map[string]roundTripperFunc{
			"ip-api.com": func(r *http.Request) (*http.Response, error) {
				return httpResp(http.StatusInternalServerError, "boom")
			},
			"get.geojs.io": func(r *http.Request) (*http.Response, error) {
				return httpResp(http.StatusOK, `{"country":"United States","country_code":"US","city":"New York","latitude":"40.71","longitude":"-74.00"}`)
			},
		})},
	}

	loc, err := service.fetchServerLocation()
	require.NoError(t, err)
	require.NotNil(t, loc)
	assert.Equal(t, "United States", loc.Country)
	assert.Equal(t, "US", loc.CountryCode)
	assert.Equal(t, "New York", loc.City)
	assert.Equal(t, 40.71, loc.Latitude)
	assert.Equal(t, -74.00, loc.Longitude)
}

func TestGeoIPService_QueryIPAPI(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		status  int
		wantLoc *GeoLocation
		wantErr string
	}{
		{
			name:   "success",
			status: http.StatusOK,
			body:   `{"status":"success","country":"Japan","countryCode":"JP","city":"Tokyo","lat":35.68,"lon":139.69}`,
			wantLoc: &GeoLocation{
				Country: "Japan", CountryCode: "JP", City: "Tokyo",
				Latitude: 35.68, Longitude: 139.69,
			},
		},
		{
			name:    "fail status",
			status:  http.StatusOK,
			body:    `{"status":"fail"}`,
			wantErr: "api returned fail status",
		},
		{
			name:    "non-200 status",
			status:  http.StatusInternalServerError,
			body:    "oops",
			wantErr: "status code: 500",
		},
		{
			name:    "invalid json",
			status:  http.StatusOK,
			body:    "{invalid",
			wantErr: "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &GeoIPService{
				client: &http.Client{Transport: routeByHost(map[string]roundTripperFunc{
					"ip-api.com": func(r *http.Request) (*http.Response, error) {
						return httpResp(tt.status, tt.body)
					},
				})},
			}

			loc, err := service.queryIPAPI("1.2.3.4")
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, loc)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantLoc, loc)
		})
	}
}

func TestGeoIPService_QueryIPAPIco(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		status  int
		wantLoc *GeoLocation
		wantErr string
	}{
		{
			name:   "success",
			status: http.StatusOK,
			body:   `{"country_name":"France","country_code":"FR","city":"Paris","latitude":48.85,"longitude":2.35}`,
			wantLoc: &GeoLocation{
				Country: "France", CountryCode: "FR", City: "Paris",
				Latitude: 48.85, Longitude: 2.35,
			},
		},
		{
			name:    "api error flag",
			status:  http.StatusOK,
			body:    `{"error":true,"reason":"rate limited"}`,
			wantErr: "api returned error",
		},
		{
			name:    "non-200 status",
			status:  http.StatusTooManyRequests,
			body:    "slow down",
			wantErr: "status code: 429",
		},
		{
			name:    "invalid json",
			status:  http.StatusOK,
			body:    "not-json",
			wantErr: "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &GeoIPService{
				client: &http.Client{Transport: routeByHost(map[string]roundTripperFunc{
					"ipapi.co": func(r *http.Request) (*http.Response, error) {
						return httpResp(tt.status, tt.body)
					},
				})},
			}

			loc, err := service.queryIPAPIco("1.2.3.4")
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, loc)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantLoc, loc)
		})
	}
}

func TestGeoIPService_QueryGeoJS(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		status  int
		wantLoc *GeoLocation
		wantErr string
	}{
		{
			name:   "success",
			status: http.StatusOK,
			body:   `{"country":"Germany","country_code":"DE","city":"Berlin","latitude":"52.52","longitude":"13.40"}`,
			wantLoc: &GeoLocation{
				Country: "Germany", CountryCode: "DE", City: "Berlin",
				Latitude: 52.52, Longitude: 13.40,
			},
		},
		{
			name:    "non-200 status",
			status:  http.StatusForbidden,
			body:    "denied",
			wantErr: "status code: 403",
		},
		{
			name:    "invalid json",
			status:  http.StatusOK,
			body:    "}}}",
			wantErr: "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &GeoIPService{
				client: &http.Client{Transport: routeByHost(map[string]roundTripperFunc{
					"get.geojs.io": func(r *http.Request) (*http.Response, error) {
						return httpResp(tt.status, tt.body)
					},
				})},
			}

			loc, err := service.queryGeoJS("1.2.3.4")
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, loc)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantLoc, loc)
		})
	}
}

func TestIsPrivateIP_InvalidString(t *testing.T) {
	// 无法解析的字符串既不是私网也不应 panic
	assert.False(t, isPrivateIP("not-an-ip"))
}

// TestGeoIPService_InitServerLocation_RetryThenSuccess 覆盖重试休眠与成功路径（约 2 秒）
func TestGeoIPService_InitServerLocation_RetryThenSuccess(t *testing.T) {
	t.Parallel()

	var calls int32
	service := &GeoIPService{
		client: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			// 每次 fetchServerLocation 依次请求 ip-api 与 geojs，
			// 让第 0 轮的两个请求都失败，第 1 轮的 ip-api 返回成功
			if atomic.AddInt32(&calls, 1) <= 2 {
				return nil, fmt.Errorf("first round fails")
			}
			return httpResp(http.StatusOK, `{"status":"success","country":"China","countryCode":"CN","city":"Beijing","lat":39.9,"lon":116.4}`)
		})},
	}

	service.initServerLocation()

	service.mu.RLock()
	loc := service.serverLocation
	service.mu.RUnlock()
	require.NotNil(t, loc)
	assert.Equal(t, "CN", loc.CountryCode)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(2))
}

// TestGeoIPService_InitServerLocation_AllAttemptsFail 覆盖 5 次重试全部失败后的告警路径（约 10 秒）
func TestGeoIPService_InitServerLocation_AllAttemptsFail(t *testing.T) {
	t.Parallel()

	service := &GeoIPService{
		client: &http.Client{Transport: roundTripperFunc(failRT)},
	}

	service.initServerLocation()

	service.mu.RLock()
	loc := service.serverLocation
	service.mu.RUnlock()
	assert.Nil(t, loc)
}
