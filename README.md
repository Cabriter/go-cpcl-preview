# CPCL 工具集（预览 + 蓝牙发送）

本项目包含两个 Go 程序：

- `cpcl-preview`：读取 `cpcl_input.txt`，解析常见 CPCL 指令并渲染 PNG 预览图。
- `cpcl-bluetooth`：读取配置中的蓝牙参数与 `cpcl_input.txt`，连接芝柯 CC3 并发送 CPCL 字节打印。

程序参数统一从 `app_config.json` 读取（渲染参数、打印文件路径、蓝牙参数）。

## 支持指令

- `! 0 200 200 <height> <qty>`（解析高度）
- `SIZE/PW/PH`（画布尺寸）
- `TEXT/T/TEXT90/T180/T270`
- `LINE/INVERSE-LINE`
- `BOX/INVERSE-BOX`
- `BARCODE128 / BARCODE 128 / VBARCODE 128`
- `QRCODE`（单行和 `ENDQR` 块模式）

## 运行方式

### 1) CPCL 预览

```bash
go mod tidy
go run ./cmd/cpcl-preview
```

### 2) 蓝牙发送打印（芝柯 CC3）

```bash
go mod tidy
go run ./cmd/cpcl-bluetooth
```

> 注意：
> 1. 蓝牙发送支持 macOS 与 Windows；Linux 仍为不支持状态；
> 2. 默认特征 UUID 为 `FF02`，如设备固件不同请在配置文件中改为实际值；
> 3. `bluetooth.interactive=true` 时会先扫描设备并在终端中交互选择序号后再连接，若连接或发送失败会自动回到设备列表继续选择；
> 4. 程序会输出扫描到的设备日志，便于排查匹配失败问题。

## 输出

- 渲染图片：`output/preview.png`
- 日志文件：`cpcl_test_YYYY-MM-DD.log`（同时输出到终端）

## 输入文件与配置

请编辑项目根目录文件：

- `cpcl_input.txt`：用于预览渲染；
- `cpcl_input.txt`：用于蓝牙打印发送；
- `app_config.json`：统一配置。

关键蓝牙配置示例：

```json
"print": {
  "cpcl_path": "./cpcl_input.txt",
  "append_print_suffix": true,
  "encoding": "gbk",
  "strip_comment_line": true
},
"bluetooth": {
  "device_address": "",
  "device_name": "CC3",
  "interactive": true,
  "scan_timeout_seconds": 15,
  "connect_timeout_seconds": 15,
  "service_uuid": "0000FF00-0000-1000-8000-00805F9B34FB",
  "write_characteristic_uuid": "0000FF02-0000-1000-8000-00805F9B34FB",
  "write_chunk_size": 180,
  "write_without_response": true
}
```

如需修复中文乱码，请将 `render.font_path` 指向支持中文的 TTF/OTF 字体。
