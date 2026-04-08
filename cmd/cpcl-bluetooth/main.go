package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"cpcl_test/internal/bluetooth"
	"cpcl_test/internal/config"
	projectlogger "cpcl_test/internal/logger"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var errUserExit = errors.New("用户主动退出")

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
	logger.Printf("蓝牙打印程序启动，项目目录: %s", projectDir)
	logger.Printf("日志文件路径: %s", logPath)

	appCfg, configPath, err := config.LoadOrCreateConfig(projectDir)
	if err != nil {
		logger.Printf("加载配置失败: %v", err)
		os.Exit(1)
	}
	logger.Printf("加载配置成功: %s", configPath)
	logger.Printf("蓝牙配置: device_address=%s device_name=%s service_uuid=%s char_uuid=%s", appCfg.Bluetooth.DeviceAddress, appCfg.Bluetooth.DeviceName, appCfg.Bluetooth.ServiceUUID, appCfg.Bluetooth.WriteCharacteristicUUID)

	cpclData, err := os.ReadFile(appCfg.Print.CPCLPath)
	if err != nil {
		logger.Printf("读取 CPCL 文件失败: path=%s err=%v", appCfg.Print.CPCLPath, err)
		os.Exit(1)
	}
	logger.Printf("读取 CPCL 文件成功: path=%s bytes=%d", appCfg.Print.CPCLPath, len(cpclData))

	payload := prepareCPCLPayload(cpclData, appCfg.Print, logger)
	if appCfg.Bluetooth.Interactive {
		tryCount := 1
		for {
			logger.Printf("进入交互式选择流程，第 %d 次尝试", tryCount)
			sender := bluetooth.NewSender(appCfg.Bluetooth, logger)
			selected, err := chooseDeviceInteractively(sender)
			if err != nil {
				if errors.Is(err, errUserExit) {
					logger.Printf("用户已取消设备选择，程序退出")
					return
				}
				logger.Printf("交互式选择设备失败: %v", err)
				os.Exit(1)
			}
			appCfg.Bluetooth.DeviceAddress = selected.Address
			appCfg.Bluetooth.DeviceName = selected.Name
			logger.Printf("已选择设备: address=%s name=%s", selected.Address, selected.Name)

			sender = bluetooth.NewSender(appCfg.Bluetooth, logger)
			err = sender.SendCPCL(payload)
			if err == nil {
				logger.Printf("蓝牙打印流程完成")
				break
			}
			logger.Printf("蓝牙发送失败，将返回设备列表重新选择: %v", err)
			tryCount++
		}
		return
	}

	sender := bluetooth.NewSender(appCfg.Bluetooth, logger)
	if err := sender.SendCPCL(payload); err != nil {
		logger.Printf("蓝牙发送失败: %v", err)
		os.Exit(1)
	}

	logger.Printf("蓝牙打印流程完成")
}

func chooseDeviceInteractively(sender *bluetooth.Sender) (bluetooth.ScanDevice, error) {
	devices, err := sender.ScanDevices()
	if err != nil {
		return bluetooth.ScanDevice{}, err
	}
	if len(devices) == 0 {
		return bluetooth.ScanDevice{}, fmt.Errorf("未扫描到蓝牙设备")
	}
	fmt.Println("扫描到以下蓝牙设备，请输入序号选择：")
	for idx, device := range devices {
		name := device.Name
		if name == "" {
			name = "<NO_NAME>"
		}
		fmt.Printf("[%d] name=%s address=%s rssi=%d\n", idx+1, name, device.Address, device.RSSI)
	}
	fmt.Print("请选择设备序号（输入 0 或 q 退出）: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return bluetooth.ScanDevice{}, fmt.Errorf("读取输入失败: %w", err)
	}
	input := strings.TrimSpace(strings.ToLower(line))
	if input == "0" || input == "q" || input == "quit" || input == "exit" {
		return bluetooth.ScanDevice{}, errUserExit
	}
	index, err := strconv.Atoi(input)
	if err != nil || index < 1 || index > len(devices) {
		return bluetooth.ScanDevice{}, fmt.Errorf("无效序号: %s", input)
	}
	return devices[index-1], nil
}

// prepareCPCLPayload 根据配置补齐 FORM/PRINT 结尾，避免打印机不出纸。
func prepareCPCLPayload(raw []byte, printCfg config.PrintConfig, logger interface{ Printf(string, ...any) }) []byte {
	processedText := normalizeCPCLText(raw, printCfg.StripCommentLine)
	trimmed := bytes.TrimSpace([]byte(processedText))
	normalized := strings.ToUpper(string(trimmed))
	hasForm := strings.Contains(normalized, "\nFORM")
	hasPrint := strings.Contains(normalized, "\nPRINT")
	if strings.HasPrefix(normalized, "FORM") {
		hasForm = true
	}
	if strings.HasPrefix(normalized, "PRINT") {
		hasPrint = true
	}

	buffer := bytes.NewBuffer(make([]byte, 0, len(trimmed)+32))
	buffer.Write(trimmed)
	if printCfg.AppendPrintSuffix && !hasForm {
		logger.Printf("CPCL 未检测到 FORM，自动追加")
		buffer.WriteString("\nFORM")
	}
	if printCfg.AppendPrintSuffix && !hasPrint {
		logger.Printf("CPCL 未检测到 PRINT，自动追加")
		buffer.WriteString("\nPRINT")
	}
	buffer.WriteString("\n")
	encoded := encodeByConfig(buffer.String(), printCfg.Encoding, logger)
	logger.Printf("CPCL 编码: encoding=%s bytes=%d", printCfg.Encoding, len(encoded))
	return encoded
}

func normalizeCPCLText(raw []byte, stripCommentLine bool) string {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if !stripCommentLine {
		return text
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "#") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func encodeByConfig(content string, encoding string, logger interface{ Printf(string, ...any) }) []byte {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "utf8", "utf-8":
		return []byte(content)
	case "gbk":
		out, _, err := transform.String(simplifiedchinese.GBK.NewEncoder(), content)
		if err == nil {
			return []byte(out)
		}
		logger.Printf("GBK 编码失败，回退 UTF-8: %v", err)
		return []byte(content)
	case "gb18030":
		out, _, err := transform.String(simplifiedchinese.GB18030.NewEncoder(), content)
		if err == nil {
			return []byte(out)
		}
		logger.Printf("GB18030 编码失败，回退 UTF-8: %v", err)
		return []byte(content)
	default:
		logger.Printf("未知编码 %s，回退 UTF-8", encoding)
		return []byte(content)
	}
}
