package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"weatherapi/internal/config"
	"weatherapi/internal/svc"
	"weatherapi/internal/types"

	"github.com/patrickmn/go-cache"
	"github.com/zeromicro/go-zero/core/logx"
)

type mockIP2Region struct {
	searchResult string
	searchErr    error
}

func (m *mockIP2Region) Search(ip any) (string, error) {
	return m.searchResult, m.searchErr
}

func (m *mockIP2Region) Close() {}

func newTestLogic(httpClient HTTPClient, ip2RegionSearcher svc.IP2RegionSearcher, amapAPIKey string, amapBaseURL string) *GetWeatherLogic {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			AmapAPIKey: amapAPIKey,
		},
		IP2Region: ip2RegionSearcher,
	}

	if amapBaseURL == "" {
		amapBaseURL = DefaultAmapBaseURL
	}

	return &GetWeatherLogic{
		Logger:      logx.WithContext(context.Background()),
		ctx:         context.Background(),
		svcCtx:      svcCtx,
		cache:       cache.New(5*time.Minute, 10*time.Minute),
		httpClient:  httpClient,
		amapBaseURL: amapBaseURL,
	}
}

func createMockServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func TestGetAdcodeFromLatLon_Success(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		resp := AmapRegeoResponse{
			Status: "1",
			Info:   "OK",
		}
		resp.Regeocode.AddressComponent.Adcode = "440300"
		resp.Regeocode.AddressComponent.City = "深圳市"
		resp.Regeocode.AddressComponent.District = "龙华区"
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	locInfo, err := l.getAdcodeFromLatLon(22.5431, 114.0579)
	if err != nil {
		t.Fatalf("getAdcodeFromLatLon 返回错误: %v", err)
	}
	if locInfo.adcode != "440300" {
		t.Errorf("getAdcodeFromLatLon 返回 adcode = %q, want %q", locInfo.adcode, "440300")
	}
	if locInfo.cityName != "深圳市" {
		t.Errorf("getAdcodeFromLatLon 返回 cityName = %q, want %q", locInfo.cityName, "深圳市")
	}
	if locInfo.district != "龙华区" {
		t.Errorf("getAdcodeFromLatLon 返回 district = %q, want %q", locInfo.district, "龙华区")
	}
}

func TestGetAdcodeFromLatLon_NoAPIKey(t *testing.T) {
	l := newTestLogic(NewDefaultHTTPClient(), nil, "", "")

	_, err := l.getAdcodeFromLatLon(22.5431, 114.0579)
	if !errors.Is(err, ErrAmapKeyNotConfigured) {
		t.Errorf("getAdcodeFromLatLon 返回错误 = %v, want ErrAmapKeyNotConfigured", err)
	}
}

func TestGetAdcodeFromLatLon_APIError(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	_, err := l.getAdcodeFromLatLon(22.5431, 114.0579)
	if err == nil {
		t.Fatal("getAdcodeFromLatLon 应返回错误，但返回 nil")
	}
}

