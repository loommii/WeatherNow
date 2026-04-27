package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"weatherapi/internal/config"
	"weatherapi/internal/svc"
)

func createAmapMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/ip":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "1",
				"info":   "OK",
				"adcode": "440300",
				"city":   "深圳市",
			})
		case "/weather/weatherInfo":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "1",
				"info":   "OK",
				"lives": []interface{}{
					map[string]interface{}{
						"province":    "广东",
						"city":        "深圳市",
						"adcode":      "440300",
						"weather":     "晴",
						"temperature": "28",
					},
				},
			})
		case "/geocode/regeo":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "1",
				"info":   "OK",
				"regeocode": map[string]interface{}{
					"addressComponent": map[string]interface{}{
						"adcode": "440300",
					},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestGetWeatherHandler_SuccessWithIP(t *testing.T) {
	amapServer := createAmapMockServer()
	defer amapServer.Close()

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			AmapAPIKey:  "test-key",
			AmapBaseURL: amapServer.URL,
		},
	}

	handler := GetWeatherHandler(svcCtx)

	req := httptest.NewRequest(http.MethodGet, "/weather?ip=120.229.15.245", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GetWeatherHandler 返回状态码 = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}

	if resp["city"] != "深圳市" {
		t.Errorf("city = %v, want 深圳市s", resp["city"])
	}
	if resp["icon"] != "sun" {
		t.Errorf("icon = %v, want sun", resp["icon"])
	}
}

func TestGetWeatherHandler_SuccessWithLatLon(t *testing.T) {
	amapServer := createAmapMockServer()
	defer amapServer.Close()

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			AmapAPIKey:  "test-key",
			AmapBaseURL: amapServer.URL,
		},
	}

	handler := GetWeatherHandler(svcCtx)

	req := httptest.NewRequest(http.MethodGet, "/weather?latitude=22.5431&longitude=114.0579", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GetWeatherHandler 返回状态码 = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}

	if resp["city"] != "深圳市" {
		t.Errorf("city = %v, want 深圳市", resp["city"])
	}
}

func TestGetWeatherHandler_NoAPIKey(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			AmapAPIKey: "",
		},
	}

	handler := GetWeatherHandler(svcCtx)

	req := httptest.NewRequest(http.MethodGet, "/weather?ip=120.229.15.245", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatal("GetWeatherHandler 应在缺少 API Key 时返回错误，但返回 200")
	}
}

func TestGetWeatherHandler_IPFromHeader(t *testing.T) {
	amapServer := createAmapMockServer()
	defer amapServer.Close()

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			AmapAPIKey:  "test-key",
			AmapBaseURL: amapServer.URL,
		},
	}

	handler := GetWeatherHandler(svcCtx)

	req := httptest.NewRequest(http.MethodGet, "/weather", nil)
	req.Header.Set("X-Forwarded-For", "120.229.15.245")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GetWeatherHandler 返回状态码 = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestGetWeatherHandler_IPFromRemoteAddr(t *testing.T) {
	amapServer := createAmapMockServer()
	defer amapServer.Close()

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			AmapAPIKey:  "test-key",
			AmapBaseURL: amapServer.URL,
		},
	}

	handler := GetWeatherHandler(svcCtx)

	req := httptest.NewRequest(http.MethodGet, "/weather", nil)
	req.RemoteAddr = "120.229.15.245:12345"
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GetWeatherHandler 返回状态码 = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	tests := []struct {
		name         string
		forwardedFor string
		realIP       string
		remoteAddr   string
		want         string
	}{
		{"X-Forwarded-For单个IP", "120.229.15.245", "", "192.168.1.1:12345", "120.229.15.245"},
		{"X-Forwarded-For多个IP取第一个", "120.229.15.245, 10.0.0.1, 172.16.0.1", "", "192.168.1.1:12345", "120.229.15.245"},
		{"X-Forwarded-For带空格", " 120.229.15.245 ", "", "192.168.1.1:12345", "120.229.15.245"},
		{"X-Real-IP", "", "120.229.15.245", "192.168.1.1:12345", "120.229.15.245"},
		{"X-Real-IP带空格", "", " 120.229.15.245 ", "192.168.1.1:12345", "120.229.15.245"},
		{"X-Forwarded-For优先于X-Real-IP", "120.229.15.245", "10.0.0.1", "192.168.1.1:12345", "120.229.15.245"},
		{"仅RemoteAddr-IPv4", "", "", "192.168.1.1:12345", "192.168.1.1"},
		{"仅RemoteAddr-IPv6", "", "", "[::1]:12345", "::1"},
		{"仅RemoteAddr-无端口", "", "", "192.168.1.1", "192.168.1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/weather", nil)
			if tt.forwardedFor != "" {
				r.Header.Set("X-Forwarded-For", tt.forwardedFor)
			}
			if tt.realIP != "" {
				r.Header.Set("X-Real-IP", tt.realIP)
			}
			r.RemoteAddr = tt.remoteAddr

			got := getClientIP(r)
			if got != tt.want {
				t.Errorf("getClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetClientIP_XForwardedForMultipleProxies(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/weather", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.1, 70.41.3.18, 150.172.238.178")
	r.RemoteAddr = "192.168.1.1:12345"

	got := getClientIP(r)
	if got != "203.0.113.1" {
		t.Errorf("getClientIP() = %q, want %q (应取 X-Forwarded-For 第一个 IP)", got, "203.0.113.1")
	}
}

func TestGetClientIP_EmptyHeaders(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/weather", nil)
	r.RemoteAddr = "10.0.0.1:54321"

	got := getClientIP(r)
	if got != "10.0.0.1" {
		t.Errorf("getClientIP() = %q, want %q", got, "10.0.0.1")
	}
}
