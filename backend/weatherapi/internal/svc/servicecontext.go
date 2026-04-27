package svc

import (
	"fmt"

	"weatherapi/internal/config"

	ip2region "github.com/lionsoul2014/ip2region/binding/golang/service"
	"github.com/zeromicro/go-zero/core/logx"
)

// IP2RegionSearcher 定义 IP 离线查询接口，用于解耦 ip2region 具体实现，方便测试注入。
type IP2RegionSearcher interface {
	Search(ip any) (string, error)
	Close()
}

// ip2RegionAdapter 适配 *ip2region.Ip2Region 到 IP2RegionSearcher 接口。
type ip2RegionAdapter struct {
	*ip2region.Ip2Region
}

// ServiceContext 持有天气服务的共享依赖，包括配置和 IP 离线查询引擎。
type ServiceContext struct {
	Config    config.Config
	IP2Region IP2RegionSearcher
}

// NewServiceContext 根据给定配置创建 ServiceContext。
// 当数据库路径已配置时，初始化 ip2region 服务用于离线 IP 定位。
// 缺失配置时会记录警告日志。
func NewServiceContext(c config.Config) *ServiceContext {
	svc := &ServiceContext{
		Config: c,
	}

	var v4Config *ip2region.Config
	var v6Config *ip2region.Config
	var err error

	if c.IP2RegionV4DBPath != "" {
		v4Config, err = ip2region.NewV4Config(ip2region.BufferCache, c.IP2RegionV4DBPath, 1)
		if err != nil {
			logx.Severef("创建 ip2region v4 配置失败: %v", err)
		}
	} else {
		logx.Slowf("IP2REGION_V4_DB_PATH 未配置，IPv4 离线查询已禁用")
	}

	if c.IP2RegionV6DBPath != "" {
		v6Config, err = ip2region.NewV6Config(ip2region.BufferCache, c.IP2RegionV6DBPath, 1)
		if err != nil {
			logx.Severef("创建 ip2region v6 配置失败: %v", err)
		}
	} else {
		logx.Slowf("IP2REGION_V6_DB_PATH 未配置，IPv6 离线查询已禁用")
	}

	if v4Config != nil || v6Config != nil {
		region, err := ip2region.NewIp2Region(v4Config, v6Config)
		if err != nil {
			logx.Severef("初始化 ip2region 服务失败: %v", err)
		} else {
			svc.IP2Region = &ip2RegionAdapter{Ip2Region: region}
			fmt.Println("ip2region 服务初始化成功 (BufferCache 模式)")
		}
	}

	return svc
}

// Close 释放 ServiceContext 持有的资源，包括 ip2region 引擎。
func (svc *ServiceContext) Close() {
	if svc.IP2Region != nil {
		svc.IP2Region.Close()
		fmt.Println("ip2region 服务已关闭")
	}
}
