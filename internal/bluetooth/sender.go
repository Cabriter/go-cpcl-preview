//go:build !darwin && !windows

package bluetooth

import (
	"fmt"
	"log"
	"runtime"

	"cpcl_test/internal/config"
)

type Sender struct {
	cfg    config.BluetoothConfig
	logger *log.Logger
}

type ScanDevice struct {
	Address string
	Name    string
	RSSI    int
}

func NewSender(cfg config.BluetoothConfig, logger *log.Logger) *Sender {
	return &Sender{
		cfg:    cfg,
		logger: logger,
	}
}

func (s *Sender) SendCPCL(data []byte) error {
	return fmt.Errorf("当前平台不支持蓝牙发送: %s，仅支持 darwin/windows", runtime.GOOS)
}

func (s *Sender) ScanDevices() ([]ScanDevice, error) {
	return nil, fmt.Errorf("当前平台不支持蓝牙扫描: %s，仅支持 darwin/windows", runtime.GOOS)
}
