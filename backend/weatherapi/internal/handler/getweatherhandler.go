// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package handler

import (
	"net"
	"net/http"
	"strings"

	"weatherapi/internal/logic"
	"weatherapi/internal/svc"
	"weatherapi/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// GetWeatherHandler 返回一个 http.HandlerFunc，解析天气请求参数，
// 当未提供 IP 时自动提取客户端 IP，并委托给天气业务逻辑处理。
func GetWeatherHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.WeatherRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		if req.IP == "" {
			req.IP = getClientIP(r)
		}

		l := logic.NewGetWeatherLogic(r.Context(), svcCtx)
		resp, err := l.GetWeather(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

// getClientIP 从请求中提取客户端 IP 地址，依次检查
// X-Forwarded-For、X-Real-IP 请求头，最后使用 RemoteAddr。
// 正确处理 IPv4 和 IPv6 地址。
func getClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		if idx := strings.Index(ip, ","); idx != -1 {
			return strings.TrimSpace(ip[:idx])
		}
		return strings.TrimSpace(ip)
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return strings.TrimSpace(host)
}
