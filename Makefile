# CatTag 蓝牙通信库 Makefile
# 提供常用的构建、测试和开发命令

# 变量定义
APP_NAME := cattag
BUILD_DIR := build
CMD_DIR := cmd/app
VERSION := 1.0.0
BUILD_TIME := $(shell date '+%Y-%m-%d %H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
BUILD_USER := $(shell whoami)
BUILD_HOST := $(shell hostname)

# 构建标志
LDFLAGS := -X 'github.com/Anniext/CatTag/cmd/app/cmd.buildTime=$(BUILD_TIME)' \
           -X 'github.com/Anniext/CatTag/cmd/app/cmd.gitCommit=$(GIT_COMMIT)' \
           -X 'github.com/Anniext/CatTag/cmd/app/cmd.gitBranch=$(GIT_BRANCH)' \
           -X 'github.com/Anniext/CatTag/cmd/app/cmd.buildUser=$(BUILD_USER)' \
           -X 'github.com/Anniext/CatTag/cmd/app/cmd.buildHost=$(BUILD_HOST)'

.PHONY: all build test lint run clean install dev help

# 默认目标
all: clean lint test build

# 构建应用程序
build:
	@echo "构建 $(APP_NAME) v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) ./$(CMD_DIR)
	@echo "构建完成: $(BUILD_DIR)/$(APP_NAME)"

# 构建发布版本（优化）
build-release:
	@echo "构建发布版本 $(APP_NAME) v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS) -s -w" -o $(BUILD_DIR)/$(APP_NAME) ./$(CMD_DIR)
	@echo "发布版本构建完成: $(BUILD_DIR)/$(APP_NAME)"

# 运行测试
test:
	@echo "运行测试..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "生成覆盖率报告..."
	go tool cover -html=coverage.out -o coverage.html

# 运行基准测试
bench:
	@echo "运行基准测试..."
	go test -bench=. -benchmem ./...

# 代码检查
lint:
	@echo "运行代码检查..."
	golangci-lint run

# 格式化代码
fmt:
	@echo "格式化代码..."
	goimports -w .
	go fmt ./...

# 运行应用程序（开发模式）
run:
	@echo "运行应用程序（开发模式）..."
	go run ./$(CMD_DIR)

# 运行服务端
run-server:
	@echo "运行蓝牙服务端..."
	go run ./$(CMD_DIR) server --log-level=debug

# 运行客户端
run-client:
	@echo "运行蓝牙客户端..."
	go run ./$(CMD_DIR) client --log-level=debug

# 运行设备扫描
run-scan:
	@echo "扫描蓝牙设备..."
	go run ./$(CMD_DIR) scan --duration=5s --show-rssi

# 安装到系统
install: build
	@echo "安装 $(APP_NAME) 到系统..."
	sudo cp $(BUILD_DIR)/$(APP_NAME) /usr/local/bin/
	@echo "安装完成，可以使用 '$(APP_NAME)' 命令"

# 卸载
uninstall:
	@echo "卸载 $(APP_NAME)..."
	sudo rm -f /usr/local/bin/$(APP_NAME)
	@echo "卸载完成"

# 清理构建文件
clean:
	@echo "清理构建文件..."
	rm -rf $(BUILD_DIR)/
	rm -f coverage.out coverage.html
	@echo "清理完成"

# 更新依赖
deps:
	@echo "更新依赖..."
	go mod tidy
	go mod download

# 安全检查
security:
	@echo "运行安全检查..."
	govulncheck ./...

# 开发环境设置
dev: deps fmt lint test
	@echo "开发环境准备完成"

# 生成变更日志
changelog:
	@echo "生成变更日志..."
	git cliff --output CHANGELOG.md

# 显示帮助信息
help:
	@echo "CatTag 蓝牙通信库 - 可用命令:"
	@echo ""
	@echo "构建命令:"
	@echo "  build         - 构建应用程序"
	@echo "  build-release - 构建优化的发布版本"
	@echo "  install       - 安装到系统"
	@echo "  uninstall     - 从系统卸载"
	@echo ""
	@echo "开发命令:"
	@echo "  test          - 运行测试"
	@echo "  bench         - 运行基准测试"
	@echo "  lint          - 代码检查"
	@echo "  fmt           - 格式化代码"
	@echo "  security      - 安全检查"
	@echo "  dev           - 开发环境设置"
	@echo ""
	@echo "运行命令:"
	@echo "  run           - 运行应用程序"
	@echo "  run-server    - 运行服务端"
	@echo "  run-client    - 运行客户端"
	@echo "  run-scan      - 扫描设备"
	@echo ""
	@echo "维护命令:"
	@echo "  clean         - 清理构建文件"
	@echo "  deps          - 更新依赖"
	@echo "  changelog     - 生成变更日志"
	@echo "  help          - 显示此帮助信息"
