//go:build windows

package bluetooth

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"cpcl_test/internal/config"
	tble "tinygo.org/x/bluetooth"
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

type connectResult struct {
	device tble.Device
	err    error
}

var (
	adapterInitMu sync.Mutex
	adapterReady  bool
)

func NewSender(cfg config.BluetoothConfig, logger *log.Logger) *Sender {
	return &Sender{
		cfg:    cfg,
		logger: logger,
	}
}

func (s *Sender) SendCPCL(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("待发送数据为空")
	}
	if s.cfg.DeviceAddress == "" && s.cfg.DeviceName == "" {
		return fmt.Errorf("配置缺失: device_address 与 device_name 不能同时为空")
	}
	if err := ensureAdapter(); err != nil {
		return err
	}
	targetAddr, targetName, err := s.resolveTargetAddress()
	if err != nil {
		return err
	}
	if targetName == "" {
		targetName = s.cfg.DeviceName
	}
	s.logger.Printf("开始连接蓝牙设备，地址=%s 名称=%s 连接超时=%ds", targetAddr.String(), targetName, s.cfg.ConnectTimeoutSeconds)
	device, err := s.connectWithTimeout(targetAddr)
	if err != nil {
		return fmt.Errorf("连接蓝牙设备失败: %w", err)
	}
	defer device.Disconnect()
	writeChar, serviceID, charID, err := s.findWriteCharacteristic(device, s.cfg.ServiceUUID, s.cfg.WriteCharacteristicUUID)
	if err != nil {
		return err
	}
	s.logger.Printf("发现写入特征成功: service=%s char=%s", serviceID, charID)
	chunkSize := s.cfg.WriteChunkSize
	if chunkSize <= 0 {
		chunkSize = 180
	}
	totalChunks := (len(data) + chunkSize - 1) / chunkSize
	s.logger.Printf("开始发送 CPCL 数据，总字节=%d 分包大小=%d 分包数=%d no_rsp=%v", len(data), chunkSize, totalChunks, s.cfg.WriteWithoutResponse)
	for offset, index := 0, 1; offset < len(data); offset, index = offset+chunkSize, index+1 {
		end := offset + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunk := data[offset:end]
		err = s.writeChunk(writeChar, chunk)
		if err != nil {
			return fmt.Errorf("发送分包失败(第%d/%d包): %w", index, totalChunks, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.logger.Printf("CPCL 数据发送完成")
	return nil
}

func (s *Sender) ScanDevices() ([]ScanDevice, error) {
	if err := ensureAdapter(); err != nil {
		return nil, err
	}
	timeout := time.Duration(s.cfg.ScanTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	results := make(map[string]ScanDevice)
	s.logger.Printf("开始扫描蓝牙设备，超时=%ds", int(timeout/time.Second))
	stopTimer := time.AfterFunc(timeout, func() {
		_ = tble.DefaultAdapter.StopScan()
	})
	defer stopTimer.Stop()
	err := tble.DefaultAdapter.Scan(func(adapter *tble.Adapter, result tble.ScanResult) {
		addr := strings.TrimSpace(result.Address.String())
		name := strings.TrimSpace(result.LocalName())
		if addr == "" {
			return
		}
		item := ScanDevice{
			Address: addr,
			Name:    name,
			RSSI:    int(result.RSSI),
		}
		old, exists := results[addr]
		if !exists || item.RSSI > old.RSSI {
			results[addr] = item
		}
	})
	if err != nil {
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

func ensureAdapter() error {
	adapterInitMu.Lock()
	defer adapterInitMu.Unlock()
	if adapterReady {
		return nil
	}
	if err := tble.DefaultAdapter.Enable(); err != nil {
		return fmt.Errorf("初始化蓝牙设备失败: %w", err)
	}
	adapterReady = true
	return nil
}

func (s *Sender) resolveTargetAddress() (tble.Address, string, error) {
	targetAddress := strings.TrimSpace(s.cfg.DeviceAddress)
	targetName := strings.TrimSpace(s.cfg.DeviceName)
	if targetAddress != "" {
		mac, err := tble.ParseMAC(normalizeMAC(targetAddress))
		if err != nil {
			return tble.Address{}, "", fmt.Errorf("设备地址格式无效: %s", targetAddress)
		}
		return tble.Address{MACAddress: tble.MACAddress{MAC: mac}}, targetName, nil
	}
	devices, err := s.ScanDevices()
	if err != nil {
		return tble.Address{}, "", err
	}
	for _, device := range devices {
		if targetName != "" && strings.EqualFold(strings.TrimSpace(device.Name), targetName) {
			mac, parseErr := tble.ParseMAC(normalizeMAC(device.Address))
			if parseErr != nil {
				continue
			}
			return tble.Address{MACAddress: tble.MACAddress{MAC: mac}}, device.Name, nil
		}
	}
	return tble.Address{}, "", fmt.Errorf("未找到目标蓝牙设备: name=%s", targetName)
}

func (s *Sender) connectWithTimeout(address tble.Address) (tble.Device, error) {
	timeout := time.Duration(s.cfg.ConnectTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	resultCh := make(chan connectResult, 1)
	go func() {
		device, err := tble.DefaultAdapter.Connect(address, tble.ConnectionParams{})
		resultCh <- connectResult{device: device, err: err}
	}()
	select {
	case result := <-resultCh:
		return result.device, result.err
	case <-time.After(timeout):
		return tble.Device{}, fmt.Errorf("连接超时")
	}
}

func (s *Sender) findWriteCharacteristic(device tble.Device, serviceUUIDText string, charUUIDText string) (tble.DeviceCharacteristic, string, string, error) {
	serviceUUID, hasServiceUUID := parseUUID(serviceUUIDText)
	charUUID, hasCharUUID := parseUUID(charUUIDText)
	services, err := device.DiscoverServices(nil)
	if err != nil {
		return tble.DeviceCharacteristic{}, "", "", fmt.Errorf("发现服务失败: %w", err)
	}
	discovered := make([]string, 0, len(services)*2)
	for _, service := range services {
		serviceID := service.UUID().String()
		discovered = append(discovered, serviceID)
		chars, err := service.DiscoverCharacteristics(nil)
		if err != nil {
			continue
		}
		for _, char := range chars {
			discovered = append(discovered, serviceID+"->"+char.UUID().String())
			if hasServiceUUID && !equalUUID(service.UUID(), serviceUUID) {
				continue
			}
			if hasCharUUID && !equalUUID(char.UUID(), charUUID) {
				continue
			}
			return char, service.UUID().String(), char.UUID().String(), nil
		}
	}
	if len(discovered) > 0 {
		s.logger.Printf("已发现服务/特征: %s", strings.Join(discovered, ", "))
	}
	return tble.DeviceCharacteristic{}, "", "", fmt.Errorf("未找到可写特征: service=%s char=%s", serviceUUIDText, charUUIDText)
}

func (s *Sender) writeChunk(writeChar tble.DeviceCharacteristic, chunk []byte) error {
	if s.cfg.WriteWithoutResponse {
		if _, err := writeChar.WriteWithoutResponse(chunk); err == nil {
			return nil
		}
		_, err := writeChar.Write(chunk)
		return err
	}
	if _, err := writeChar.Write(chunk); err == nil {
		return nil
	}
	_, err := writeChar.WriteWithoutResponse(chunk)
	return err
}

func parseUUID(text string) (tble.UUID, bool) {
	val := strings.TrimSpace(text)
	if val == "" {
		return tble.UUID{}, false
	}
	parsed, err := tble.ParseUUID(val)
	if err != nil {
		return tble.UUID{}, false
	}
	return parsed, true
}

func equalUUID(left tble.UUID, right tble.UUID) bool {
	return left.Bytes() == right.Bytes()
}

func normalizeMAC(addr string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(addr), "-", ":"))
}
