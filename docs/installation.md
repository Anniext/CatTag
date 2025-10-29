# 安装指南

本指南将帮助您安装和配置 CatTag 蓝牙通信库。

## 系统要求

### 软件要求

- **Go 版本**: 1.25.1 或更高版本
- **操作系统**: Windows 10+, Linux (Ubuntu 18.04+), macOS 10.15+
- **蓝牙支持**: 蓝牙 4.0+ 适配器

### 硬件要求

- 蓝牙适配器（内置或外置）
- 至少 512MB 可用内存
- 至少 100MB 可用磁盘空间

## 安装步骤

### 1. 安装 Go

如果您还没有安装 Go 1.25.1，请从 [官方网站](https://golang.org/dl/) 下载并安装。

验证 Go 安装：

```bash
go version
```

### 2. 创建项目

```bash
mkdir my-bluetooth-app
cd my-bluetooth-app
go mod init my-bluetooth-app
```

### 3. 安装 CatTag 库

```bash
go get github.com/Anniext/CatTag
```

### 4. 验证安装

创建一个简单的测试文件 `main.go`：

```go
package main

import (
    "fmt"
    "github.com/Anniext/CatTag/pkg/bluetooth"
)

func main() {
    config := bluetooth.DefaultConfig()
    fmt.Printf("CatTag 库安装成功！服务UUID: %s\n", config.ServerConfig.ServiceUUID)
}
```

运行测试：

```bash
go run main.go
```

## 平台特定配置

### Windows

1. 确保蓝牙服务已启动
2. 可能需要管理员权限运行应用
3. 安装 Microsoft Visual C++ Redistributable

### Linux

1. 安装蓝牙开发库：

```bash
# Ubuntu/Debian
sudo apt-get install libbluetooth-dev

# CentOS/RHEL
sudo yum install bluez-libs-devel
```

2. 确保用户在 `bluetooth` 组中：

```bash
sudo usermod -a -G bluetooth $USER
```

### macOS

1. 确保蓝牙已启用
2. 可能需要在"系统偏好设置"中授权蓝牙访问权限

## 下一步

安装完成后，请查看：

- [快速入门指南](quickstart.md)
- [基础概念](concepts.md)
- [示例应用](../examples/README.md)
