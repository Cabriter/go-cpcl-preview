package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// NewProjectLogger 创建同时写入终端和日志文件的 logger。
func NewProjectLogger(projectDir string) (*log.Logger, string, error) {
	projectName := filepath.Base(projectDir)
	dateStr := time.Now().Format("2006-01-02")
	logFileName := fmt.Sprintf("%s_%s.log", projectName, dateStr)
	logPath := filepath.Join(projectDir, logFileName)

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, "", fmt.Errorf("打开日志文件失败: %w", err)
	}

	multi := io.MultiWriter(os.Stdout, f)
	l := log.New(multi, "", log.LstdFlags|log.Lshortfile)
	return l, logPath, nil
}
