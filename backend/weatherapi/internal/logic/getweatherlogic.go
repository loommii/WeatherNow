package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"weatherapi/internal/svc"
	"weatherapi/internal/types"

	"github.com/patrickmn/go-cache"
	"github.com/zeromicro/go-zero/core/logx"
)

var (
	// ErrAmapKeyNotConfigured 当高德 API Key 缺失时返回。
	ErrAmapKeyNotConfigured = errors.New("amap API key is not configured")
	// ErrIP2RegionNotInitialized 当 ip2region 服务不可用时返回。
	ErrIP2RegionNotInitialized = errors.New("ip2region service not initialized")
	// ErrAdcodeNotFound 当无法解析 adcode 时返回。
	ErrAdcodeNotFound = errors.New("adcode not found")
)

const (
	// DefaultAdcode 是所有查询方式均失败时的兜底 adcode（北京）。
	DefaultAdcode = "110000"
	// AmapHTTPTimeout 是高德 API HTTP 请求超时时间。
	AmapHTTPTimeout = 10 * time.Second
	// DefaultAmapBaseURL 是高德 API 的默认基础 URL。
	DefaultAmapBaseURL = "https://restapi.amap.com/v3"
)

// HTTPClient 定义 HTTP 客户端接口，用于解耦外部 HTTP 调用，方便测试时注入 mock。
type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

// defaultHTTPClient 是生产环境使用的带超时 HTTP 客户端。
type defaultHTTPClient struct {
	client *http.Client
}

func (c *defaultHTTPClient) Get(url string) (*http.Response, error) {
	return c.client.Get(url)
}

// NewDefaultHTTPClient 创建带超时的默认 HTTP 客户端。
func NewDefaultHTTPClient() HTTPClient {
	return &defaultHTTPClient{
		client: &http.Client{
			Timeout: AmapHTTPTimeout,
		},
	}
}

// AmapRegeoResponse 表示高德逆地理编码 API 的响应结构。
type AmapRegeoResponse struct {
	Status    string `json:"status"`
	Info      string `json:"info"`
	Infocode  string `json:"infocode"`
	Regeocode struct {
		AddressComponent struct {
			Adcode   string      `json:"adcode"`
			City     interface{} `json:"city"`
			District string      `json:"district"`
		} `json:"addressComponent"`
	} `json:"regeocode"`
}

// AmapGeoResponse 表示高德地理编码 API 的响应结构。
type AmapGeoResponse struct {
	Status   string `json:"status"`
	Info     string `json:"info"`
	Infocode string `json:"infocode"`
	Geocodes []struct {
		Adcode string `json:"adcode"`
		City   string `json:"city"`
	} `json:"geocodes"`
}

// AmapWeatherResponse 表示高德天气查询 API 的响应结构。
type AmapWeatherResponse struct {
	Status   string `json:"status"`
	Info     string `json:"info"`
	Infocode string `json:"infocode"`
	Lives    []struct {
		Province      string `json:"province"`
		City          string `json:"city"`
		Adcode        string `json:"adcode"`
		Weather       string `json:"weather"`
		Temperature   string `json:"temperature"`
		Winddirection string `json:"winddirection"`
		Windpower     string `json:"windpower"`
		Humidity      string `json:"humidity"`
		Reporttime    string `json:"reporttime"`
	} `json:"lives"`
}

// AmapIPResponse 表示高德 IP 定位 API 的响应结构。
type AmapIPResponse struct {
	Status   string      `json:"status"`
	Info     string      `json:"info"`
	Infocode string      `json:"infocode"`
	Province interface{} `json:"province"`
	City     interface{} `json:"city"`
	Adcode   interface{} `json:"adcode"`
}

// GetWeatherLogic 封装天气查询的业务逻辑，
// 包括 IP 定位、adcode 解析和天气数据获取。
type GetWeatherLogic struct {
	logx.Logger
	ctx         context.Context
	svcCtx      *svc.ServiceContext
	cache       *cache.Cache
	httpClient  HTTPClient
	amapBaseURL string
}

