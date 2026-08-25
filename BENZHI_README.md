# BENZHI_README

基于 Go 实现的口述史公开授权工作台 Web 项目，一款后端服务，已完整实现面向地方文化机构的口述史公开授权工作台，覆盖知情同意、逐段敏感性标注、可追溯脱敏、伦理退回闭环、公开清单冻结、递增凭据签发、审计时间线与现场摘要校验，并以内置浏览器页面和可自行结束的真实 HTTP 自检提供可达运行链路。

## 项目说明
- 项目：benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d
- 项目用途：已完整实现面向地方文化机构的口述史公开授权工作台，覆盖知情同意、逐段敏感性标注、可追溯脱敏、伦理退回闭环、公开清单冻结、递增凭据签发、审计时间线与现场摘要校验，并以内置浏览器页面和可自行结束的真实 HTTP 自检提供可达运行链路。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d-arm64 linux/arm64
docker run -it benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck`
