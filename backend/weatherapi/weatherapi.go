// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"flag"
	"fmt"
	"os"
	"slices"

	"weatherapi/internal/config"
	"weatherapi/internal/handler"
	"weatherapi/internal/svc"

	"github.com/joho/godotenv"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/weatherapi-api.yaml", "the config file")

func main() {
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		logx.Slowf("加载 .env 文件失败: %v", err)
	}

	var c config.Config
	conf.MustLoad(*configFile, &c)

	if c.Environment == "" {
		logx.Severef("ENV 环境变量未设置，必须为以下值之一: dev, sit, prod")
		os.Exit(1)
	}

	validEnvs := []string{"dev", "sit", "prod"}
	if !slices.Contains(validEnvs, c.Environment) {
		logx.Severef("无效的 ENV 值 %q，必须为以下值之一: dev, sit, prod", c.Environment)
		os.Exit(1)
	}

	var opts []rest.RunOption
	switch c.Environment {
	case "dev":
		opts = append(opts, rest.WithCors("*"))
	default:
		if len(c.CORSAllowOrigins) > 0 {
			opts = append(opts, rest.WithCors(c.CORSAllowOrigins...))
		}
	}

	server := rest.MustNewServer(c.RestConf, opts...)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	defer ctx.Close()
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d (env: %s)...\n", c.Host, c.Port, c.Environment)
	server.Start()
}