// NewGetWeatherLogic 根据给定上下文和服务上下文创建 GetWeatherLogic 实例。
func NewGetWeatherLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWeatherLogic {
	c := cache.New(5*time.Minute, 10*time.Minute)
	baseURL := svcCtx.Config.AmapBaseURL
	if baseURL == "" {
		baseURL = DefaultAmapBaseURL
	}
	return &GetWeatherLogic{
		Logger:      logx.WithContext(ctx),
		ctx:         ctx,
		svcCtx:      svcCtx,
		cache:       c,
		httpClient:  NewDefaultHTTPClient(),
		amapBaseURL: baseURL,
	}
}

// isIPv6 判断给定 IP 字符串是否为 IPv6 地址。
func isIPv6(ip string) bool {
	return strings.Contains(ip, ":")
}

// isLocalIP 判断给定 IP 是否为本机回环或空地址。
func isLocalIP(ip string) bool {
	return ip == "" || ip == "127.0.0.1" || ip == "::1" || ip == "[::1]"
}

// parseCityFromRegion 从 ip2region 查询结果中提取城市名称。
// 结果格式为"国家|省份|城市|ISP"。对于直辖市（北京、上海、天津、重庆），
// 由于城市字段通常为空，会返回带"市"后缀的省份名。
func parseCityFromRegion(region string) string {
	parts := strings.Split(region, "|")
	if len(parts) < 3 {
		return ""
	}

	province := strings.TrimSpace(parts[1])
	city := strings.TrimSpace(parts[2])

	if city != "" && city != "0" {
		return city
	}

	if province != "" && province != "0" {
		municipalities := []string{"北京", "上海", "天津", "重庆"}
		for _, m := range municipalities {
			if strings.HasPrefix(province, m) {
				return m + "市"
			}
		}
		return province
	}

	return ""
}

