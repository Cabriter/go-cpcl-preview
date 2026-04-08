# CPCL 预览渲染器

本项目用于读取 `cpcl_input.txt`，解析常见 CPCL 指令并渲染 PNG 预览图。
程序参数从 `app_config.json` 读取（输入文件、输出文件、字体、条码/二维码参数）。

## 支持指令

- `! 0 200 200 <height> <qty>`（解析高度）
- `SIZE/PW/PH`（画布尺寸）
- `TEXT/T/TEXT90/T180/T270`
- `LINE/INVERSE-LINE`
- `BOX/INVERSE-BOX`
- `BARCODE128 / BARCODE 128 / VBARCODE 128`
- `QRCODE`（单行和 `ENDQR` 块模式）

## 运行方式

```bash
go mod tidy
go run ./cmd/cpcl-preview
```

## 输出

- 渲染图片：`output/preview.png`
- 日志文件：`cpcl_test_YYYY-MM-DD.log`（同时输出到终端）

## 输入文件

请编辑项目根目录下 `cpcl_input.txt`，程序启动后会自动读取。  
如需修复中文乱码，请在 `app_config.json` 的 `render.font_path` 指向支持中文的 TTF/OTF 字体。
