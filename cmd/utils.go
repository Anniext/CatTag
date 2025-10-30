package cmd

import (
	"log"
	"os"
)

// initLogger 初始化日志系统
func initLogger(level string) {
	// TODO: 实现更完善的日志初始化
	// 可以使用 slog 或其他结构化日志库

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	switch level {
	case "debug":
		log.SetOutput(os.Stdout)
		log.Println("日志级别设置为: DEBUG")
	case "info":
		log.SetOutput(os.Stdout)
		log.Println("日志级别设置为: INFO")
	case "warn":
		log.SetOutput(os.Stderr)
		log.Println("日志级别设置为: WARN")
	case "error":
		log.SetOutput(os.Stderr)
		log.Println("日志级别设置为: ERROR")
	default:
		log.SetOutput(os.Stdout)
		log.Printf("未知日志级别 '%s'，使用默认级别 INFO", level)
	}
}
