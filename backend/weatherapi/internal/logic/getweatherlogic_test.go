package logic

import "testing"

func TestIsIPv6(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"IPv4地址", "192.168.1.1", false},
		{"IPv6完整地址", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", true},
		{"IPv6缩写地址", "::1", true},
		{"IPv6带方括号", "[::1]", true},
		{"空字符串", "", false},
		{"IPv4回环地址", "127.0.0.1", false},
		{"IPv6简写", "fe80::1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIPv6(tt.ip); got != tt.want {
				t.Errorf("isIPv6(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestIsLocalIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"空字符串", "", true},
		{"IPv4回环地址", "127.0.0.1", true},
		{"IPv6回环地址", "::1", true},
		{"IPv6回环带方括号", "[::1]", true},
		{"普通IPv4", "192.168.1.1", false},
		{"公网IPv4", "120.229.15.245", false},
		{"IPv6地址", "2001:db8::1", false},
		{"localhost", "localhost", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLocalIP(tt.ip); got != tt.want {
				t.Errorf("isLocalIP(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestParseCityFromRegion(t *testing.T) {
	tests := []struct {
		name   string
		region string
		want   string
	}{
		{"标准格式", "中国|广东省|深圳市|电信", "深圳市"},
		{"直辖市-北京", "中国|北京|0|联通", "北京市"},
		{"直辖市-上海", "中国|上海市|0|电信", "上海市"},
		{"直辖市-天津", "中国|天津市|0|联通", "天津市"},
		{"直辖市-重庆", "中国|重庆市|0|电信", "重庆市"},
		{"省份无城市", "中国|河北省|0|电信", "河北省"},
		{"城市为空字符串", "中国|广东省||电信", "广东省"},
		{"分隔符不足", "中国|广东省", ""},
		{"空字符串", "", ""},
		{"省份和城市都为0", "中国|0|0|电信", ""},
		{"省份为0城市有值", "中国|0|深圳市|电信", "深圳市"},
		{"带空格", "中国 | 广东省 | 深圳市 | 电信", "深圳市"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseCityFromRegion(tt.region); got != tt.want {
				t.Errorf("parseCityFromRegion(%q) = %q, want %q", tt.region, got, tt.want)
			}
		})
	}
}

func TestParseAmapIPField(t *testing.T) {
	tests := []struct {
		name  string
		field interface{}
		want  string
	}{
		{"字符串值", "440300", "440300"},
		{"字符串数组", []interface{}{"440300"}, "440300"},
		{"空字符串数组", []interface{}{}, ""},
		{"数组首元素非字符串", []interface{}{123}, ""},
		{"整数值", 123, ""},
		{"nil值", nil, ""},
		{"空字符串", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseAmapIPField(tt.field); got != tt.want {
				t.Errorf("parseAmapIPField(%v) = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}

func TestMapWeatherToIcon(t *testing.T) {
	tests := []struct {
		name    string
		weather string
		want    string
	}{
		{"晴", "晴", "sun"},
		{"多云", "多云", "cloud"},
		{"阴", "阴", "cloud"},
		{"小雨", "小雨", "rain"},
		{"中雨", "中雨", "rain"},
		{"大雨", "大雨", "rain"},
		{"暴雨", "暴雨", "rain"},
		{"大暴雨", "大暴雨", "rain"},
		{"特大暴雨", "特大暴雨", "rain"},
		{"阵雨", "阵雨", "rain"},
		{"雷阵雨", "雷阵雨", "rain"},
		{"小雪", "小雪", "snow"},
		{"中雪", "中雪", "snow"},
		{"大雪", "大雪", "snow"},
		{"暴雪", "暴雪", "snow"},
		{"雨夹雪", "雨夹雪", "snow"},
		{"雾", "雾", "cloud"},
		{"霾", "霾", "cloud"},
		{"未知天气", "龙卷风", "unknown"},
		{"空字符串", "", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapWeatherToIcon(tt.weather); got != tt.want {
				t.Errorf("mapWeatherToIcon(%q) = %q, want %q", tt.weather, got, tt.want)
			}
		})
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name   string
		rawURL string
		apiKey string
		want   string
	}{
		{"替换URL中的Key", "https://restapi.amap.com/v3/ip?key=abc123&ip=1.2.3.4", "abc123", "https://restapi.amap.com/v3/ip?key=********&ip=1.2.3.4"},
		{"URL中无Key", "https://restapi.amap.com/v3/ip?ip=1.2.3.4", "abc123", "https://restapi.amap.com/v3/ip?ip=1.2.3.4"},
		{"空Key", "https://restapi.amap.com/v3/ip?key=abc123", "", "https://restapi.amap.com/v3/ip?key=abc123"},
		{"Key出现多次", "key=abc123&other=abc123", "abc123", "key=********&other=********"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskAPIKey(tt.rawURL, tt.apiKey); got != tt.want {
				t.Errorf("maskAPIKey(%q, %q) = %q, want %q", tt.rawURL, tt.apiKey, got, tt.want)
			}
		})
	}
}

func TestBuildDisplayCity(t *testing.T) {
	tests := []struct {
		name        string
		regeoCity   string
		regeoDist   string
		weatherCity string
		want        string
	}{
		{"市+区组合", "深圳市", "龙华区", "龙华区", "深圳市龙华区"},
		{"仅有市名", "深圳市", "", "深圳市", "深圳市"},
		{"直辖市-区名", "", "朝阳区", "朝阳区", "朝阳区"},
		{"全部为空", "", "", "", ""},
		{"市名与天气城市相同", "深圳市", "", "深圳市", "深圳市"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildDisplayCity(tt.regeoCity, tt.regeoDist, tt.weatherCity); got != tt.want {
				t.Errorf("buildDisplayCity(%q, %q, %q) = %q, want %q", tt.regeoCity, tt.regeoDist, tt.weatherCity, got, tt.want)
			}
		})
	}
}
