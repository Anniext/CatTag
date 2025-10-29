# 技术栈和构建系统

## 技术栈

### 核心技术

- **Go 1.25.1**: 使用最新版本的 Go，充分利用泛型和并发优化特性
- **蓝牙协议**: 支持 RFCOMM 和 L2CAP 协议栈
- **加密**: AES-256-GCM 加密算法

### 开发工具链

- **golangci-lint**: 代码质量检查，配置了严格的代码规范
- **goimports**: 自动导入管理和代码格式化
- **govulncheck**: 安全漏洞检查
- **pre-commit**: 提交前代码检查
- **typos**: 拼写检查
- **git-cliff**: 自动生成变更日志

## 构建系统

### Makefile 命令

```bash
# 构建项目
make build

# 运行测试（包含竞态检测和覆盖率）
make test

# 代码检查
make lint

# 运行应用
make run

# 清理构建文件
make clean
```

### 常用开发命令

```bash
# 安装依赖
go mod tidy

# 运行特定测试
go test -v ./pkg/...

# 生成测试覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# 代码格式化
goimports -w .

# 安全检查
govulncheck ./...

# 生成变更日志
git cliff --output CHANGELOG.md
```

## 代码质量标准

### 代码规范

- 函数长度限制：80 行代码，50 条语句
- 行长度限制：120 字符
- 圈复杂度限制：15
- 认知复杂度检查
- 强制错误处理检查

### 测试要求

- 单元测试覆盖率要求
- 竞态条件检测
- 性能基准测试
- 集成测试和端到端测试

### 安全要求

- 所有外部输入必须验证
- 敏感数据加密存储
- 定期安全漏洞扫描
