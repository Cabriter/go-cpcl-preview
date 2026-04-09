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

## 文档

- `CPCL-输入说明.md`：如何编辑输入文件与排版建议。
- `CPCL-语义说明.md`：逐条解释每个指令中数字参数的语义、默认值与兼容行为。

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
> 4. 程序会输出扫描到的设备日志，便于排查匹配失败问题；
> 5. 发送前默认会执行“重建 CPCL”以提高不同固件兼容性，如需严格透传原文请设置 `print.skip_rebuild=true`。

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
  "strip_comment_line": true,
  "skip_rebuild": false,
  "export_rebuild_cpcl": false,
  "rebuild_cpcl_path": "./output/rebuild_cpcl.txt"
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

## 发送重建偏差说明

- `cpcl-bluetooth` 默认在发送前重建 CPCL（头部、文本、条码、二维码数字会按规则调整），这是为了兼容不同机型/固件对参数的解释差异。
- 如果你观察到“预览和实打数字不一致”，通常来自重建规则而非输入文件本身，可开启重建导出进行比对。
- 启用 `print.export_rebuild_cpcl=true` 后，会输出重建后的发送内容到 `print.rebuild_cpcl_path`，并在日志中记录导出路径和字节数。
- 如需验证“是否由重建导致偏差”，请临时设置 `print.skip_rebuild=true` 再打印对比。

## 偏差排查步骤

- 第一步：确认日志中 `CPCL 编码`、`导出重建 CPCL 文件成功`（若已开启导出）是否正常。
- 第二步：对比 `cpcl_input.txt` 与 `rebuild_cpcl_path`，重点看 `PW/PH`、`TEXT*`、`BARCODE/VBARCODE`、`B QR ... U` 的数字变化。
- 第三步：若重建后偏差明显，先尝试 `skip_rebuild=true`；若透传打印正常，可继续按业务调整输入模板。
- 第四步：若透传仍异常，检查 `encoding`（推荐 `gbk`/`gb18030`）、`service_uuid`、`write_characteristic_uuid` 及设备固件版本。