func TestGetAdcodeFromLatLon_StatusNotOne(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		resp := AmapRegeoResponse{
			Status: "0",
			Info:   "INVALID_KEY",
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	_, err := l.getAdcodeFromLatLon(22.5431, 114.0579)
	if err == nil {
		t.Fatal("getAdcodeFromLatLon 应返回错误，但返回 nil")
	}
}

func TestGetAdcodeFromLatLon_EmptyAdcode(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		resp := AmapRegeoResponse{
			Status: "1",
			Info:   "OK",
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	_, err := l.getAdcodeFromLatLon(22.5431, 114.0579)
	if !errors.Is(err, ErrAdcodeNotFound) {
		t.Errorf("getAdcodeFromLatLon 返回错误 = %v, want ErrAdcodeNotFound", err)
	}
}

func TestGetAdcodeFromCity_Success(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		resp := AmapGeoResponse{
			Status: "1",
			Info:   "OK",
		}
		resp.Geocodes = []struct {
			Adcode string `json:"adcode"`
			City   string `json:"city"`
		}{{Adcode: "440300", City: "深圳市"}}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	adcode, err := l.getAdcodeFromCity("深圳市")
	if err != nil {
		t.Fatalf("getAdcodeFromCity 返回错误: %v", err)
	}
	if adcode != "440300" {
		t.Errorf("getAdcodeFromCity 返回 adcode = %q, want %q", adcode, "440300")
	}
}

func TestGetAdcodeFromCity_NoAPIKey(t *testing.T) {
	l := newTestLogic(NewDefaultHTTPClient(), nil, "", "")

	_, err := l.getAdcodeFromCity("深圳市")
	if !errors.Is(err, ErrAmapKeyNotConfigured) {
		t.Errorf("getAdcodeFromCity 返回错误 = %v, want ErrAmapKeyNotConfigured", err)
	}
}

func TestGetAdcodeFromCity_NoResults(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		resp := AmapGeoResponse{
			Status: "1",
			Info:   "OK",
			Geocodes: []struct {
				Adcode string `json:"adcode"`
				City   string `json:"city"`
			}{},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	_, err := l.getAdcodeFromCity("不存在的城市")
	if err == nil {
		t.Fatal("getAdcodeFromCity 应返回错误，但返回 nil")
	}
}

func TestGetAdcodeFromAmapIP_Success(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		resp := AmapIPResponse{
			Status: "1",
			Info:   "OK",
		}
		resp.Adcode = "440300"
		resp.Province = "广东省"
		resp.City = "深圳市"
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	adcode, err := l.getAdcodeFromAmapIP("120.229.15.245")
	if err != nil {
		t.Fatalf("getAdcodeFromAmapIP 返回错误: %v", err)
	}
	if adcode != "440300" {
		t.Errorf("getAdcodeFromAmapIP 返回 adcode = %q, want %q", adcode, "440300")
	}
}

func TestGetAdcodeFromAmapIP_NoAPIKey(t *testing.T) {
	l := newTestLogic(NewDefaultHTTPClient(), nil, "", "")

	_, err := l.getAdcodeFromAmapIP("120.229.15.245")
	if !errors.Is(err, ErrAmapKeyNotConfigured) {
		t.Errorf("getAdcodeFromAmapIP 返回错误 = %v, want ErrAmapKeyNotConfigured", err)
	}
}

func TestGetAdcodeFromAmapIP_EmptyAdcode(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		resp := AmapIPResponse{
			Status: "1",
			Info:   "OK",
		}
		resp.Adcode = []interface{}{}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	_, err := l.getAdcodeFromAmapIP("120.229.15.245")
	if !errors.Is(err, ErrAdcodeNotFound) {
		t.Errorf("getAdcodeFromAmapIP 返回错误 = %v, want ErrAdcodeNotFound", err)
	}
}

func TestGetAdcodeFromAmapIP_ArrayAdcode(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		resp := AmapIPResponse{
			Status: "1",
			Info:   "OK",
		}
		resp.Adcode = []interface{}{"440300"}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	adcode, err := l.getAdcodeFromAmapIP("120.229.15.245")
	if err != nil {
		t.Fatalf("getAdcodeFromAmapIP 返回错误: %v", err)
	}
	if adcode != "440300" {
		t.Errorf("getAdcodeFromAmapIP 返回 adcode = %q, want %q", adcode, "440300")
	}
}

func TestGetAdcodeFromIP2Region_NotInitialized(t *testing.T) {
	l := newTestLogic(NewDefaultHTTPClient(), nil, "test-key", "")

	_, err := l.getAdcodeFromIP2Region("120.229.15.245")
	if !errors.Is(err, ErrIP2RegionNotInitialized) {
		t.Errorf("getAdcodeFromIP2Region 返回错误 = %v, want ErrIP2RegionNotInitialized", err)
	}
}

func TestGetAdcodeFromIP2Region_SearchError(t *testing.T) {
	mockRegion := &mockIP2Region{searchErr: fmt.Errorf("数据库查询失败")}
	l := newTestLogic(NewDefaultHTTPClient(), mockRegion, "test-key", "")

	_, err := l.getAdcodeFromIP2Region("120.229.15.245")
	if err == nil {
		t.Fatal("getAdcodeFromIP2Region 应返回错误，但返回 nil")
	}
}

func TestGetAdcodeFromIP2Region_EmptyResult(t *testing.T) {
	mockRegion := &mockIP2Region{searchResult: ""}
	l := newTestLogic(NewDefaultHTTPClient(), mockRegion, "test-key", "")

	_, err := l.getAdcodeFromIP2Region("120.229.15.245")
	if err == nil {
		t.Fatal("getAdcodeFromIP2Region 应返回错误，但返回 nil")
	}
}

func TestGetAdcodeFromIP2Region_CannotParseCity(t *testing.T) {
	mockRegion := &mockIP2Region{searchResult: "中国|0|0|电信"}
	l := newTestLogic(NewDefaultHTTPClient(), mockRegion, "test-key", "")

	_, err := l.getAdcodeFromIP2Region("120.229.15.245")
	if err == nil {
		t.Fatal("getAdcodeFromIP2Region 应返回错误，但返回 nil")
	}
}

func TestGetAdcodeFromIP_LocalIP(t *testing.T) {
	l := newTestLogic(NewDefaultHTTPClient(), nil, "test-key", "")

	adcode, err := l.getAdcodeFromIP("127.0.0.1")
	if err != nil {
		t.Fatalf("getAdcodeFromIP 返回错误: %v", err)
	}
	if adcode != DefaultAdcode {
		t.Errorf("getAdcodeFromIP 返回 adcode = %q, want %q", adcode, DefaultAdcode)
	}
}

func TestGetAdcodeFromIP_EmptyIP(t *testing.T) {
	l := newTestLogic(NewDefaultHTTPClient(), nil, "test-key", "")

	adcode, err := l.getAdcodeFromIP("")
	if err != nil {
		t.Fatalf("getAdcodeFromIP 返回错误: %v", err)
	}
	if adcode != DefaultAdcode {
		t.Errorf("getAdcodeFromIP 返回 adcode = %q, want %q", adcode, DefaultAdcode)
	}
}

func TestGetAdcodeFromIP_IPv6Fallback(t *testing.T) {
	l := newTestLogic(NewDefaultHTTPClient(), nil, "test-key", "")

	adcode, err := l.getAdcodeFromIP("2001:db8::1")
	if err != nil {
		t.Fatalf("getAdcodeFromIP 返回错误: %v", err)
	}
	if adcode != DefaultAdcode {
		t.Errorf("getAdcodeFromIP 返回 adcode = %q, want %q (IPv6应降级到默认值)", adcode, DefaultAdcode)
	}
}

func TestGetAdcodeFromIP_AmapIPSuccess(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		resp := AmapIPResponse{
			Status: "1",
			Info:   "OK",
		}
		resp.Adcode = "440300"
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	adcode, err := l.getAdcodeFromIP("120.229.15.245")
	if err != nil {
		t.Fatalf("getAdcodeFromIP 返回错误: %v", err)
	}
	if adcode != "440300" {
		t.Errorf("getAdcodeFromIP 返回 adcode = %q, want %q", adcode, "440300")
	}
}

func TestGetAdcodeFromIP_AllFailFallback(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		resp := AmapIPResponse{
			Status: "0",
			Info:   "INVALID_KEY",
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	adcode, err := l.getAdcodeFromIP("120.229.15.245")
	if err != nil {
		t.Fatalf("getAdcodeFromIP 返回错误: %v", err)
	}
	if adcode != DefaultAdcode {
		t.Errorf("getAdcodeFromIP 返回 adcode = %q, want %q (所有方式失败应降级到默认值)", adcode, DefaultAdcode)
	}
}

func TestGetWeather_ByIP(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ip" {
			resp := AmapIPResponse{Status: "1", Info: "OK"}
			resp.Adcode = "440300"
			resp.City = "深圳市"
			json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/weather/weatherInfo" {
			resp := AmapWeatherResponse{Status: "1", Info: "OK"}
			resp.Lives = []struct {
				Province      string `json:"province"`
				City          string `json:"city"`
				Adcode        string `json:"adcode"`
				Weather       string `json:"weather"`
				Temperature   string `json:"temperature"`
				Winddirection string `json:"winddirection"`
				Windpower     string `json:"windpower"`
				Humidity      string `json:"humidity"`
				Reporttime    string `json:"reporttime"`
			}{{
				Province:    "广东",
				City:        "深圳市",
				Adcode:      "440300",
				Weather:     "晴",
				Temperature: "28",
			}}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	resp, err := l.GetWeather(&types.WeatherRequest{IP: "120.229.15.245"})
	if err != nil {
		t.Fatalf("GetWeather 返回错误: %v", err)
	}
	if resp.City != "深圳市" {
		t.Errorf("GetWeather 返回 City = %q, want %q", resp.City, "深圳市")
	}
	if resp.Temperature != 28 {
		t.Errorf("GetWeather 返回 Temperature = %v, want %v", resp.Temperature, 28)
	}
	if resp.Icon != "sun" {
		t.Errorf("GetWeather 返回 Icon = %q, want %q", resp.Icon, "sun")
	}
	if resp.Description != "晴" {
		t.Errorf("GetWeather 返回 Description = %q, want %q", resp.Description, "晴")
	}
}

func TestGetWeather_ByLatLon(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/geocode/regeo" {
			resp := AmapRegeoResponse{Status: "1", Info: "OK"}
			resp.Regeocode.AddressComponent.Adcode = "440300"
			json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/weather/weatherInfo" {
			resp := AmapWeatherResponse{Status: "1", Info: "OK"}
			resp.Lives = []struct {
				Province      string `json:"province"`
				City          string `json:"city"`
				Adcode        string `json:"adcode"`
				Weather       string `json:"weather"`
				Temperature   string `json:"temperature"`
				Winddirection string `json:"winddirection"`
				Windpower     string `json:"windpower"`
				Humidity      string `json:"humidity"`
				Reporttime    string `json:"reporttime"`
			}{{
				City:        "深圳市",
				Weather:     "多云",
				Temperature: "25",
			}}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	lat := 22.5431
	lon := 114.0579
	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	resp, err := l.GetWeather(&types.WeatherRequest{Latitude: &lat, Longitude: &lon})
	if err != nil {
		t.Fatalf("GetWeather 返回错误: %v", err)
	}
	if resp.City != "深圳市" {
		t.Errorf("GetWeather 返回 City = %q, want %q", resp.City, "深圳市")
	}
	if resp.Icon != "cloud" {
		t.Errorf("GetWeather 返回 Icon = %q, want %q", resp.Icon, "cloud")
	}
}

func TestGetWeather_NoAPIKey(t *testing.T) {
	l := newTestLogic(NewDefaultHTTPClient(), nil, "", "")

	_, err := l.GetWeather(&types.WeatherRequest{IP: "120.229.15.245"})
	if !errors.Is(err, ErrAmapKeyNotConfigured) {
		t.Errorf("GetWeather 返回错误 = %v, want ErrAmapKeyNotConfigured", err)
	}
}

func TestGetWeather_CacheHit(t *testing.T) {
	callCount := 0
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Path == "/ip" {
			resp := AmapIPResponse{Status: "1", Info: "OK"}
			resp.Adcode = "440300"
			json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/weather/weatherInfo" {
			resp := AmapWeatherResponse{Status: "1", Info: "OK"}
			resp.Lives = []struct {
				Province      string `json:"province"`
				City          string `json:"city"`
				Adcode        string `json:"adcode"`
				Weather       string `json:"weather"`
				Temperature   string `json:"temperature"`
				Winddirection string `json:"winddirection"`
				Windpower     string `json:"windpower"`
				Humidity      string `json:"humidity"`
				Reporttime    string `json:"reporttime"`
			}{{
				City:        "深圳市",
				Weather:     "晴",
				Temperature: "28",
			}}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	resp1, err := l.GetWeather(&types.WeatherRequest{IP: "120.229.15.245"})
	if err != nil {
		t.Fatalf("第一次 GetWeather 返回错误: %v", err)
	}

	resp2, err := l.GetWeather(&types.WeatherRequest{IP: "120.229.15.245"})
	if err != nil {
		t.Fatalf("第二次 GetWeather 返回错误: %v", err)
	}

	if callCount != 2 {
		t.Errorf("API 调用次数 = %d, 应只调用2次（IP+天气各1次），第二次从缓存获取", callCount)
	}

	if resp1.Temperature != resp2.Temperature {
		t.Errorf("缓存结果不一致: 第一次 = %v, 第二次 = %v", resp1.Temperature, resp2.Temperature)
	}
}

func TestGetWeather_InvalidTemperature(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ip" {
			resp := AmapIPResponse{Status: "1", Info: "OK"}
			resp.Adcode = "440300"
			json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/weather/weatherInfo" {
			resp := AmapWeatherResponse{Status: "1", Info: "OK"}
			resp.Lives = []struct {
				Province      string `json:"province"`
				City          string `json:"city"`
				Adcode        string `json:"adcode"`
				Weather       string `json:"weather"`
				Temperature   string `json:"temperature"`
				Winddirection string `json:"winddirection"`
				Windpower     string `json:"windpower"`
				Humidity      string `json:"humidity"`
				Reporttime    string `json:"reporttime"`
			}{{
				City:        "深圳市",
				Weather:     "晴",
				Temperature: "invalid",
			}}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	_, err := l.GetWeather(&types.WeatherRequest{IP: "120.229.15.245"})
	if err == nil {
		t.Fatal("GetWeather 应在温度无效时返回错误，但返回 nil")
	}
}

func TestGetWeather_NoLiveData(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ip" {
			resp := AmapIPResponse{Status: "1", Info: "OK"}
			resp.Adcode = "440300"
			json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/weather/weatherInfo" {
			resp := AmapWeatherResponse{Status: "1", Info: "OK"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	_, err := l.GetWeather(&types.WeatherRequest{IP: "120.229.15.245"})
	if err == nil {
		t.Fatal("GetWeather 应在无实时天气数据时返回错误，但返回 nil")
	}
}

func TestNewGetWeatherLogic(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			AmapAPIKey:  "test-key",
			AmapBaseURL: "http://localhost:9999",
		},
	}

	l := NewGetWeatherLogic(context.Background(), svcCtx)

	if l == nil {
		t.Fatal("NewGetWeatherLogic 返回 nil")
	}
	if l.amapBaseURL != "http://localhost:9999" {
		t.Errorf("amapBaseURL = %q, want %q", l.amapBaseURL, "http://localhost:9999")
	}
	if l.cache == nil {
		t.Error("cache 不应为 nil")
	}
	if l.httpClient == nil {
		t.Error("httpClient 不应为 nil")
	}
}

func TestNewGetWeatherLogic_DefaultBaseURL(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			AmapAPIKey: "test-key",
		},
	}

	l := NewGetWeatherLogic(context.Background(), svcCtx)

	if l.amapBaseURL != DefaultAmapBaseURL {
		t.Errorf("amapBaseURL = %q, want %q (未配置时应使用默认值)", l.amapBaseURL, DefaultAmapBaseURL)
	}
}

func TestGetAdcodeFromIP_IP2RegionSuccess(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/geocode/geo" {
			resp := AmapGeoResponse{Status: "1", Info: "OK"}
			resp.Geocodes = []struct {
				Adcode string `json:"adcode"`
				City   string `json:"city"`
			}{{Adcode: "440300", City: "深圳市"}}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	mockRegion := &mockIP2Region{searchResult: "中国|广东省|深圳市|电信"}
	l := newTestLogic(server.Client(), mockRegion, "test-key", server.URL)

	adcode, err := l.getAdcodeFromIP("120.229.15.245")
	if err != nil {
		t.Fatalf("getAdcodeFromIP 返回错误: %v", err)
	}
	if adcode != "440300" {
		t.Errorf("getAdcodeFromIP 返回 adcode = %q, want %q", adcode, "440300")
	}
}

func TestGetAdcodeFromIP_IP2RegionFailThenAmapSuccess(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ip" {
			resp := AmapIPResponse{Status: "1", Info: "OK"}
			resp.Adcode = "440300"
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	mockRegion := &mockIP2Region{searchErr: fmt.Errorf("查询失败")}
	l := newTestLogic(server.Client(), mockRegion, "test-key", server.URL)

	adcode, err := l.getAdcodeFromIP("120.229.15.245")
	if err != nil {
		t.Fatalf("getAdcodeFromIP 返回错误: %v", err)
	}
	if adcode != "440300" {
		t.Errorf("getAdcodeFromIP 返回 adcode = %q, want %q (ip2region 失败后应降级到高德)", adcode, "440300")
	}
}

func TestGetWeather_HTTPRequestFail(t *testing.T) {
	l := newTestLogic(NewDefaultHTTPClient(), nil, "test-key", "http://127.0.0.1:1")

	_, err := l.GetWeather(&types.WeatherRequest{IP: "120.229.15.245"})
	if err == nil {
		t.Fatal("GetWeather 应在 HTTP 请求失败时返回错误，但返回 nil")
	}
}

func TestGetAdcodeFromLatLon_InvalidJSON(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "invalid json")
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	_, err := l.getAdcodeFromLatLon(22.5431, 114.0579)
	if err == nil {
		t.Fatal("getAdcodeFromLatLon 应在 JSON 无效时返回错误，但返回 nil")
	}
}

func TestGetAdcodeFromCity_InvalidJSON(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "invalid json")
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	_, err := l.getAdcodeFromCity("深圳市")
	if err == nil {
		t.Fatal("getAdcodeFromCity 应在 JSON 无效时返回错误，但返回 nil")
	}
}

