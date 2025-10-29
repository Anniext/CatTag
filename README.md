# CatTag - Go 蓝牙通信库

CatTag 是一个用 Go 语言开发的现代化蓝牙通信库，专注于提供高性能、类型安全的蓝牙设备通信解决方案。

## 核心功能

- **蓝牙设备管理**: 设备发现、连接管理、状态监控
- **双向通信**: 支持服务端和客户端模式的蓝牙通信
- **消息处理**: 泛型消息处理，支持自定义消息类型
- **连接池管理**: 高效的多连接管理和负载均衡
- **安全通信**: AES-256-GCM 加密和设备认证
- **健康监控**: 连接质量监控和自动故障恢复

## 技术特色

- 利用 Go 1.25 的最新特性，包括泛型和并发优化
- 支持 RFCOMM 和 L2CAP 协议
- 提供构建器模式的配置管理
- 完整的错误处理和重试机制
- 高性能并发消息处理

## 项目结构

```
CatTag/
├── cmd/                    # 应用程序入口点
│   └── app/               # 主应用程序
├── internal/              # 内部包（不对外暴露）
│   ├── adapter/           # 蓝牙适配器实现
│   ├── device/            # 设备管理
│   ├── protocol/          # 协议处理
│   ├── security/          # 安全管理
│   └── config/            # 配置管理
├── pkg/                   # 公共库（可被外部引用）
│   └── bluetooth/         # 蓝牙核心接口和类型
├── examples/              # 示例应用
├── docs/                  # 文档
└── test/                  # 集成测试
```

## 快速开始

### 构建项目

```bash
# 构建项目
make build

# 运行测试
make test

# 代码检查
make lint
```

### 运行应用

```bash
# 运行服务端模式
./CatTag -mode=server

# 运行客户端模式
./CatTag -mode=client

# 同时运行服务端和客户端
./CatTag -mode=both

# 查看帮助
./CatTag -help
```

## 开发状态

当前项目处于开发阶段，已完成：

✅ 项目结构和核心接口定义  
✅ 错误处理体系和常量定义  
✅ 配置管理系统  
✅ 基础测试框架

正在开发中：

- 蓝牙适配器实现
- 设备管理功能
- 消息处理系统
- 连接池管理
- 安全功能

## 开发环境设置

### 安装 Go

登录 Go 官网下载 go 二进制包`https://go.dev/doc/install`

```bash
rm -rf /usr/local/go && tar -C /usr/local -xzf go1.25.3.linux-amd64.tar.gz
```

### 安装开发工具

```bash
# 安装 goimports
go install golang.org/x/tools/cmd/goimports@latest

# 安装 golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 安装 govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest

# 安装 pre-commit
pipx install pre-commit

# 安装 typos
cargo install typos-cli

# 安装 git cliff
cargo install git-cliff
```

安装成功后运行 `pre-commit install` 即可启用提交前检查。

### 生成变更日志

```bash
git cliff --output CHANGELOG.md
```

## 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。
