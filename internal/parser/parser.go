package parser

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Label 保存整张标签画布信息和指令列表。
type Label struct {
	Width        int
	Height       int
	Instructions []Instruction
}

// Instruction 是所有 CPCL 指令的通用接口。
type Instruction interface {
	Kind() string
}

// TextInstruction 对应 TEXT 指令。
type TextInstruction struct {
	X        int
	Y        int
	Rotation int
	Text     string
}

func (t TextInstruction) Kind() string { return "TEXT" }

// LineInstruction 对应 LINE 指令。
type LineInstruction struct {
	X1        int
	Y1        int
	X2        int
	Y2        int
	Thickness int
	Inverse   bool
}

func (l LineInstruction) Kind() string { return "LINE" }

// BoxInstruction 对应 BOX 指令。
type BoxInstruction struct {
	X         int
	Y         int
	Width     int
	Height    int
	Thickness int
	Inverse   bool
}

func (b BoxInstruction) Kind() string { return "BOX" }

// Barcode128Instruction 对应 BARCODE128 指令。
type Barcode128Instruction struct {
	X int
	Y int
	// 条码高度，单位像素。
	Height int
	// 模块宽度，单位像素。
	ModuleWidth int
	// 是否垂直条码（VBARCODE）。
	Vertical bool
	// 条码数据。
	Data string
}

func (b Barcode128Instruction) Kind() string { return "BARCODE128" }

// QRCodeInstruction 对应 QRCODE 指令。
type QRCodeInstruction struct {
	X          int
	Y          int
	ModuleSize int
	Data       string
}

func (q QRCodeInstruction) Kind() string { return "QRCODE" }

type sourceLine struct {
	no   int
	text string
}