func TestGetAdcodeFromAmapIP_InvalidJSON(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "invalid json")
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	_, err := l.getAdcodeFromAmapIP("120.229.15.245")
	if err == nil {
		t.Fatal("getAdcodeFromAmapIP 应在 JSON 无效时返回错误，但返回 nil")
	}
}

func TestGetAdcodeFromAmapIP_StatusNotOne(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		resp := AmapIPResponse{Status: "0", Info: "INVALID_KEY"}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	_, err := l.getAdcodeFromAmapIP("120.229.15.245")
	if err == nil {
		t.Fatal("getAdcodeFromAmapIP 应在 API 状态错误时返回错误，但返回 nil")
	}
}

func TestGetAdcodeFromAmapIP_LocalIP(t *testing.T) {
	server := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		resp := AmapIPResponse{Status: "1", Info: "OK"}
		resp.Adcode = "110000"
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	l := newTestLogic(server.Client(), nil, "test-key", server.URL)

	adcode, err := l.getAdcodeFromAmapIP("127.0.0.1")
	if err != nil {
		t.Fatalf("getAdcodeFromAmapIP 返回错误: %v", err)
	}
	if adcode != "110000" {
		t.Errorf("getAdcodeFromAmapIP 返回 adcode = %q, want %q", adcode, "110000")
	}
}

func TestGetAdcodeFromAmapIP_IPv6Skipped(t *testing.T) {
	l := newTestLogic(NewDefaultHTTPClient(), nil, "test-key", "")

	_, err := l.getAdcodeFromAmapIP("2001:db8::1")
	if err == nil {
		t.Fatal("getAdcodeFromAmapIP 应在 IPv6 地址时返回错误（高德不支持 IPv6），但返回 nil")
	}
}
