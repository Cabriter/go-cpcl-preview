package bluetooth

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"cpcl_test/internal/config"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/darwin"
)

// Sender 负责蓝牙连接与 CPCL 字节发送。
type Sender struct {
	cfg    config.BluetoothConfig
	logger *log.Logger
}

var bleDeviceReady bool

type ScanDevice struct {
	Address string
	Name    string
	RSSI    int
}

type connectResult struct {
	client ble.Client
	err    error
}

// NewSender 创建蓝牙发送器。
func NewSender(cfg config.BluetoothConfig, logger *log.Logger) *Sender {
	return &Sender{
		cfg:    cfg,
		logger: logger,
	}
}

// SendCPCL 连接蓝牙打印机并发送 CPCL 字节流。
func (s *Sender) SendCPCL(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("待发送数据为空")
	}
	if s.cfg.DeviceAddress == "" && s.cfg.DeviceName == "" {
		return fmt.Errorf("配置缺失: device_address 与 device_name 不能同时为空")
	}

	if err := ensureBLEDevice(); err != nil {
		return err
	}

	s.logger.Printf("开始连接蓝牙设备，地址=%s 名称=%s 扫描超时=%ds 连接超时=%ds", s.cfg.DeviceAddress, s.cfg.DeviceName, s.cfg.ScanTimeoutSeconds, s.cfg.ConnectTimeoutSeconds)
	connectCtx, cancel := context.WithTimeout(context.Background(), time.Duration(s.cfg.ConnectTimeoutSeconds)*time.Second)
	defer cancel()
	s.logger.Printf("连接阶段不支持输入交互，正在等待目标设备广播，超时后会返回重试流程")

	seenAdvertisements := map[string]struct{}{}
	client, err := s.connectWithTimeout(connectCtx, func(a ble.Advertisement) bool {
		addr := strings.TrimSpace(a.Addr().String())
		name := strings.TrimSpace(a.LocalName())
		key := addr + "|" + name
		if _, exists := seenAdvertisements[key]; !exists {
			seenAdvertisements[key] = struct{}{}
			s.logger.Printf("扫描到蓝牙设备: addr=%s name=%s rssi=%d", addr, name, a.RSSI())
		}
		return matchDevice(a, s.cfg.DeviceAddress, s.cfg.DeviceName)
	})
	if err != nil {
		return fmt.Errorf("连接蓝牙设备失败: %w", err)
	}
	defer func() {
		s.logger.Printf("断开蓝牙连接")
		client.CancelConnection()
	}()

	s.logger.Printf("蓝牙连接成功，开始发现服务与特征")
	profile, err := client.DiscoverProfile(true)
	if err != nil {
		return fmt.Errorf("发现服务失败: %w", err)
	}

	writeChar, serviceID, charID := findWriteCharacteristic(profile, s.cfg.ServiceUUID, s.cfg.WriteCharacteristicUUID)
	if writeChar == nil {
		return fmt.Errorf("未找到可写特征: service=%s char=%s", s.cfg.ServiceUUID, s.cfg.WriteCharacteristicUUID)
	}
	s.logger.Printf("发现写入特征成功: service=%s char=%s", serviceID, charID)

	chunkSize := s.cfg.WriteChunkSize
	totalChunks := (len(data) + chunkSize - 1) / chunkSize
	s.logger.Printf("开始发送 CPCL 数据，总字节=%d 分包大小=%d 分包数=%d no_rsp=%v", len(data), chunkSize, totalChunks, s.cfg.WriteWithoutResponse)
	for offset, index := 0, 1; offset < len(data); offset, index = offset+chunkSize, index+1 {
		end := offset + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunk := data[offset:end]
		if err := client.WriteCharacteristic(writeChar, chunk, s.cfg.WriteWithoutResponse); err != nil {
			return fmt.Errorf("发送分包失败(第%d/%d包): %w", index, totalChunks, err)
		}
		// 加入短暂间隔，避免打印机缓存处理不过来。
		time.Sleep(20 * time.Millisecond)
	}

	s.logger.Printf("CPCL 数据发送完成")
	return nil
}