// ParseCPCL 读取并解析常见可渲染 CPCL 指令。
func ParseCPCL(r io.Reader) (*Label, error) {
	scanner := bufio.NewScanner(r)
	label := &Label{
		// 给默认值，避免输入缺失页面定义时崩溃。
		Width:  600,
		Height: 400,
	}

	lines := make([]sourceLine, 0, 128)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		rawLine := strings.TrimSpace(stripInlineComment(scanner.Text()))
		if rawLine == "" || strings.HasPrefix(rawLine, "#") {
			continue
		}
		lines = append(lines, sourceLine{no: lineNo, text: rawLine})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("扫描输入失败: %w", err)
	}

	for idx := 0; idx < len(lines); idx++ {
		line := lines[idx]
		rawLine := line.text
		fields := strings.Fields(rawLine)
		cmd := strings.ToUpper(fields[0])

		switch cmd {
		case "!":
			// CPCL 头部: ! <offset> <hres> <vres> <height> <qty>
			if len(fields) >= 5 {
				if h, err := strconv.Atoi(fields[4]); err == nil && h > 0 {
					label.Height = h
				}
			}
		case "SIZE":
			if len(fields) != 3 {
				return nil, fmt.Errorf("line %d: SIZE 格式应为 SIZE <width> <height>", line.no)
			}
			w, err := strconv.Atoi(fields[1])
			if err != nil {
				return nil, fmt.Errorf("line %d: SIZE width 无效: %w", line.no, err)
			}
			h, err := strconv.Atoi(fields[2])
			if err != nil {
				return nil, fmt.Errorf("line %d: SIZE height 无效: %w", line.no, err)
			}
			label.Width = w
			label.Height = h
		case "PW", "PAGE-WIDTH":
			if len(fields) < 2 {
				return nil, fmt.Errorf("line %d: %s 格式应为 %s <width>", line.no, cmd, cmd)
			}
			w, err := strconv.Atoi(fields[1])
			if err != nil {
				return nil, fmt.Errorf("line %d: %s width 无效: %w", line.no, cmd, err)
			}
			label.Width = w
		case "PH", "PAGE-HEIGHT":
			if len(fields) < 2 {
				return nil, fmt.Errorf("line %d: %s 格式应为 %s <height>", line.no, cmd, cmd)
			}
			h, err := strconv.Atoi(fields[1])
			if err != nil {
				return nil, fmt.Errorf("line %d: %s height 无效: %w", line.no, cmd, err)
			}
			label.Height = h
		case "TEXT", "T":
			ins, err := parseTextInstruction(fields, 0)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", line.no, err)
			}
			label.Instructions = append(label.Instructions, ins)
		case "TEXT90", "T90", "VT":
			ins, err := parseTextInstruction(fields, 90)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", line.no, err)
			}
			label.Instructions = append(label.Instructions, ins)
		case "TEXT180", "T180":
			ins, err := parseTextInstruction(fields, 180)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", line.no, err)
			}
			label.Instructions = append(label.Instructions, ins)
		case "TEXT270", "T270":
			ins, err := parseTextInstruction(fields, 270)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", line.no, err)
			}
			label.Instructions = append(label.Instructions, ins)
		case "LINE", "INVERSE-LINE":
			if len(fields) != 5 && len(fields) != 6 {
				return nil, fmt.Errorf("line %d: %s 格式应为 %s <x1> <y1> <x2> <y2> [thickness]", line.no, cmd, cmd)
			}
			x1, y1, x2, y2, err := parseInt4(fields[1], fields[2], fields[3], fields[4])
			if err != nil {
				return nil, fmt.Errorf("line %d: %s 参数无效: %w", line.no, cmd, err)
			}
			thickness := 1
			if len(fields) == 6 {
				thickness, err = strconv.Atoi(fields[5])
				if err != nil {
					return nil, fmt.Errorf("line %d: %s thickness 无效: %w", line.no, cmd, err)
				}
			}
			label.Instructions = append(label.Instructions, LineInstruction{
				X1: x1, Y1: y1, X2: x2, Y2: y2,
				Thickness: thickness,
				Inverse:   cmd == "INVERSE-LINE",
			})
		case "BOX", "INVERSE-BOX":
			if len(fields) != 5 && len(fields) != 6 {
				return nil, fmt.Errorf("line %d: %s 格式应为 %s <x> <y> <width> <height> [thickness]", line.no, cmd, cmd)
			}
			x, y, w, h, err := parseInt4(fields[1], fields[2], fields[3], fields[4])
			if err != nil {
				return nil, fmt.Errorf("line %d: %s 参数无效: %w", line.no, cmd, err)
			}
			thickness := 1
			if len(fields) == 6 {
				thickness, err = strconv.Atoi(fields[5])
				if err != nil {
					return nil, fmt.Errorf("line %d: %s thickness 无效: %w", line.no, cmd, err)
				}
			}
			label.Instructions = append(label.Instructions, BoxInstruction{
				X: x, Y: y, Width: w, Height: h,
				Thickness: thickness,
				Inverse:   cmd == "INVERSE-BOX",
			})
		case "BARCODE128":
			// BARCODE128 x y height data...
			if len(fields) < 5 {
				return nil, fmt.Errorf("line %d: BARCODE128 格式应为 BARCODE128 <x> <y> <height> <data>", line.no)
			}
			x, err := strconv.Atoi(fields[1])
			if err != nil {
				return nil, fmt.Errorf("line %d: BARCODE128 x 无效: %w", line.no, err)
			}
			y, err := strconv.Atoi(fields[2])
			if err != nil {
				return nil, fmt.Errorf("line %d: BARCODE128 y 无效: %w", line.no, err)
			}
			h, err := strconv.Atoi(fields[3])
			if err != nil {
				return nil, fmt.Errorf("line %d: BARCODE128 height 无效: %w", line.no, err)
			}
			data := strings.Join(fields[4:], " ")
			label.Instructions = append(label.Instructions, Barcode128Instruction{
				X: x, Y: y, Height: h, ModuleWidth: 2, Data: data,
			})
		case "BARCODE", "VBARCODE":
			// BARCODE 128 <wide> <narrow> <height> <x> <y> <data>
			// VBARCODE 128 <wide> <narrow> <height> <x> <y> <data>
			if len(fields) < 8 || fields[1] != "128" {
				return nil, fmt.Errorf("line %d: %s 目前仅支持 128，格式应为 %s 128 <wide> <narrow> <height> <x> <y> <data>", line.no, cmd, cmd)
			}
			moduleWidth, err := strconv.Atoi(fields[3])
			if err != nil {
				return nil, fmt.Errorf("line %d: %s narrow 无效: %w", line.no, cmd, err)
			}
			h, err := strconv.Atoi(fields[4])
			if err != nil {
				return nil, fmt.Errorf("line %d: %s height 无效: %w", line.no, cmd, err)
			}
			x, err := strconv.Atoi(fields[5])
			if err != nil {
				return nil, fmt.Errorf("line %d: %s x 无效: %w", line.no, cmd, err)
			}
			y, err := strconv.Atoi(fields[6])
			if err != nil {
				return nil, fmt.Errorf("line %d: %s y 无效: %w", line.no, cmd, err)
			}
			data := strings.Join(fields[7:], " ")
			label.Instructions = append(label.Instructions, Barcode128Instruction{
				X: x, Y: y, Height: h, ModuleWidth: moduleWidth, Data: data, Vertical: cmd == "VBARCODE",
			})
		case "QRCODE":
			ins, nextIdx, err := parseQRCodeInstruction(lines, idx)
			if err != nil {
				return nil, err
			}
			label.Instructions = append(label.Instructions, ins)
			idx = nextIdx
		case "FORM", "PRINT", "END", "CLS", "SETMAG", "SETLP", "LEFT", "RIGHT", "CENTER", "BARCODE-TEXT", "COUNTRY", "CODEPAGE":
			// 控制类指令：不参与绘制，保留兼容。
			continue
		default:
			return nil, fmt.Errorf("line %d: 不支持的指令 %q", line.no, fields[0])
		}
	}
	return label, nil
}