// parseAmapIPField 从高德 IP 响应字段中提取字符串值，
// 该字段可能返回字符串或字符串数组。
func parseAmapIPField(field interface{}) string {
	switch v := field.(type) {
	case string:
		return v
	case []interface{}:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

// mapWeatherToIcon 将中文天气描述映射为图标标识。
func mapWeatherToIcon(weather string) string {
	switch weather {
	case "晴":
		return "sun"
	case "多云":
		return "cloud"
	case "阴":
		return "cloud"
	case "阵雨", "雷阵雨", "小雨", "中雨", "大雨", "暴雨", "大暴雨", "特大暴雨":
		return "rain"
	case "雨夹雪", "小雪", "中雪", "大雪", "暴雪":
		return "snow"
	case "雾", "霾":
		return "cloud"
	default:
		return "unknown"
	}
}

// maskAPIKey 将 URL 中的高德 API Key 替换为星号，用于安全日志输出。
func maskAPIKey(rawURL, apiKey string) string {
	if apiKey == "" {
		return rawURL
	}
	return strings.ReplaceAll(rawURL, apiKey, "********")
}

// buildDisplayCity 组合完整城市展示名称。
// regeoCity 是逆地理编码返回的市级名称（如"深圳市"），
// regeoDistrict 是逆地理编码返回的区级名称（如"龙华区"），
// weatherCity 是天气 API 返回的城市名称（区级 adcode 时为区名，如"龙华区"）。
// 组合规则：市名 + 区名，避免重复。例如 "深圳市" + "龙华区" = "深圳市龙华区"。
func buildDisplayCity(regeoCity, regeoDistrict, weatherCity string) string {
	if regeoCity != "" && regeoDistrict != "" {
		return regeoCity + regeoDistrict
	}
	if regeoCity != "" {
		return regeoCity
	}
	return weatherCity
}

// locationInfo 包含从逆地理编码获取的位置信息。
type locationInfo struct {
	adcode   string
	cityName string
	district string
}

// getAdcodeFromLatLon 通过高德逆地理编码 API 将经纬度坐标解析为位置信息。
func (l *GetWeatherLogic) getAdcodeFromLatLon(latitude, longitude float64) (*locationInfo, error) {
	amapAPIKey := l.svcCtx.Config.AmapAPIKey
	if amapAPIKey == "" {
		return nil, ErrAmapKeyNotConfigured
	}

	baseURL := l.amapBaseURL + "/geocode/regeo"
	params := url.Values{}
	params.Add("location", fmt.Sprintf("%f,%f", longitude, latitude))
	params.Add("key", amapAPIKey)
	params.Add("output", "json")

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	l.Debugf("调用高德逆地理编码 API，请求 URL: %s", maskAPIKey(fullURL, amapAPIKey))

	httpResp, err := l.httpClient.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("getAdcodeFromLatLon: HTTP 请求失败: %w", err)
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("getAdcodeFromLatLon: 读取响应体失败: %w", err)
	}
	l.Debugf("高德逆地理编码 API 响应 - 状态: %s, 内容: %s", httpResp.Status, string(bodyBytes))

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getAdcodeFromLatLon: API 返回状态码 %d, 内容: %s", httpResp.StatusCode, string(bodyBytes))
	}

	var regeoResp AmapRegeoResponse
	if err := json.Unmarshal(bodyBytes, &regeoResp); err != nil {
		return nil, fmt.Errorf("getAdcodeFromLatLon: 解析响应 JSON 失败: %w", err)
	}

	if regeoResp.Status != "1" {
		return nil, fmt.Errorf("getAdcodeFromLatLon: API 状态错误, info: %s", regeoResp.Info)
	}

	adcode := regeoResp.Regeocode.AddressComponent.Adcode
	if adcode == "" {
		return nil, fmt.Errorf("getAdcodeFromLatLon: %w", ErrAdcodeNotFound)
	}

	cityName := parseAmapIPField(regeoResp.Regeocode.AddressComponent.City)

	return &locationInfo{
		adcode:   adcode,
		cityName: cityName,
		district: regeoResp.Regeocode.AddressComponent.District,
	}, nil
}

// getAdcodeFromCity 通过高德地理编码 API 将城市名称解析为 adcode。
func (l *GetWeatherLogic) getAdcodeFromCity(cityName string) (string, error) {
	amapAPIKey := l.svcCtx.Config.AmapAPIKey
	if amapAPIKey == "" {
		return "", ErrAmapKeyNotConfigured
	}

	baseURL := l.amapBaseURL + "/geocode/geo"
	params := url.Values{}
	params.Add("address", cityName)
	params.Add("key", amapAPIKey)
	params.Add("output", "json")

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	l.Debugf("调用高德地理编码 API，请求 URL: %s", maskAPIKey(fullURL, amapAPIKey))

	httpResp, err := l.httpClient.Get(fullURL)
	if err != nil {
		return "", fmt.Errorf("getAdcodeFromCity: HTTP 请求失败: %w", err)
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", fmt.Errorf("getAdcodeFromCity: 读取响应体失败: %w", err)
	}
	l.Debugf("高德地理编码 API 响应 - 状态: %s, 内容: %s", httpResp.Status, string(bodyBytes))

	if httpResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("getAdcodeFromCity: API 返回状态码 %d, 内容: %s", httpResp.StatusCode, string(bodyBytes))
	}

	var geoResp AmapGeoResponse
	if err := json.Unmarshal(bodyBytes, &geoResp); err != nil {
		return "", fmt.Errorf("getAdcodeFromCity: 解析响应 JSON 失败: %w", err)
	}

	if geoResp.Status != "1" || len(geoResp.Geocodes) == 0 {
		return "", fmt.Errorf("getAdcodeFromCity: 城市名 %q 无查询结果", cityName)
	}

	return geoResp.Geocodes[0].Adcode, nil
}

