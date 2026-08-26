# Makefile: unify common tasks

SHELL := /bin/bash
.PHONY: build install start stop restart status test test-cover lint fmt vet clean \
	test-web test-website verify verify-e2e

build:
	./build.sh

install:
	./install.sh

start:
	./start.sh start

stop:
	./start.sh stop

restart:
	./start.sh restart

status:
	./start.sh status

# 运行全部单元测试（需要本地 Redis：redis-server；不可用时集成用例自动跳过）
test:
	go test ./... -count=1

# 后台控制台前端测试（vitest 单测 + locale 完整性守护）
test-web:
	cd web && npx vitest run

# 官网测试（locale 对齐 + sitemap 完整性守护）
test-website:
	cd ../prerender-offcial-website && npx vitest run

# 一键本地验证: 静态检查 + Go 全量 + 双前端测试
verify: lint test test-web test-website

# E2E 验证: 启动隔离实例 → API 43 端点全验证 → 浏览器全站巡检
verify-e2e:
	./scripts/verify_e2e.sh

# 运行测试并生成覆盖率报告 coverage.html
test-cover:
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# 静态检查
vet:
	go vet ./...

lint: vet
	@test -z "$$(gofmt -l . )" || (echo "以下文件未格式化，请运行 make fmt:" && gofmt -l . && exit 1)

fmt:
	gofmt -w .

clean:
	rm -f coverage*.out *.cover.out coverage.html controllers_coverage.html dump.rdb