func (s *Sender) connectWithTimeout(ctx context.Context, filter func(ble.Advertisement) bool) (ble.Client, error) {
	resultCh := make(chan connectResult, 1)
	go func() {
		client, err := ble.Connect(ctx, filter)
		resultCh <- connectResult{client: client, err: err}
	}()
	select {
	case <-ctx.Done():
		_ = ble.Stop()
		return nil, fmt.Errorf("连接超时: %w", ctx.Err())
	case result := <-resultCh:
		return result.client, result.err
	}
}

func (s *Sender) ScanDevices() ([]ScanDevice, error) {
	if err := ensureBLEDevice(); err != nil {
		return nil, err
	}
	timeout := time.Duration(s.cfg.ScanTimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	results := make(map[string]ScanDevice)
	s.logger.Printf("开始扫描蓝牙设备，超时=%ds", s.cfg.ScanTimeoutSeconds)
	err := ble.Scan(ctx, false, func(a ble.Advertisement) {
		addr := strings.TrimSpace(a.Addr().String())
		name := strings.TrimSpace(a.LocalName())
		if addr == "" {
			return
		}
		device := ScanDevice{
			Address: addr,
			Name:    name,
			RSSI:    a.RSSI(),
		}
		old, exists := results[addr]
		if !exists || device.RSSI > old.RSSI {
			results[addr] = device
		}
	}, nil)
	if err != nil && err != context.DeadlineExceeded {
		return nil, fmt.Errorf("扫描蓝牙设备失败: %w", err)
	}
	list := make([]ScanDevice, 0, len(results))
	for _, item := range results {
		list = append(list, item)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].RSSI > list[j].RSSI
	})
	s.logger.Printf("扫描完成，发现设备数=%d", len(list))
	return list, nil
}

func ensureBLEDevice() error {
	if bleDeviceReady {
		return nil
	}
	device, err := darwin.NewDevice()
	if err != nil {
		return fmt.Errorf("初始化蓝牙设备失败: %w", err)
	}
	ble.SetDefaultDevice(device)
	bleDeviceReady = true
	return nil
}

func matchDevice(advertisement ble.Advertisement, targetAddress string, targetName string) bool {
	addr := strings.TrimSpace(advertisement.Addr().String())
	name := strings.TrimSpace(advertisement.LocalName())

	if targetAddress != "" && strings.EqualFold(addr, strings.TrimSpace(targetAddress)) {
		return true
	}
	if targetName != "" && strings.EqualFold(name, strings.TrimSpace(targetName)) {
		return true
	}
	return false
}

func findWriteCharacteristic(profile *ble.Profile, serviceUUIDText string, charUUIDText string) (*ble.Characteristic, string, string) {
	var serviceUUID ble.UUID
	var charUUID ble.UUID
	var hasServiceUUID bool
	var hasCharUUID bool
	if strings.TrimSpace(serviceUUIDText) != "" {
		parsed, err := ble.Parse(strings.TrimSpace(serviceUUIDText))
		if err == nil {
			serviceUUID = parsed
			hasServiceUUID = true
		}
	}
	if strings.TrimSpace(charUUIDText) != "" {
		parsed, err := ble.Parse(strings.TrimSpace(charUUIDText))
		if err == nil {
			charUUID = parsed
			hasCharUUID = true
		}
	}
	for _, service := range profile.Services {
		if hasServiceUUID && !service.UUID.Equal(serviceUUID) {
			continue
		}
		for _, characteristic := range service.Characteristics {
			if hasCharUUID && !characteristic.UUID.Equal(charUUID) {
				continue
			}
			if characteristic.Property&ble.CharWrite == 0 && characteristic.Property&ble.CharWriteNR == 0 {
				continue
			}
			return characteristic, service.UUID.String(), characteristic.UUID.String()
		}
	}
	return nil, "", ""
}