// getAdcodeFromIP2Region 通过 ip2region 离线数据库将 IP 地址解析为城市名，
// 再通过高德地理编码 API 转换为 adcode。
func (l *GetWeatherLogic) getAdcodeFromIP2Region(ip string) (string, error) {
	if l.svcCtx.IP2Region == nil {
		return "", ErrIP2RegionNotInitialized
	}

	region, err := l.svcCtx.IP2Region.Search(ip)
	if err != nil {
		return "", fmt.Errorf("getAdcodeFromIP2Region: 查询 IP %s 失败: %w", ip, err)
	}

	if region == "" {
		return "", fmt.Errorf("getAdcodeFromIP2Region: IP %s 查询结果为空", ip)
	}

	l.Infof("ip2region 解析 IP %s 到区域: %s", ip, region)

	cityName := parseCityFromRegion(region)
	if cityName == "" {
		return "", fmt.Errorf("getAdcodeFromIP2Region: 从区域 %q 解析城市名失败", region)
	}

	l.Infof("解析到城市名: %s，通过高德地理编码 API 转换为 adcode", cityName)
	return l.getAdcodeFromCity(cityName)
}

// getAdcodeFromAmapIP 通过高德 IP 定位 API 将 IPv4 地址解析为 adcode。
// 高德 IP API 不支持 IPv6，IPv6 地址将导致 ip 参数被省略。
func (l *GetWeatherLogic) getAdcodeFromAmapIP(ip string) (string, error) {
	amapAPIKey := l.svcCtx.Config.AmapAPIKey
	if amapAPIKey == "" {
		return "", ErrAmapKeyNotConfigured
	}

	baseURL := l.amapBaseURL + "/ip"
	params := url.Values{}
	params.Add("key", amapAPIKey)
	if !isLocalIP(ip) && !isIPv6(ip) {
		params.Add("ip", ip)
	}
	params.Add("output", "json")

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	l.Debugf("调用高德 IP 定位 API，请求 URL: %s", maskAPIKey(fullURL, amapAPIKey))

	httpResp, err := l.httpClient.Get(fullURL)
	if err != nil {
		return "", fmt.Errorf("getAdcodeFromAmapIP: HTTP 请求失败: %w", err)
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", fmt.Errorf("getAdcodeFromAmapIP: 读取响应体失败: %w", err)
	}
	l.Debugf("高德 IP 定位 API 响应 - 状态: %s, 内容: %s", httpResp.Status, string(bodyBytes))

	if httpResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("getAdcodeFromAmapIP: API 返回状态码 %d, 内容: %s", httpResp.StatusCode, string(bodyBytes))
	}

	var ipResp AmapIPResponse
	if err := json.Unmarshal(bodyBytes, &ipResp); err != nil {
		return "", fmt.Errorf("getAdcodeFromAmapIP: 解析响应 JSON 失败: %w", err)
	}

	if ipResp.Status != "1" {
		return "", fmt.Errorf("getAdcodeFromAmapIP: API 错误, info: %s", ipResp.Info)
	}

	adcode := parseAmapIPField(ipResp.Adcode)
	if adcode == "" {
		return "", fmt.Errorf("getAdcodeFromAmapIP: %w", ErrAdcodeNotFound)
	}

	return adcode, nil
}

// getAdcodeFromIP 通过降级链从 IP 地址解析 adcode：
//  1. ip2region 离线查询（支持 IPv4 和 IPv6）
//  2. 高德 IP API（仅 IPv4；高德不支持 IPv6）
//  3. 默认 adcode（北京 110000）
func (l *GetWeatherLogic) getAdcodeFromIP(ip string) (string, error) {
	if !isLocalIP(ip) && l.svcCtx.IP2Region != nil {
		adcode, err := l.getAdcodeFromIP2Region(ip)
		if err == nil {
			return adcode, nil
		}
		l.Infof("ip2region 查询失败: %v，降级到下一方式", err)
	}

	if !isIPv6(ip) {
		adcode, err := l.getAdcodeFromAmapIP(ip)
		if err == nil {
			return adcode, nil
		}
		l.Infof("高德 IP API 查询失败: %v，降级到默认值（北京）", err)
	} else {
		l.Infof("IPv6 地址 (%s) 不被高德 IP API 支持，降级到默认值（北京）", ip)
	}

	return DefaultAdcode, nil
}

