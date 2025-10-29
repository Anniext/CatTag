package bluetooth

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogAlertHandler 日志告警处理器
type LogAlertHandler struct {
	name   string
	logger *log.Logger
}

// FileAlertHandler 文件告警处理器
type FileAlertHandler struct {
	name     string
	filePath string
	mu       sync.Mutex
}

// CallbackAlertHandler 回调告警处理器
type CallbackAlertHandler struct {
	name     string
	callback func(Alert) error
}

// CompositeAlertHandler 复合告警处理器
type CompositeAlertHandler struct {
	name     string
	handlers []AlertHandler
}

// NewLogAlertHandler 创建日志告警处理器
func NewLogAlertHandler() *LogAlertHandler {
	return &LogAlertHandler{
		name:   "LogAlertHandler",
		logger: log.New(os.Stdout, "[BLUETOOTH-ALERT] ", log.LstdFlags),
	}
}

// Handle 处理告警
func (lah *LogAlertHandler) Handle(alert Alert) error {
	severity := lah.getAlertSeverityString(alert.Severity)
	message := fmt.Sprintf("[%s] %s - %s (Component: %s)",
		severity, alert.Timestamp.Format(time.RFC3339), alert.Message, alert.Component)

	lah.logger.Println(message)

	// 如果有元数据，也记录下来
	if alert.Metadata != nil {
		if metadataBytes, err := json.Marshal(alert.Metadata); err == nil {
			lah.logger.Printf("  Metadata: %s", string(metadataBytes))
		}
	}

	return nil
}

