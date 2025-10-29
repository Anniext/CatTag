package bluetooth

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strconv"
	"strings"
	"time"
)

// JSONReportGenerator JSON格式报告生成器
type JSONReportGenerator struct {
	name string
}

// CSVReportGenerator CSV格式报告生成器
type CSVReportGenerator struct {
	name string
}

// HTMLReportGenerator HTML格式报告生成器
type HTMLReportGenerator struct {
	name     string
	template *template.Template
}

// TextReportGenerator 文本格式报告生成器
type TextReportGenerator struct {
	name string
}

// NewJSONReportGenerator 创建JSON报告生成器
func NewJSONReportGenerator() *JSONReportGenerator {
	return &JSONReportGenerator{
		name: "JSONReportGenerator",
	}
}

// GenerateReport 生成JSON格式报告
func (jrg *JSONReportGenerator) GenerateReport(metrics *PoolMetrics) ([]byte, error) {
	// 创建报告结构
	report := map[string]interface{}{
		"report_type":    "bluetooth_pool_metrics",
		"generated_at":   time.Now().Format(time.RFC3339),
		"report_version": "1.0",
		"summary": map[string]interface{}{
			"active_connections":      metrics.ActiveConnections,
			"total_connections":       metrics.TotalConnections,
			"peak_connections":        metrics.PeakConnections,
			"total_bytes_transferred": metrics.TotalBytesTransferred,
			"total_messages":          metrics.TotalMessages,
			"average_latency_ms":      metrics.AverageLatency.Milliseconds(),
			"error_rate_percent":      metrics.ErrorRate * 100,
			"throughput_msg_per_sec":  metrics.Throughput,
		},
		"connections_by_priority": metrics.ConnectionsByPriority,
		"device_metrics":          metrics.ConnectionsByDevice,
		"performance_history":     metrics.PerformanceHistory,
		"alerts":                  metrics.Alerts,
		"uptime_seconds":          time.Since(metrics.StartTime).Seconds(),
	}

	return json.MarshalIndent(report, "", "  ")
}

// GetReportFormat 获取报告格式
func (jrg *JSONReportGenerator) GetReportFormat() string {
	return "json"
}

// NewCSVReportGenerator 创建CSV报告生成器
func NewCSVReportGenerator() *CSVReportGenerator {
	return &CSVReportGenerator{
		name: "CSVReportGenerator",
	}
}

// GenerateReport 生成CSV格式报告
func (crg *CSVReportGenerator) GenerateReport(metrics *PoolMetrics) ([]byte, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)

	// 写入摘要信息
	writer.Write([]string{"指标类型", "指标名称", "数值", "单位"})
	writer.Write([]string{"摘要", "活跃连接数", strconv.Itoa(metrics.ActiveConnections), "个"})
	writer.Write([]string{"摘要", "总连接数", strconv.FormatUint(metrics.TotalConnections, 10), "个"})
	writer.Write([]string{"摘要", "峰值连接数", strconv.Itoa(metrics.PeakConnections), "个"})
	writer.Write([]string{"摘要", "总传输字节数", strconv.FormatUint(metrics.TotalBytesTransferred, 10), "字节"})
	writer.Write([]string{"摘要", "总消息数", strconv.FormatUint(metrics.TotalMessages, 10), "条"})
	writer.Write([]string{"摘要", "平均延迟", strconv.FormatInt(metrics.AverageLatency.Milliseconds(), 10), "毫秒"})
	writer.Write([]string{"摘要", "错误率", fmt.Sprintf("%.2f", metrics.ErrorRate*100), "%"})
	writer.Write([]string{"摘要", "吞吐量", fmt.Sprintf("%.2f", metrics.Throughput), "消息/秒"})

	// 空行分隔
	writer.Write([]string{})

	// 写入设备指标
	writer.Write([]string{"设备ID", "连接时间", "最后活动", "发送字节", "接收字节", "发送消息", "接收消息", "错误数", "延迟(ms)", "优先级", "健康评分", "使用次数"})
	for deviceID, deviceMetrics := range metrics.ConnectionsByDevice {
		writer.Write([]string{
			deviceID,
			deviceMetrics.ConnectionTime.Format(time.RFC3339),
			deviceMetrics.LastActivity.Format(time.RFC3339),
			strconv.FormatUint(deviceMetrics.BytesSent, 10),
			strconv.FormatUint(deviceMetrics.BytesReceived, 10),
			strconv.FormatUint(deviceMetrics.MessagesSent, 10),
			strconv.FormatUint(deviceMetrics.MessagesReceived, 10),
			strconv.FormatUint(deviceMetrics.ErrorCount, 10),
			strconv.FormatInt(deviceMetrics.AverageLatency.Milliseconds(), 10),
			deviceMetrics.Priority.String(),
			fmt.Sprintf("%.2f", deviceMetrics.HealthScore),
			strconv.FormatUint(deviceMetrics.UseCount, 10),
		})
	}

	writer.Flush()
	return buffer.Bytes(), writer.Error()
}

