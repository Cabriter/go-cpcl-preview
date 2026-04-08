# CPCL 输入说明（`cpcl_input.txt`）

本文档用于帮助你快速理解并修改 `cpcl_input.txt`，重点面向当前项目的实际行为（预览 + 蓝牙发送）。

## 1. 坐标与纸张基础

- 坐标原点：左上角 `(0,0)`。
- X 轴：向右增大；Y 轴：向下增大。
- 常用纸宽（78mm）建议：
  - `PW 600`
  - `PH` 按内容高度设置，如 `920`

示例：

```cpcl
! 0 200 200 920 1
PW 600
PH 920
```

## 2. 项目支持的指令

### 页面与尺寸

- `! 0 200 200 <height> <qty>`
- `SIZE <width> <height>`
- `PW <width>` / `PAGE-WIDTH <width>`
- `PH <height>` / `PAGE-HEIGHT <height>`

### 文本

- `TEXT <x> <y> <text...>`
- `TEXT <font> <size> <x> <y> <text...>`
- `TEXT90 ...` / `TEXT180 ...` / `TEXT270 ...`

建议优先使用旋转专用指令（`TEXT90/180/270`），实机兼容性更稳定。

### 线与框

- `LINE <x1> <y1> <x2> <y2> [thickness]`
- `INVERSE-LINE <x1> <y1> <x2> <y2> [thickness]`
- `BOX <x> <y> <width> <height> [thickness]`
- `INVERSE-BOX <x> <y> <width> <height> [thickness]`

### 条码

- `BARCODE128 <x> <y> <height> <data...>`
- `BARCODE 128 <wide> <narrow> <height> <x> <y> <data...>`
- `VBARCODE 128 <wide> <narrow> <height> <x> <y> <data...>`

78mm 标签建议从窄条码开始，例如：

```cpcl
BARCODE 128 1 1 56 35 280 ORD-2601
```

### 二维码

- 单行：
  - `QRCODE <x> <y> <moduleSize> <data...>`
- 块模式：
  - `QRCODE <x> <y> M 2 U <moduleSize>`
  - `MA,<data>`
  - `ENDQR`

建议使用块模式，打印机兼容性更好：

```cpcl
QRCODE 360 400 M 2 U 4
MA,https://example.com/order/ORDER-2026-0001
ENDQR
```

## 3. 发送时会自动做的事

`go run ./cmd/cpcl-bluetooth` 发送前会自动处理：

1. 规范换行符（CRLF/CR -> LF）。
2. 可按配置移除 `#` 注释行（`strip_comment_line=true`）。
3. 解析并重建一份规范 CPCL。
4. `encoding=gbk/gb18030` 时自动加：
   - `COUNTRY CHINA`
   - `CODEPAGE 936`
5. 若末尾缺少 `FORM/PRINT` 且配置开启，会自动补齐。

这也是“预览能看、实机异常”时常见差异来源之一。

## 4. 78mm 排版经验（避免重叠）

- 条码和二维码必须分区，建议中间留至少 20~30 dots 空隙。
- 二维码建议 `U 4~6`，过大容易横向占满并与条码重叠。
- 竖排文字尽量放边缘区域（最左或最右），避免压住主体内容。
- 若文字被裁切：
  - 降低 X 值或减小字号；
  - 检查 `PW` 是否过小；
  - 检查是否用了不兼容的旋转写法。

## 5. 推荐编辑流程

1. 先改 `cpcl_input.txt`。
2. 运行预览：

```bash
go run ./cmd/cpcl-preview
```

3. 预览通过后再蓝牙发送：

```bash
go run ./cmd/cpcl-bluetooth
```

## 6. 当前可参考模板

你可以直接以当前 `cpcl_input.txt` 为模板继续微调：

- `PW/PH` 定义在文件头；
- 条码在左、二维码在右下；
- 竖排文本在边缘；
- 最后保留 `FORM`、`PRINT`。