// getAlertSeverityString 获取告警严重程度字符串
func (lah *LogAlertHandler) getAlertSeverityString(severity AlertSeverity) string {
	switch severity {
	case AlertSeverityInfo:
		return "INFO"
	case AlertSeverityWarning:
		return "WARNING"
	case AlertSeverityError:
		return "ERROR"
	case AlertSeverityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// getAlertLevelString 获取告警级别字符串
func (lah *LogAlertHandler) getAlertLevelString(level AlertLevel) string {
	switch level {
	case AlertLevelInfo:
		return "INFO"
	case AlertLevelWarn:
		return "WARN"
	case AlertLevelError:
		return "ERROR"
	case AlertLevelCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// getAlertTypeString 获取告警类型字符串
func (lah *LogAlertHandler) getAlertTypeString(alertType AlertType) string {
	switch alertType {
	case AlertTypeConnectionLimit:
		return "CONNECTION_LIMIT"
	case AlertTypeHighLatency:
		return "HIGH_LATENCY"
	case AlertTypeHighErrorRate:
		return "HIGH_ERROR_RATE"
	case AlertTypeDeviceOffline:
		return "DEVICE_OFFLINE"
	case AlertTypeMemoryUsage:
		return "MEMORY_USAGE"
	case AlertTypeCPUUsage:
		return "CPU_USAGE"
	case AlertTypeConnectionFailed:
		return "CONNECTION_FAILED"
	default:
		return "UNKNOWN"
	}
}

// NewFileAlertHandler 创建文件告警处理器
func NewFileAlertHandler(filePath string) *FileAlertHandler {
	// 确保目录存在
	dir := filepath.Dir(filePath)
	os.MkdirAll(dir, 0755)

	return &FileAlertHandler{
		name:     "FileAlertHandler",
		filePath: filePath,
	}
}

// Handle 处理告警
func (fah *FileAlertHandler) Handle(alert Alert) error {
	fah.mu.Lock()
	defer fah.mu.Unlock()

	// 打开文件（追加模式）
	file, err := os.OpenFile(fah.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("无法打开告警文件: %w", err)
	}
	defer file.Close()

	// 序列化告警为JSON
	alertData, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("无法序列化告警数据: %w", err)
	}

	// 写入文件
	_, err = file.WriteString(string(alertData) + "\n")
	if err != nil {
		return fmt.Errorf("无法写入告警文件: %w", err)
	}

	return nil
}

// NewCallbackAlertHandler 创建回调告警处理器
func NewCallbackAlertHandler(callback func(Alert) error) *CallbackAlertHandler {
	return &CallbackAlertHandler{
		name:     "CallbackAlertHandler",
		callback: callback,
	}
}

// Handle 处理告警
func (cah *CallbackAlertHandler) Handle(alert Alert) error {
	if cah.callback != nil {
		return cah.callback(alert)
	}
	return nil
}

// NewCompositeAlertHandler 创建复合告警处理器
func NewCompositeAlertHandler(handlers ...AlertHandler) *CompositeAlertHandler {
	return &CompositeAlertHandler{
		name:     "CompositeAlertHandler",
		handlers: handlers,
	}
}

// Handle 处理告警
func (cah *CompositeAlertHandler) Handle(alert Alert) error {
	var errors []error

	// 调用所有子处理器
	for _, handler := range cah.handlers {
		if err := handler.Handle(alert); err != nil {
			errors = append(errors, fmt.Errorf("handler error: %w", err))
		}
	}

	// 如果有错误，返回组合错误
	if len(errors) > 0 {
		var errorMsg string
		for i, err := range errors {
			if i > 0 {
				errorMsg += "; "
			}
			errorMsg += err.Error()
		}
		return fmt.Errorf("复合处理器错误: %s", errorMsg)
	}

	return nil
}

// AddHandler 添加子处理器
func (cah *CompositeAlertHandler) AddHandler(handler AlertHandler) {
	cah.handlers = append(cah.handlers, handler)
}

// RemoveHandler 移除子处理器
func (cah *CompositeAlertHandler) RemoveHandler(index int) {
	if index >= 0 && index < len(cah.handlers) {
		cah.handlers = append(cah.handlers[:index], cah.handlers[index+1:]...)
	}
}

// EmailAlertHandler 邮件告警处理器（示例实现）
type EmailAlertHandler struct {
	name       string
	smtpServer string
	smtpPort   int
	username   string
	password   string
	recipients []string
}

// NewEmailAlertHandler 创建邮件告警处理器
func NewEmailAlertHandler(smtpServer string, smtpPort int, username, password string, recipients []string) *EmailAlertHandler {
	return &EmailAlertHandler{
		name:       "EmailAlertHandler",
		smtpServer: smtpServer,
		smtpPort:   smtpPort,
		username:   username,
		password:   password,
		recipients: recipients,
	}
}

// Handle 处理告警（发送邮件）
func (eah *EmailAlertHandler) Handle(alert Alert) error {
	// 这里是一个简化的邮件发送实现
	// 在实际应用中，需要使用SMTP库来发送邮件

	subject := fmt.Sprintf("蓝牙连接池告警: %s", eah.getAlertSeverityString(alert.Severity))
	body := fmt.Sprintf(`
告警详情:
- 规则: %s
- 严重程度: %s
- 消息: %s
- 组件: %s
- 时间: %s

元数据: %v
`, alert.Rule.Name, eah.getAlertSeverityString(alert.Severity),
		alert.Message, alert.Component, alert.Timestamp.Format(time.RFC3339), alert.Metadata)

	// 模拟发送邮件
	log.Printf("发送邮件告警 - 主题: %s, 收件人: %v", subject, eah.recipients)
	log.Printf("邮件内容: %s", body)

	return nil
}

// getAlertSeverityString 获取告警严重程度字符串
func (eah *EmailAlertHandler) getAlertSeverityString(severity AlertSeverity) string {
	switch severity {
	case AlertSeverityInfo:
		return "信息"
	case AlertSeverityWarning:
		return "警告"
	case AlertSeverityError:
		return "错误"
	case AlertSeverityCritical:
		return "严重"
	default:
		return "未知"
	}
}

// getAlertTypeString 获取告警类型字符串
func (eah *EmailAlertHandler) getAlertTypeString(alertType AlertType) string {
	switch alertType {
	case AlertTypeConnectionLimit:
		return "连接数限制"
	case AlertTypeHighLatency:
		return "高延迟"
	case AlertTypeHighErrorRate:
		return "高错误率"
	case AlertTypeDeviceOffline:
		return "设备离线"
	case AlertTypeMemoryUsage:
		return "内存使用"
	case AlertTypeCPUUsage:
		return "CPU使用"
	case AlertTypeConnectionFailed:
		return "连接失败"
	default:
		return "未知类型"
	}
}

// WebhookAlertHandler Webhook告警处理器（示例实现）
type WebhookAlertHandler struct {
	name string
	url  string
}

// NewWebhookAlertHandler 创建Webhook告警处理器
func NewWebhookAlertHandler(url string) *WebhookAlertHandler {
	return &WebhookAlertHandler{
		name: "WebhookAlertHandler",
		url:  url,
	}
}

// Handle 处理告警（发送Webhook）
func (wah *WebhookAlertHandler) Handle(alert Alert) error {
	// 这里是一个简化的Webhook发送实现
	// 在实际应用中，需要使用HTTP客户端发送POST请求

	alertData, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("无法序列化告警数据: %w", err)
	}

	// 模拟发送Webhook
	log.Printf("发送Webhook告警到 %s: %s", wah.url, string(alertData))

	return nil
}

// ThrottledAlertHandler 限流告警处理器
type ThrottledAlertHandler struct {
	name           string
	handler        AlertHandler
	throttleWindow time.Duration
	maxAlerts      int
	mu             sync.Mutex
	alertCounts    map[string][]time.Time // 按告警类型记录时间戳
}

// NewThrottledAlertHandler 创建限流告警处理器
func NewThrottledAlertHandler(handler AlertHandler, throttleWindow time.Duration, maxAlerts int) *ThrottledAlertHandler {
	return &ThrottledAlertHandler{
		name:           "ThrottledAlertHandler",
		handler:        handler,
		throttleWindow: throttleWindow,
		maxAlerts:      maxAlerts,
		alertCounts:    make(map[string][]time.Time),
	}
}

// Handle 处理告警（带限流）
func (tah *ThrottledAlertHandler) Handle(alert Alert) error {
	tah.mu.Lock()
	defer tah.mu.Unlock()

	alertKey := fmt.Sprintf("%s_%s", alert.Rule.Name, alert.Component)
	now := time.Now()

	// 清理过期的时间戳
	tah.cleanupOldTimestamps(alertKey, now)

	// 检查是否超过限制
	if len(tah.alertCounts[alertKey]) >= tah.maxAlerts {
		// 超过限制，丢弃告警
		log.Printf("告警被限流丢弃: %s (规则: %s, 组件: %s)", alert.Message, alert.Rule.Name, alert.Component)
		return nil
	}

	// 记录时间戳
	tah.alertCounts[alertKey] = append(tah.alertCounts[alertKey], now)

	// 调用实际处理器
	return tah.handler.Handle(alert)
}

// cleanupOldTimestamps 清理过期的时间戳
func (tah *ThrottledAlertHandler) cleanupOldTimestamps(alertKey string, now time.Time) {
	timestamps := tah.alertCounts[alertKey]
	cutoff := now.Add(-tah.throttleWindow)

	var validTimestamps []time.Time
	for _, timestamp := range timestamps {
		if timestamp.After(cutoff) {
			validTimestamps = append(validTimestamps, timestamp)
		}
	}

	tah.alertCounts[alertKey] = validTimestamps
}

// getAlertTypeString 获取告警类型字符串
func (tah *ThrottledAlertHandler) getAlertTypeString(alertType AlertType) string {
	switch alertType {
	case AlertTypeConnectionLimit:
		return "CONNECTION_LIMIT"
	case AlertTypeHighLatency:
		return "HIGH_LATENCY"
	case AlertTypeHighErrorRate:
		return "HIGH_ERROR_RATE"
	case AlertTypeDeviceOffline:
		return "DEVICE_OFFLINE"
	case AlertTypeMemoryUsage:
		return "MEMORY_USAGE"
	case AlertTypeCPUUsage:
		return "CPU_USAGE"
	case AlertTypeConnectionFailed:
		return "CONNECTION_FAILED"
	default:
		return "UNKNOWN"
	}
}