// GetReportFormat 获取报告格式
func (crg *CSVReportGenerator) GetReportFormat() string {
	return "csv"
}

// NewHTMLReportGenerator 创建HTML报告生成器
func NewHTMLReportGenerator() *HTMLReportGenerator {
	funcMap := template.FuncMap{
		"mul": func(a, b float64) float64 { return a * b },
	}
	tmpl := template.Must(template.New("report").Funcs(funcMap).Parse(htmlReportTemplate))
	return &HTMLReportGenerator{
		name:     "HTMLReportGenerator",
		template: tmpl,
	}
}

// GenerateReport 生成HTML格式报告
func (hrg *HTMLReportGenerator) GenerateReport(metrics *PoolMetrics) ([]byte, error) {
	var buffer bytes.Buffer

	// 准备模板数据
	data := struct {
		*PoolMetrics
		GeneratedAt string
		Uptime      string
	}{
		PoolMetrics: metrics,
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		Uptime:      time.Since(metrics.StartTime).String(),
	}

	err := hrg.template.Execute(&buffer, data)
	if err != nil {
		return nil, fmt.Errorf("生成HTML报告失败: %w", err)
	}

	return buffer.Bytes(), nil
}

// GetReportFormat 获取报告格式
func (hrg *HTMLReportGenerator) GetReportFormat() string {
	return "html"
}

// NewTextReportGenerator 创建文本报告生成器
func NewTextReportGenerator() *TextReportGenerator {
	return &TextReportGenerator{
		name: "TextReportGenerator",
	}
}

