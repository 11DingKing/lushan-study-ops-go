# BENZHI_README

这是一个基于 Go 实现的后端服务，用于承载 lushan-study-ops-go 的业务处理、数据管理与稳定运行。

## 项目说明

- 项目：11DingKing/lushan-study-ops-go
- 项目用途：Lushan Study Operations is a Go backend for delivering cross-venue study tours around Lushan. It coordinates school and family cohorts through application, plan composition, venue and mentor holds, risk acknowledgement, confirmation, attendance grouping, weather rerouting, artifact archival, cancellation settlement, and durable follow-up work.
- Go 工具链：`golang:1.25.0`
- 前端工具链：无

## 标准构建、运行和测试命令

进入容器后执行：

```bash
# 编译
cd '/app' && GOTOOLCHAIN=local go build ./...

# 启动
cd '/app' && GOTOOLCHAIN=local go run ./cmd/server

# 测试
cd '/app' && GOTOOLCHAIN=local go test ./...
```

## Docker 构建和进入容器

```bash
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-task-27-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-task-27-arm64 linux/arm64
docker run -it benzhi-task-27-amd64:latest
docker run -it --platform linux/arm64 benzhi-task-27-arm64:latest
```

## 题目验证命令

1. 预期退出码 1：`go test ./internal/worker -run '^TestFreshWorkerJobWaitsForStaleWindow$' -count=1`
