package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AppConfig 保存程序运行参数，统一从文件读取，避免硬编码。
type AppConfig struct {
	InputPath  string          `json:"input_path"`
	OutputPath string          `json:"output_path"`
	Render     RenderConfig    `json:"render"`
	Print      PrintConfig     `json:"print"`
	Bluetooth  BluetoothConfig `json:"bluetooth"`
}

// RenderConfig 保存渲染相关参数。
type RenderConfig struct {
	FontPath           string  `json:"font_path"`
	FontSize           float64 `json:"font_size"`
	BarcodeModuleWidth int     `json:"barcode_module_width"`
	QRCodeModuleSize   int     `json:"qrcode_module_size"`
}

// PrintConfig 保存蓝牙打印输入参数。
type PrintConfig struct {
	CPCLPath          string `json:"cpcl_path"`
	AppendPrintSuffix bool   `json:"append_print_suffix"`
	Encoding          string `json:"encoding"`
	StripCommentLine  bool   `json:"strip_comment_line"`
}

// BluetoothConfig 保存蓝牙连接参数。
type BluetoothConfig struct {
	DeviceAddress           string `json:"device_address"`
	DeviceName              string `json:"device_name"`
	Interactive             bool   `json:"interactive"`
	ScanTimeoutSeconds      int    `json:"scan_timeout_seconds"`
	ConnectTimeoutSeconds   int    `json:"connect_timeout_seconds"`
	ServiceUUID             string `json:"service_uuid"`
	WriteCharacteristicUUID string `json:"write_characteristic_uuid"`
	WriteChunkSize          int    `json:"write_chunk_size"`
	WriteWithoutResponse    bool   `json:"write_without_response"`
}

// LoadOrCreateConfig 读取配置文件；文件不存在时写入示例配置。
func LoadOrCreateConfig(projectDir string) (*AppConfig, string, error) {
	configPath := filepath.Join(projectDir, "app_config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		example := defaultConfig()
		if err := writeConfig(configPath, example); err != nil {
			return nil, "", fmt.Errorf("写入示例配置失败: %w", err)
		}
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, "", fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg AppConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, "", fmt.Errorf("解析配置文件失败: %w", err)
	}

	applyDefaultValues(&cfg)
	resolveRelativePaths(&cfg, projectDir)
	return &cfg, configPath, nil
}

func writeConfig(configPath string, cfg *AppConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(configPath, data, 0o644)
}

func defaultConfig() *AppConfig {
	return &AppConfig{
		InputPath:  "./cpcl_input.txt",
		OutputPath: "./output/preview.png",
		Render: RenderConfig{
			FontPath:           "./Arial Unicode.ttf",
			FontSize:           24,
			BarcodeModuleWidth: 2,
			QRCodeModuleSize:   6,
		},
		Print: PrintConfig{
			CPCLPath:          "./cpcl_input.txt",
			AppendPrintSuffix: true,
			Encoding:          "gbk",
			StripCommentLine:  true,
		},
		Bluetooth: BluetoothConfig{
			DeviceAddress:           "",
			DeviceName:              "CC3",
			Interactive:             true,
			ScanTimeoutSeconds:      15,
			ConnectTimeoutSeconds:   15,
			ServiceUUID:             "0000FF00-0000-1000-8000-00805F9B34FB",
			WriteCharacteristicUUID: "0000FF02-0000-1000-8000-00805F9B34FB",
			WriteChunkSize:          180,
			WriteWithoutResponse:    true,
		},
	}
}

func applyDefaultValues(cfg *AppConfig) {
	defaultCfg := defaultConfig()
	if cfg.InputPath == "" {
		cfg.InputPath = defaultCfg.InputPath
	}
	if cfg.OutputPath == "" {
		cfg.OutputPath = defaultCfg.OutputPath
	}
	if cfg.Render.FontPath == "" {
		cfg.Render.FontPath = defaultCfg.Render.FontPath
	}
	if cfg.Render.FontSize <= 0 {
		cfg.Render.FontSize = defaultCfg.Render.FontSize
	}
	if cfg.Render.BarcodeModuleWidth <= 0 {
		cfg.Render.BarcodeModuleWidth = defaultCfg.Render.BarcodeModuleWidth
	}
	if cfg.Render.QRCodeModuleSize <= 0 {
		cfg.Render.QRCodeModuleSize = defaultCfg.Render.QRCodeModuleSize
	}
	if cfg.Print.CPCLPath == "" {
		cfg.Print.CPCLPath = defaultCfg.Print.CPCLPath
	}
	if cfg.Print.Encoding == "" {
		cfg.Print.Encoding = defaultCfg.Print.Encoding
	}
	if cfg.Bluetooth.ScanTimeoutSeconds <= 0 {
		cfg.Bluetooth.ScanTimeoutSeconds = defaultCfg.Bluetooth.ScanTimeoutSeconds
	}
	if cfg.Bluetooth.ConnectTimeoutSeconds <= 0 {
		cfg.Bluetooth.ConnectTimeoutSeconds = defaultCfg.Bluetooth.ConnectTimeoutSeconds
	}
	if cfg.Bluetooth.ServiceUUID == "" {
		cfg.Bluetooth.ServiceUUID = defaultCfg.Bluetooth.ServiceUUID
	}
	if cfg.Bluetooth.WriteCharacteristicUUID == "" {
		cfg.Bluetooth.WriteCharacteristicUUID = defaultCfg.Bluetooth.WriteCharacteristicUUID
	}
	if cfg.Bluetooth.WriteChunkSize <= 0 {
		cfg.Bluetooth.WriteChunkSize = defaultCfg.Bluetooth.WriteChunkSize
	}
	if cfg.Bluetooth.DeviceName == "" && cfg.Bluetooth.DeviceAddress == "" {
		cfg.Bluetooth.DeviceName = defaultCfg.Bluetooth.DeviceName
	}
}

func resolveRelativePaths(cfg *AppConfig, projectDir string) {
	cfg.InputPath = resolvePath(cfg.InputPath, projectDir)
	cfg.OutputPath = resolvePath(cfg.OutputPath, projectDir)
	cfg.Render.FontPath = resolvePath(cfg.Render.FontPath, projectDir)
	cfg.Print.CPCLPath = resolvePath(cfg.Print.CPCLPath, projectDir)
}

func resolvePath(path, baseDir string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}
