package main

import (
	"mcp-mowen/service"

	"github.com/bytedance/gopkg/util/logger"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer(
		"mcp-mowen",
		"1.0.0",
	)
	logger.Info("初始化数据库...")
	if err := service.InitSQLite(); err != nil {
		logger.Fatalf("数据库初始化失败: %v", err)
	}

	logger.Info("开始注册工具...")
	service.RegisterAllTools(s)

	logger.Info("启动墨问MCP服务器...")
	if err := server.ServeStdio(s); err != nil {
		logger.Errorf("服务器错误: %v", err)
	}
}
