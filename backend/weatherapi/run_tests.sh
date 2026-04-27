#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo "========================================="
echo "  天气服务单元测试执行脚本"
echo "========================================="

echo ""
echo "[1/4] 运行单元测试（含竞态检测）..."
go test -race -count=1 -v ./internal/logic/ ./internal/handler/ ./internal/svc/

echo ""
echo "[2/4] 生成覆盖率数据..."
go test -race -coverprofile=coverage.out ./internal/logic/ ./internal/handler/ ./internal/svc/

echo ""
echo "[3/4] 覆盖率报告（按函数）："
go tool cover -func=coverage.out

echo ""
echo "[4/4] 生成 HTML 覆盖率报告..."
go tool cover -html=coverage.out -o coverage.html
echo "HTML 覆盖率报告已生成: coverage.html"

echo ""
echo "========================================="
echo "  测试完成！"
echo "========================================="
