package main

import (
	"fmt"
	"os"

	"cpcl_test/internal/config"
	projectlogger "cpcl_test/internal/logger"
	"cpcl_test/internal/parser"
	"cpcl_test/internal/render"
)

func main() {
	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("获取当前目录失败: %v\n", err)
		os.Exit(1)
	}

	logger, logPath, err := projectlogger.NewProjectLogger(projectDir)
	if err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	logger.Printf("程序启动，项目目录: %s", projectDir)
	logger.Printf("日志文件路径: %s", logPath)

	appCfg, configPath, err := config.LoadOrCreateConfig(projectDir)
	if err != nil {
		logger.Printf("加载配置失败: %v", err)
		os.Exit(1)
	}
	logger.Printf("加载配置成功: %s", configPath)
	logger.Printf("配置参数: input=%s output=%s font=%s", appCfg.InputPath, appCfg.OutputPath, appCfg.Render.FontPath)

	logger.Printf("准备读取 CPCL 输入文件: %s", appCfg.InputPath)
	inputFile, err := os.Open(appCfg.InputPath)
	if err != nil {
		logger.Printf("打开输入文件失败: %v", err)
		os.Exit(1)
	}
	defer inputFile.Close()

	label, err := parser.ParseCPCL(inputFile)
	if err != nil {
		logger.Printf("解析 CPCL 失败: %v", err)
		os.Exit(1)
	}
	logger.Printf("解析成功: 宽=%d 高=%d 指令数=%d", label.Width, label.Height, len(label.Instructions))

	if err := render.RenderToPNG(label, appCfg.OutputPath, appCfg.Render, logger); err != nil {
		logger.Printf("渲染 PNG 失败: %v", err)
		os.Exit(1)
	}

	logger.Printf("渲染成功，输出文件: %s", appCfg.OutputPath)
}
