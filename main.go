package main

import (
	"github.com/Anniext/CatTag/cmd"
)

// main 函数是应用程序的入口点
// 使用 Cobra 框架来处理命令行接口
func main() {
	// 执行根命令，这将解析命令行参数并执行相应的子命令
	cmd.Execute()
}
