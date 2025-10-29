# CatTag 蓝牙通信库文档

欢迎使用 CatTag 蓝牙通信库！这是一个用 Go 语言开发的现代化蓝牙通信解决方案，充分利用 Go 1.25 的最新特性，提供高性能、类型安全的蓝牙设备通信能力。

## 文档目录

### 快速开始

- [安装指南](installation.md) - 如何安装和配置 CatTag 库
- [快速入门](quickstart.md) - 5 分钟上手指南
- [基础概念](concepts.md) - 核心概念和架构介绍

### API 文档

- [核心接口](api/core-interfaces.md) - 主要接口和类型定义
- [蓝牙组件](api/bluetooth-component.md) - BluetoothComponent 详细文档
- [服务端 API](api/server-api.md) - BluetoothServer 接口文档
- [客户端 API](api/client-api.md) - BluetoothClient 接口文档
- [设备管理](api/device-manager.md) - DeviceManager 接口文档
- [连接池](api/connection-pool.md) - ConnectionPool 接口文档
- [健康检查](api/health-checker.md) - HealthChecker 接口文档
- [安全管理](api/security-manager.md) - SecurityManager 接口文档
- [消息处理](api/message-handler.md) - MessageHandler 接口文档

### 使用指南

- [配置管理](guides/configuration.md) - 配置系统使用指南
- [构建器模式](guides/builder-pattern.md) - 组件构建器使用指南
- [错误处理](guides/error-handling.md) - 错误处理最佳实践
- [性能优化](guides/performance.md) - 性能优化指南
- [安全最佳实践](guides/security.md) - 安全配置和最佳实践
- [测试指南](guides/testing.md) - 单元测试和集成测试

### 示例和教程

- [示例应用](examples/README.md) - 完整示例应用
- [代码片段](examples/code-snippets.md) - 常用代码片段
- [最佳实践](examples/best-practices.md) - 开发最佳实践

### 故障排除

- [常见问题](troubleshooting/faq.md) - 常见问题解答
- [故障诊断](troubleshooting/diagnostics.md) - 问题诊断指南
- [性能调优](troubleshooting/performance-tuning.md) - 性能问题解决

### 高级主题

- [架构设计](advanced/architecture.md) - 详细架构设计
- [扩展开发](advanced/extensions.md) - 扩展和插件开发
- [内部实现](advanced/internals.md) - 内部实现细节

## 版本信息

- **当前版本**: 1.0.0
- **Go 版本要求**: 1.25.1+
- **支持平台**: Windows, Linux, macOS
- **蓝牙协议**: RFCOMM, L2CAP

## 快速链接

- [GitHub 仓库](https://github.com/Anniext/CatTag)
- [问题反馈](https://github.com/Anniext/CatTag/issues)
- [贡献指南](../CONTRIBUTING.md)
- [更新日志](../CHANGELOG.md)

## 获取帮助

如果您在使用过程中遇到问题，可以通过以下方式获取帮助：

1. 查阅本文档的相关章节
2. 查看 [常见问题](troubleshooting/faq.md)
3. 在 GitHub 上提交 [Issue](https://github.com/Anniext/CatTag/issues)
4. 参考 [示例应用](../examples/) 中的代码

## 贡献

我们欢迎社区贡献！请参阅 [贡献指南](../CONTRIBUTING.md) 了解如何参与项目开发。

## 许可证

本项目采用 MIT 许可证，详情请参阅 [LICENSE](../LICENSE) 文件。