// GetWeather 根据请求获取当前天气数据。
// 通过坐标或 IP 解析位置，查询高德天气 API，并缓存结果供后续请求使用。
func (l *GetWeatherLogic) GetWeather(req *types.WeatherRequest) (resp *types.WeatherResponse, err error) {
	var cacheKey string
	if req.Latitude != nil && req.Longitude != nil {
		cacheKey = fmt.Sprintf("weather:%f,%f", *req.Latitude, *req.Longitude)
	} else {
		cacheKey = fmt.Sprintf("weather:ip:%s", req.IP)
	}

	if x, found := l.cache.Get(cacheKey); found {
		if cachedResp, ok := x.(*types.WeatherResponse); ok {
			l.Infof("从缓存返回天气数据，key: %s", cacheKey)
			return cachedResp, nil
		}
	}

	amapAPIKey := l.svcCtx.Config.AmapAPIKey
	if amapAPIKey == "" {
		return nil, ErrAmapKeyNotConfigured
	}

	var adcode string
	var locInfo *locationInfo
	if req.Latitude != nil && req.Longitude != nil {
		locInfo, err = l.getAdcodeFromLatLon(*req.Latitude, *req.Longitude)
		if err != nil {
			return nil, fmt.Errorf("GetWeather: 解析 adcode 失败: %w", err)
		}
		adcode = locInfo.adcode
	} else {
		adcode, err = l.getAdcodeFromIP(req.IP)
		if err != nil {
			return nil, fmt.Errorf("GetWeather: 解析 adcode 失败: %w", err)
		}
	}

	baseURL := l.amapBaseURL + "/weather/weatherInfo"
	params := url.Values{}
	params.Add("city", adcode)
	params.Add("key", amapAPIKey)
	params.Add("extensions", "base")
	params.Add("output", "json")

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	l.Debugf("调用高德天气 API，请求 URL: %s", maskAPIKey(fullURL, amapAPIKey))

	httpResp, err := l.httpClient.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("GetWeather: HTTP 请求失败: %w", err)
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("GetWeather: 读取响应体失败: %w", err)
	}
	l.Debugf("高德天气 API 响应 - 状态: %s, 内容: %s", httpResp.Status, string(bodyBytes))

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GetWeather: API 返回状态码 %d, 内容: %s", httpResp.StatusCode, string(bodyBytes))
	}

	var weatherResp AmapWeatherResponse
	if err := json.Unmarshal(bodyBytes, &weatherResp); err != nil {
		return nil, fmt.Errorf("GetWeather: 解析响应 JSON 失败: %w", err)
	}

	if weatherResp.Status != "1" || len(weatherResp.Lives) == 0 {
		return nil, fmt.Errorf("GetWeather: 无实时天气数据, info: %s", weatherResp.Info)
	}

	liveWeather := weatherResp.Lives[0]
	icon := mapWeatherToIcon(liveWeather.Weather)

	tempFloat, err := strconv.ParseFloat(liveWeather.Temperature, 64)
	if err != nil {
		return nil, fmt.Errorf("GetWeather: 解析温度 %q 失败: %w", liveWeather.Temperature, err)
	}

	displayCity := liveWeather.City
	if locInfo != nil {
		displayCity = buildDisplayCity(locInfo.cityName, locInfo.district, liveWeather.City)
	}

	resp = &types.WeatherResponse{
		Temperature: tempFloat,
		Description: liveWeather.Weather,
		City:        displayCity,
		Icon:        icon,
	}

	l.cache.Set(cacheKey, resp, cache.DefaultExpiration)
	return resp, nil
}
