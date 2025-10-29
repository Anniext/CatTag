package bluetooth

import (
	"log/slog"
	"sync"
	"time"
)

// NewLogManager 创建新的日志管理器
func NewLogManager(logger *slog.Logger) *LogManager {
	return &LogManager{
		logger:         logger,
		logLevel:       slog.LevelInfo,
		logHandlers:    make([]LogHandler, 0),
		structuredLogs: true,
		logBuffer:      make([]LogEntry, 0),
		bufferSize:     1000,
	}
}

// SetLogLevel 设置日志级别
func (lm *LogManager) SetLogLevel(level slog.Level) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.logLevel = level
}

// AddHandler 添加日志处理器
func (lm *LogManager) AddHandler(handler LogHandler) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.logHandlers = append(lm.logHandlers, handler)
}

// EnableStructuredLogs 启用结构化日志
func (lm *LogManager) EnableStructuredLogs(enable bool) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.structuredLogs = enable
}

// LogInfo 记录信息日志
func (lm *LogManager) LogInfo(component, message string, fields map[string]interface{}) {
	lm.log(slog.LevelInfo, component, message, fields, nil)
}

// LogWarn 记录警告日志
func (lm *LogManager) LogWarn(component, message string, fields map[string]interface{}) {
	lm.log(slog.LevelWarn, component, message, fields, nil)
}

// LogError 记录错误日志
func (lm *LogManager) LogError(component, message string, err error, fields map[string]interface{}) {
	lm.log(slog.LevelError, component, message, fields, err)
}

// LogDebug 记录调试日志
func (lm *LogManager) LogDebug(component, message string, fields map[string]interface{}) {
	lm.log(slog.LevelDebug, component, message, fields, nil)
}

// log 内部日志记录方法
func (lm *LogManager) log(level slog.Level, component, message string, fields map[string]interface{}, err error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// 检查日志级别
	if level < lm.logLevel {
		return
	}

	// 创建日志条目
	entry := LogEntry{
		Level:     level,
		Message:   message,
		Component: component,
		Timestamp: time.Now(),
		Fields:    fields,
		Error:     err,
	}

	// 添加到缓冲区
	lm.addToBuffer(entry)

	// 使用标准日志记录器
	lm.logWithSlog(entry)

	// 调用自定义处理器
	lm.callHandlers(entry)
}

// logWithSlog 使用slog记录日志
func (lm *LogManager) logWithSlog(entry LogEntry) {
	attrs := make([]slog.Attr, 0)
	attrs = append(attrs, slog.String("component", entry.Component))

	// 添加字段
	if entry.Fields != nil {
		for key, value := range entry.Fields {
			attrs = append(attrs, slog.Any(key, value))
		}
	}

	// 添加错误信息
	if entry.Error != nil {
		attrs = append(attrs, slog.String("error", entry.Error.Error()))
	}

	// 记录日志
	lm.logger.LogAttrs(nil, entry.Level, entry.Message, attrs...)
}

// callHandlers 调用自定义处理器
func (lm *LogManager) callHandlers(entry LogEntry) {
	for _, handler := range lm.logHandlers {
		go func(h LogHandler) {
			if err := h.Handle(entry); err != nil {
				// 处理器错误，使用标准日志记录
				lm.logger.Error("日志处理器错误", "error", err)
			}
		}(handler)
	}
}

// addToBuffer 添加到缓冲区
func (lm *LogManager) addToBuffer(entry LogEntry) {
	// 检查缓冲区大小
	if len(lm.logBuffer) >= lm.bufferSize {
		// 移除最旧的条目
		lm.logBuffer = lm.logBuffer[1:]
	}

	lm.logBuffer = append(lm.logBuffer, entry)
}

// GetLogBuffer 获取日志缓冲区
func (lm *LogManager) GetLogBuffer() []LogEntry {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	// 返回副本
	buffer := make([]LogEntry, len(lm.logBuffer))
	copy(buffer, lm.logBuffer)
	return buffer
}

// ClearBuffer 清空缓冲区
func (lm *LogManager) ClearBuffer() {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.logBuffer = make([]LogEntry, 0)
}

// GetLogStatistics 获取日志统计
func (lm *LogManager) GetLogStatistics() LogStatistics {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	stats := LogStatistics{
		TotalLogs:       len(lm.logBuffer),
		LogsByLevel:     make(map[slog.Level]int),
		LogsByComponent: make(map[string]int),
	}

	for _, entry := range lm.logBuffer {
		stats.LogsByLevel[entry.Level]++
		stats.LogsByComponent[entry.Component]++
	}

	return stats
}

// LogStatistics 日志统计信息
type LogStatistics struct {
	TotalLogs       int                `json:"total_logs"`        // 总日志数
	LogsByLevel     map[slog.Level]int `json:"logs_by_level"`     // 按级别分组
	LogsByComponent map[string]int     `json:"logs_by_component"` // 按组件分组
}

// FileLogHandler 文件日志处理器
type FileLogHandler struct {
	filePath string
	mu       sync.Mutex
}

// NewFileLogHandler 创建文件日志处理器
func NewFileLogHandler(filePath string) *FileLogHandler {
	return &FileLogHandler{
		filePath: filePath,
	}
}

// Handle 处理日志条目
func (flh *FileLogHandler) Handle(entry LogEntry) error {
	flh.mu.Lock()
	defer flh.mu.Unlock()

	// 简化的文件写入逻辑
	// 实际实现中需要更完善的文件操作
	return nil
}

// ConsoleLogHandler 控制台日志处理器
type ConsoleLogHandler struct{}

// NewConsoleLogHandler 创建控制台日志处理器
func NewConsoleLogHandler() *ConsoleLogHandler {
	return &ConsoleLogHandler{}
}

// Handle 处理日志条目
func (clh *ConsoleLogHandler) Handle(entry LogEntry) error {
	// 简化的控制台输出逻辑
	// 实际实现中可以格式化输出
	return nil
}
