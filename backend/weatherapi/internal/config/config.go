package config

import "github.com/zeromicro/go-zero/rest"

// Config 定义应用配置，包含 REST 服务设置、高德 API Key 和 ip2region 离线数据库路径。
type Config struct {
	rest.RestConf
	// Environment 指定运行环境：dev、sit 或 prod。
	Environment string `json:"environment,optional,env=ENV"`
	// AmapAPIKey 是高德 Web 服务的 API Key。
	AmapAPIKey string `json:"amapApiKey,optional,env=AMAP_API_KEY"`
	// AmapBaseURL 是高德 API 的基础 URL，默认为 https://restapi.amap.com/v3。用于测试时注入 mock 服务器。
	AmapBaseURL string `json:"amapBaseUrl,optional,env=AMAP_BASE_URL"`
	// IP2RegionV4DBPath 是 ip2region IPv4 xdb 数据库文件路径。
	IP2RegionV4DBPath string `json:"ip2regionV4DbPath,optional,env=IP2REGION_V4_DB_PATH"`
	// IP2RegionV6DBPath 是 ip2region IPv6 xdb 数据库文件路径。
	IP2RegionV6DBPath string `json:"ip2regionV6DbPath,optional" env:"IP2REGION_V6_DB_PATH"`
	// CORSAllowOrigins 指定允许跨域的来源域名列表。dev 环境默认允许所有来源。
	CORSAllowOrigins []string `json:"corsAllowOrigins,optional"`
}
