package main

import (
	"fmt"
	"mcp-mowen/service"

	"github.com/bytedance/gopkg/util/logger"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer(
		"mcp-mowen",
		"1.0.0",
	)

	fmt.Println("开始注册工具...")
	service.RegisterAllTools(s)

	logger.Info("启动墨问MCP服务器...")
	if err := server.ServeStdio(s); err != nil {
		logger.Errorf("服务器错误: %v", err)
	}
}