// GenerateReport 生成文本格式报告
func (trg *TextReportGenerator) GenerateReport(metrics *PoolMetrics) ([]byte, error) {
	var buffer bytes.Buffer

	// 报告标题
	buffer.WriteString("蓝牙连接池监控报告\n")
	buffer.WriteString("==================\n\n")

	// 生成时间和运行时间
	buffer.WriteString(fmt.Sprintf("生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	buffer.WriteString(fmt.Sprintf("运行时间: %s\n\n", time.Since(metrics.StartTime).String()))

	// 连接池摘要
	buffer.WriteString("连接池摘要\n")
	buffer.WriteString("----------\n")
	buffer.WriteString(fmt.Sprintf("活跃连接数: %d\n", metrics.ActiveConnections))
	buffer.WriteString(fmt.Sprintf("总连接数: %d\n", metrics.TotalConnections))
	buffer.WriteString(fmt.Sprintf("峰值连接数: %d\n", metrics.PeakConnections))
	buffer.WriteString(fmt.Sprintf("创建连接数: %d\n", metrics.ConnectionsCreated))
	buffer.WriteString(fmt.Sprintf("关闭连接数: %d\n", metrics.ConnectionsClosed))
	buffer.WriteString(fmt.Sprintf("拒绝连接数: %d\n", metrics.ConnectionsRejected))
	buffer.WriteString("\n")

	// 性能指标
	buffer.WriteString("性能指标\n")
	buffer.WriteString("--------\n")
	buffer.WriteString(fmt.Sprintf("总传输字节数: %d\n", metrics.TotalBytesTransferred))
	buffer.WriteString(fmt.Sprintf("总消息数: %d\n", metrics.TotalMessages))
	buffer.WriteString(fmt.Sprintf("平均延迟: %v\n", metrics.AverageLatency))
	buffer.WriteString(fmt.Sprintf("错误率: %.2f%%\n", metrics.ErrorRate*100))
	buffer.WriteString(fmt.Sprintf("吞吐量: %.2f 消息/秒\n", metrics.Throughput))
	buffer.WriteString("\n")

	// 按优先级分组的连接数
	if len(metrics.ConnectionsByPriority) > 0 {
		buffer.WriteString("按优先级分组的连接数\n")
		buffer.WriteString("------------------\n")
		for priority, count := range metrics.ConnectionsByPriority {
			buffer.WriteString(fmt.Sprintf("%s: %d\n", priority.String(), count))
		}
		buffer.WriteString("\n")
	}

	// 设备详细信息
	if len(metrics.ConnectionsByDevice) > 0 {
		buffer.WriteString("设备详细信息\n")
		buffer.WriteString("------------\n")

		// 按设备ID排序
		var deviceIDs []string
		for deviceID := range metrics.ConnectionsByDevice {
			deviceIDs = append(deviceIDs, deviceID)
		}
		sort.Strings(deviceIDs)

		for _, deviceID := range deviceIDs {
			deviceMetrics := metrics.ConnectionsByDevice[deviceID]
			buffer.WriteString(fmt.Sprintf("设备ID: %s\n", deviceID))
			buffer.WriteString(fmt.Sprintf("  连接时间: %s\n", deviceMetrics.ConnectionTime.Format("2006-01-02 15:04:05")))
			buffer.WriteString(fmt.Sprintf("  最后活动: %s\n", deviceMetrics.LastActivity.Format("2006-01-02 15:04:05")))
			buffer.WriteString(fmt.Sprintf("  发送字节: %d\n", deviceMetrics.BytesSent))
			buffer.WriteString(fmt.Sprintf("  接收字节: %d\n", deviceMetrics.BytesReceived))
			buffer.WriteString(fmt.Sprintf("  发送消息: %d\n", deviceMetrics.MessagesSent))
			buffer.WriteString(fmt.Sprintf("  接收消息: %d\n", deviceMetrics.MessagesReceived))
			buffer.WriteString(fmt.Sprintf("  错误数: %d\n", deviceMetrics.ErrorCount))
			buffer.WriteString(fmt.Sprintf("  平均延迟: %v\n", deviceMetrics.AverageLatency))
			buffer.WriteString(fmt.Sprintf("  优先级: %s\n", deviceMetrics.Priority.String()))
			buffer.WriteString(fmt.Sprintf("  健康评分: %.2f\n", deviceMetrics.HealthScore))
			buffer.WriteString(fmt.Sprintf("  使用次数: %d\n", deviceMetrics.UseCount))
			buffer.WriteString("\n")
		}
	}

	// 告警信息
	if len(metrics.Alerts) > 0 {
		buffer.WriteString("告警信息\n")
		buffer.WriteString("--------\n")

		// 按时间排序告警
		alerts := make([]Alert, len(metrics.Alerts))
		copy(alerts, metrics.Alerts)
		sort.Slice(alerts, func(i, j int) bool {
			return alerts[i].Timestamp.After(alerts[j].Timestamp)
		})

		for _, alert := range alerts {
			buffer.WriteString(fmt.Sprintf("[%s] %s - %s\n",
				alert.Timestamp.Format("2006-01-02 15:04:05"),
				trg.getAlertSeverityString(alert.Severity),
				alert.Message))
			if alert.Component != "" {
				buffer.WriteString(fmt.Sprintf("  组件: %s\n", alert.Component))
			}
		}
		buffer.WriteString("\n")
	}

	// 性能历史（最近10个快照）
	if len(metrics.PerformanceHistory) > 0 {
		buffer.WriteString("性能历史（最近10个快照）\n")
		buffer.WriteString("----------------------\n")

		// 取最近的10个快照
		history := metrics.PerformanceHistory
		if len(history) > 10 {
			history = history[len(history)-10:]
		}

		for _, snapshot := range history {
			buffer.WriteString(fmt.Sprintf("%s - 连接数: %d, 吞吐量: %.2f, 延迟: %v, 错误率: %.2f%%\n",
				snapshot.Timestamp.Format("15:04:05"),
				snapshot.ActiveConnections,
				snapshot.Throughput,
				snapshot.AverageLatency,
				snapshot.ErrorRate*100))
		}
	}

	return buffer.Bytes(), nil
}

// GetReportFormat 获取报告格式
func (trg *TextReportGenerator) GetReportFormat() string {
	return "text"
}

// getAlertSeverityString 获取告警严重程度字符串
func (trg *TextReportGenerator) getAlertSeverityString(severity AlertSeverity) string {
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

// HTML报告模板
const htmlReportTemplate = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>蓝牙连接池监控报告</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background-color: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background-color: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1, h2 { color: #333; }
        .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; margin: 20px 0; }
        .metric-card { background: #f8f9fa; padding: 15px; border-radius: 6px; border-left: 4px solid #007bff; }
        .metric-value { font-size: 24px; font-weight: bold; color: #007bff; }
        .metric-label { color: #666; font-size: 14px; }
        table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        th, td { padding: 10px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background-color: #f8f9fa; font-weight: bold; }
        .alert { padding: 10px; margin: 5px 0; border-radius: 4px; }
        .alert-info { background-color: #d1ecf1; border-left: 4px solid #17a2b8; }
        .alert-warn { background-color: #fff3cd; border-left: 4px solid #ffc107; }
        .alert-error { background-color: #f8d7da; border-left: 4px solid #dc3545; }
        .alert-critical { background-color: #f5c6cb; border-left: 4px solid #721c24; }
        .timestamp { color: #666; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>蓝牙连接池监控报告</h1>
        <p><strong>生成时间:</strong> {{.GeneratedAt}}</p>
        <p><strong>运行时间:</strong> {{.Uptime}}</p>
        
        <h2>连接池摘要</h2>
        <div class="summary">
            <div class="metric-card">
                <div class="metric-value">{{.ActiveConnections}}</div>
                <div class="metric-label">活跃连接数</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">{{.TotalConnections}}</div>
                <div class="metric-label">总连接数</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">{{.PeakConnections}}</div>
                <div class="metric-label">峰值连接数</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">{{.TotalBytesTransferred}}</div>
                <div class="metric-label">总传输字节数</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">{{printf "%.2f" .Throughput}}</div>
                <div class="metric-label">吞吐量 (消息/秒)</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">{{printf "%.2f%%" (mul .ErrorRate 100)}}</div>
                <div class="metric-label">错误率</div>
            </div>
        </div>
        
        {{if .ConnectionsByDevice}}
        <h2>设备连接详情</h2>
        <table>
            <thead>
                <tr>
                    <th>设备ID</th>
                    <th>连接时间</th>
                    <th>最后活动</th>
                    <th>发送字节</th>
                    <th>接收字节</th>
                    <th>错误数</th>
                    <th>健康评分</th>
                    <th>优先级</th>
                </tr>
            </thead>
            <tbody>
                {{range $deviceID, $metrics := .ConnectionsByDevice}}
                <tr>
                    <td>{{$deviceID}}</td>
                    <td>{{$metrics.ConnectionTime.Format "2006-01-02 15:04:05"}}</td>
                    <td>{{$metrics.LastActivity.Format "2006-01-02 15:04:05"}}</td>
                    <td>{{$metrics.BytesSent}}</td>
                    <td>{{$metrics.BytesReceived}}</td>
                    <td>{{$metrics.ErrorCount}}</td>
                    <td>{{printf "%.2f" $metrics.HealthScore}}</td>
                    <td>{{$metrics.Priority.String}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
        {{end}}
        
        {{if .Alerts}}
        <h2>告警信息</h2>
        {{range .Alerts}}
        <div class="alert alert-{{if eq .Level 0}}info{{else if eq .Level 1}}warn{{else if eq .Level 2}}error{{else}}critical{{end}}">
            <strong>{{.Message}}</strong>
            {{if .DeviceID}}<br>设备ID: {{.DeviceID}}{{end}}
            <div class="timestamp">{{.Timestamp.Format "2006-01-02 15:04:05"}}</div>
        </div>
        {{end}}
        {{end}}
    </div>
</body>
</html>
`

// CustomReportGenerator 自定义报告生成器
type CustomReportGenerator struct {
	name      string
	format    string
	generator func(*PoolMetrics) ([]byte, error)
}

// NewCustomReportGenerator 创建自定义报告生成器
func NewCustomReportGenerator(name, format string, generator func(*PoolMetrics) ([]byte, error)) *CustomReportGenerator {
	return &CustomReportGenerator{
		name:      name,
		format:    format,
		generator: generator,
	}
}

// GenerateReport 生成自定义格式报告
func (crg *CustomReportGenerator) GenerateReport(metrics *PoolMetrics) ([]byte, error) {
	if crg.generator != nil {
		return crg.generator(metrics)
	}
	return nil, fmt.Errorf("未设置报告生成器函数")
}

// GetReportFormat 获取报告格式
func (crg *CustomReportGenerator) GetReportFormat() string {
	return crg.format
}

// MultiFormatReportGenerator 多格式报告生成器
type MultiFormatReportGenerator struct {
	name       string
	generators map[string]ReportGenerator
}

// NewMultiFormatReportGenerator 创建多格式报告生成器
func NewMultiFormatReportGenerator() *MultiFormatReportGenerator {
	return &MultiFormatReportGenerator{
		name:       "MultiFormatReportGenerator",
		generators: make(map[string]ReportGenerator),
	}
}

// AddGenerator 添加格式生成器
func (mfrg *MultiFormatReportGenerator) AddGenerator(generator ReportGenerator) {
	mfrg.generators[generator.GetReportFormat()] = generator
}

// GenerateReport 生成指定格式的报告
func (mfrg *MultiFormatReportGenerator) GenerateReport(metrics *PoolMetrics) ([]byte, error) {
	// 默认生成JSON格式
	return mfrg.GenerateReportByFormat(metrics, "json")
}

// GenerateReportByFormat 按格式生成报告
func (mfrg *MultiFormatReportGenerator) GenerateReportByFormat(metrics *PoolMetrics, format string) ([]byte, error) {
	generator, exists := mfrg.generators[format]
	if !exists {
		return nil, fmt.Errorf("不支持的报告格式: %s", format)
	}

	return generator.GenerateReport(metrics)
}

// GetReportFormat 获取报告格式
func (mfrg *MultiFormatReportGenerator) GetReportFormat() string {
	var formats []string
	for format := range mfrg.generators {
		formats = append(formats, format)
	}
	return strings.Join(formats, ",")
}

// GetSupportedFormats 获取支持的格式列表
func (mfrg *MultiFormatReportGenerator) GetSupportedFormats() []string {
	var formats []string
	for format := range mfrg.generators {
		formats = append(formats, format)
	}
	return formats
}