func parseInt4(a, b, c, d string) (int, int, int, int, error) {
	i1, err := strconv.Atoi(a)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	i2, err := strconv.Atoi(b)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	i3, err := strconv.Atoi(c)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	i4, err := strconv.Atoi(d)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return i1, i2, i3, i4, nil
}

func stripInlineComment(line string) string {
	commentPos := strings.Index(line, "#")
	if commentPos < 0 {
		return line
	}
	return line[:commentPos]
}

func parseTextInstruction(fields []string, rotation int) (TextInstruction, error) {
	// 兼容两种格式:
	// 1) TEXT x y content...
	// 2) TEXT font size x y content...
	if len(fields) < 4 {
		return TextInstruction{}, fmt.Errorf("%s 参数不足", fields[0])
	}

	// 简写格式：TEXT x y content...
	if _, errX := strconv.Atoi(fields[1]); errX == nil {
		if _, errY := strconv.Atoi(fields[2]); errY == nil {
			x, _ := strconv.Atoi(fields[1])
			y, _ := strconv.Atoi(fields[2])
			return TextInstruction{
				X: x, Y: y, Rotation: rotation, Text: strings.Join(fields[3:], " "),
			}, nil
		}
	}

	// CPCL 常见格式：TEXT/T font size x y content...
	if len(fields) < 6 {
		return TextInstruction{}, fmt.Errorf("%s 格式应为 %s <x> <y> <content> 或 %s <font> <size> <x> <y> <content>", fields[0], fields[0], fields[0])
	}
	x, err := strconv.Atoi(fields[3])
	if err != nil {
		return TextInstruction{}, fmt.Errorf("%s x 无效: %w", fields[0], err)
	}
	y, err := strconv.Atoi(fields[4])
	if err != nil {
		return TextInstruction{}, fmt.Errorf("%s y 无效: %w", fields[0], err)
	}
	return TextInstruction{
		X: x, Y: y, Rotation: rotation, Text: strings.Join(fields[5:], " "),
	}, nil
}

func parseQRCodeInstruction(lines []sourceLine, idx int) (QRCodeInstruction, int, error) {
	line := lines[idx]
	fields := strings.Fields(line.text)
	if len(fields) < 3 {
		return QRCodeInstruction{}, idx, fmt.Errorf("line %d: QRCODE 参数不足", line.no)
	}

	x, err := strconv.Atoi(fields[1])
	if err != nil {
		return QRCodeInstruction{}, idx, fmt.Errorf("line %d: QRCODE x 无效: %w", line.no, err)
	}
	y, err := strconv.Atoi(fields[2])
	if err != nil {
		return QRCodeInstruction{}, idx, fmt.Errorf("line %d: QRCODE y 无效: %w", line.no, err)
	}

	// 单行简写：QRCODE x y moduleSize data...
	if len(fields) >= 5 {
		if moduleSize, errSize := strconv.Atoi(fields[3]); errSize == nil {
			return QRCodeInstruction{
				X: x, Y: y, ModuleSize: moduleSize, Data: strings.Join(fields[4:], " "),
			}, idx, nil
		}
	}

	// CPCL 块格式：
	// QRCODE x y M 2 U 6
	// MA,content
	// ENDQR
	moduleSize := extractLastInt(fields, 6)
	dataParts := make([]string, 0, 2)
	nextIdx := idx
	for i := idx + 1; i < len(lines); i++ {
		nextIdx = i
		lineText := strings.TrimSpace(lines[i].text)
		lineUpper := strings.ToUpper(lineText)
		if lineUpper == "ENDQR" {
			break
		}
		if commaIdx := strings.Index(lineText, ","); commaIdx > 0 {
			// 兼容 MA,<data> / M1,<data> 等格式。
			payload := strings.TrimSpace(lineText[commaIdx+1:])
			if payload != "" {
				dataParts = append(dataParts, payload)
			}
		}
	}

	if len(dataParts) == 0 {
		return QRCodeInstruction{}, idx, fmt.Errorf("line %d: QRCODE 未找到有效数据段(MA,<data>)", line.no)
	}
	return QRCodeInstruction{
		X: x, Y: y, ModuleSize: moduleSize, Data: strings.Join(dataParts, ""),
	}, nextIdx, nil
}

func extractLastInt(fields []string, defaultValue int) int {
	for i := len(fields) - 1; i >= 0; i-- {
		if v, err := strconv.Atoi(fields[i]); err == nil {
			return v
		}
	}
	return defaultValue
}
