package svc

import (
	"testing"

	"weatherapi/internal/config"
)

func TestNewServiceContext_NoDBPaths(t *testing.T) {
	c := config.Config{
		AmapAPIKey: "test-key",
	}

	svcCtx := NewServiceContext(c)

	if svcCtx == nil {
		t.Fatal("NewServiceContext 返回 nil")
	}
	if svcCtx.Config.AmapAPIKey != "test-key" {
		t.Errorf("Config.AmapAPIKey = %q, want %q", svcCtx.Config.AmapAPIKey, "test-key")
	}
	if svcCtx.IP2Region != nil {
		t.Error("IP2Region 应为 nil（未配置数据库路径）")
	}
}

func TestNewServiceContext_InvalidDBPath(t *testing.T) {
	c := config.Config{
		AmapAPIKey:        "test-key",
		IP2RegionV4DBPath: "/nonexistent/path/ip2region.xdb",
	}

	svcCtx := NewServiceContext(c)

	if svcCtx == nil {
		t.Fatal("NewServiceContext 返回 nil")
	}
	if svcCtx.IP2Region != nil {
		t.Error("IP2Region 应为 nil（数据库路径无效）")
	}
}

func TestNewServiceContext_EmptyConfig(t *testing.T) {
	c := config.Config{}

	svcCtx := NewServiceContext(c)

	if svcCtx == nil {
		t.Fatal("NewServiceContext 返回 nil")
	}
	if svcCtx.IP2Region != nil {
		t.Error("IP2Region 应为 nil（空配置）")
	}
}

func TestServiceContext_Close_NilIP2Region(t *testing.T) {
	svcCtx := &ServiceContext{
		Config: config.Config{AmapAPIKey: "test-key"},
	}

	svcCtx.Close()
}

type mockIP2RegionSearcher struct {
	closed bool
}

func (m *mockIP2RegionSearcher) Search(ip any) (string, error) {
	return "", nil
}

func (m *mockIP2RegionSearcher) Close() {
	m.closed = true
}

func TestServiceContext_Close_WithIP2Region(t *testing.T) {
	mock := &mockIP2RegionSearcher{}
	svcCtx := &ServiceContext{
		Config:    config.Config{AmapAPIKey: "test-key"},
		IP2Region: mock,
	}

	svcCtx.Close()

	if !mock.closed {
		t.Error("Close 应调用 IP2Region.Close()")
	}
}
