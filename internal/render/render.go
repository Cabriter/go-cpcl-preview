package render

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"cpcl_test/internal/config"
	"cpcl_test/internal/parser"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/qr"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// RenderToPNG 将解析后的 Label 渲染为 PNG 文件。
func RenderToPNG(label *parser.Label, outputPath string, renderCfg config.RenderConfig, logger *log.Logger) error {
	if label.Width <= 0 || label.Height <= 0 {
		return fmt.Errorf("标签尺寸非法: width=%d height=%d", label.Width, label.Height)
	}

	canvas := image.NewRGBA(image.Rect(0, 0, label.Width, label.Height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: image.White}, image.Point{}, draw.Src)

	textFace, cleanup, err := loadTextFace(renderCfg, logger)
	if err != nil {
		return err
	}
	defer cleanup()

	for idx, ins := range label.Instructions {
		logger.Printf("开始渲染第 %d 条指令: %s", idx+1, ins.Kind())

		switch v := ins.(type) {
		case parser.TextInstruction:
			drawText(canvas, textFace, v.X, v.Y, v.Text, v.Rotation, image.Black)
		case parser.LineInstruction:
			drawLine(canvas, v.X1, v.Y1, v.X2, v.Y2, v.Thickness, v.Inverse, image.Black, image.White)
		case parser.BoxInstruction:
			drawBox(canvas, v, image.Black, image.White)
		case parser.Barcode128Instruction:
			moduleWidth := v.ModuleWidth
			if moduleWidth <= 0 {
				moduleWidth = renderCfg.BarcodeModuleWidth
			}
			if err := drawBarcode128(canvas, v.X, v.Y, v.Height, moduleWidth, v.Data, v.Vertical); err != nil {
				// 条码失败不影响其他元素渲染，但输出详细日志便于定位问题。
				logger.Printf("BARCODE128 渲染失败: x=%d y=%d height=%d moduleWidth=%d data=%q err=%v", v.X, v.Y, v.Height, moduleWidth, v.Data, err)
			}
		case parser.QRCodeInstruction:
			moduleSize := v.ModuleSize
			if moduleSize <= 0 {
				moduleSize = renderCfg.QRCodeModuleSize
			}
			if err := drawQRCode(canvas, v.X, v.Y, moduleSize, v.Data); err != nil {
				logger.Printf("QRCODE 渲染失败: x=%d y=%d moduleSize=%d data=%q err=%v", v.X, v.Y, moduleSize, v.Data, err)
			}
		default:
			logger.Printf("未知指令类型: %T", v)
		}
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建 PNG 文件失败: %w", err)
	}
	defer file.Close()

	if err := png.Encode(file, canvas); err != nil {
		return fmt.Errorf("编码 PNG 失败: %w", err)
	}
	return nil
}

func loadTextFace(renderCfg config.RenderConfig, logger *log.Logger) (font.Face, func(), error) {
	candidates := []string{
		renderCfg.FontPath,
		"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
		"/System/Library/Fonts/Supplemental/NISC18030.ttf",
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, path := range candidates {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		fontBytes, err := os.ReadFile(path)
		if err != nil {
			logger.Printf("加载字体文件失败: path=%s err=%v", path, err)
			continue
		}
		ft, err := opentype.Parse(fontBytes)
		if err != nil {
			logger.Printf("解析字体文件失败: path=%s err=%v", path, err)
			continue
		}
		face, err := opentype.NewFace(ft, &opentype.FaceOptions{
			Size:    renderCfg.FontSize,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err != nil {
			logger.Printf("创建字体 Face 失败: path=%s err=%v", path, err)
			continue
		}
		if !fontSupportsRune(face, '中') {
			_ = face.Close()
			logger.Printf("字体不支持中文，尝试下一个: path=%s", path)
			continue
		}
		logger.Printf("字体加载成功: %s", path)
		return face, func() { _ = face.Close() }, nil
	}
	logger.Printf("未找到可用中文字体，回退 basicfont")
	return basicfont.Face7x13, func() {}, nil
}

func drawText(img *image.RGBA, face font.Face, x, y int, text string, rotation int, c color.Color) {
	if rotation != 0 {
		drawTextRotated(img, face, x, y, text, rotation, c)
		return
	}
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

func drawTextRotated(img *image.RGBA, face font.Face, x, y int, text string, rotation int, c color.Color) {
	rotation = ((rotation%360)+360)%360
	if rotation == 0 {
		drawText(img, face, x, y, text, 0, c)
		return
	}

	ascent := face.Metrics().Ascent.Ceil()
	advance := font.MeasureString(face, text).Ceil()
	if ascent <= 0 {
		ascent = 12
	}
	if advance <= 0 {
		advance = len(text) * 8
	}

	tmpW := maxInt(advance+8, 16)
	tmpH := maxInt(ascent+8, 16)
	tmp := image.NewRGBA(image.Rect(0, 0, tmpW, tmpH))
	draw.Draw(tmp, tmp.Bounds(), &image.Uniform{C: color.Transparent}, image.Point{}, draw.Src)

	d := &font.Drawer{
		Dst:  tmp,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.P(2, ascent),
	}
	d.DrawString(text)

	switch rotation {
	case 90:
		blitImage(img, rotate90(tmp), x, y-ascent)
	case 180:
		blitImage(img, rotate180(tmp), x-advance, y-ascent)
	case 270:
		blitImage(img, rotate270(tmp), x, y)
	default:
		drawText(img, face, x, y, text, 0, c)
	}
}

func blitImage(dst *image.RGBA, src *image.RGBA, x, y int) {
	for sx := 0; sx < src.Bounds().Dx(); sx++ {
		for sy := 0; sy < src.Bounds().Dy(); sy++ {
			p := src.RGBAAt(src.Bounds().Min.X+sx, src.Bounds().Min.Y+sy)
			if p.A == 0 {
				continue
			}
			setPixelSafe(dst, x+sx, y+sy, p)
		}
	}
}

func rotate90(src *image.RGBA) *image.RGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			dst.Set(h-1-y, x, src.At(x, y))
		}
	}
	return dst
}

func rotate180(src *image.RGBA) *image.RGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			dst.Set(w-1-x, h-1-y, src.At(x, y))
		}
	}
	return dst
}

func rotate270(src *image.RGBA) *image.RGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			dst.Set(y, w-1-x, src.At(x, y))
		}
	}
	return dst
}

// drawLine 使用 Bresenham 算法绘制线段。
func drawLine(img *image.RGBA, x1, y1, x2, y2, thickness int, inverse bool, black, white color.Color) {
	if thickness <= 0 {
		thickness = 1
	}
	lineColor := black
	if inverse {
		lineColor = white
	}
	dx := abs(x2 - x1)
	dy := -abs(y2 - y1)
	sx := -1
	if x1 < x2 {
		sx = 1
	}
	sy := -1
	if y1 < y2 {
		sy = 1
	}
	err := dx + dy

	for {
		drawPointWithThickness(img, x1, y1, thickness, lineColor)
		if x1 == x2 && y1 == y2 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x1 += sx
		}
		if e2 <= dx {
			err += dx
			y1 += sy
		}
	}
}

func drawPointWithThickness(img *image.RGBA, cx, cy, thickness int, c color.Color) {
	half := thickness / 2
	for dx := -half; dx <= half; dx++ {
		for dy := -half; dy <= half; dy++ {
			setPixelSafe(img, cx+dx, cy+dy, c)
		}
	}
}

func drawBox(img *image.RGBA, box parser.BoxInstruction, black, white color.Color) {
	if box.Inverse {
		fillRect(img, box.X, box.Y, box.Width, box.Height, white)
		return
	}

	drawLine(img, box.X, box.Y, box.X+box.Width, box.Y, box.Thickness, false, black, white)
	drawLine(img, box.X, box.Y, box.X, box.Y+box.Height, box.Thickness, false, black, white)
	drawLine(img, box.X+box.Width, box.Y, box.X+box.Width, box.Y+box.Height, box.Thickness, false, black, white)
	drawLine(img, box.X, box.Y+box.Height, box.X+box.Width, box.Y+box.Height, box.Thickness, false, black, white)
}

func fillRect(img *image.RGBA, x, y, w, h int, c color.Color) {
	for px := x; px <= x+w; px++ {
		for py := y; py <= y+h; py++ {
			setPixelSafe(img, px, py, c)
		}
	}
}

func drawBarcode128(img *image.RGBA, x, y, height, moduleWidth int, data string, vertical bool) error {
	if height <= 0 {
		return fmt.Errorf("barcode height 必须大于 0")
	}
	if data == "" {
		return fmt.Errorf("barcode data 不能为空")
	}
	if moduleWidth <= 0 {
		moduleWidth = 2
	}

	code, err := code128.Encode(data)
	if err != nil {
		return fmt.Errorf("编码条码失败: %w", err)
	}
	targetW := code.Bounds().Dx() * moduleWidth
	scaled, err := barcode.Scale(code, targetW, height)
	if err != nil {
		return fmt.Errorf("条码缩放失败: %w", err)
	}
	scaledRGBA := toRGBA(scaled)
	if vertical {
		scaledRGBA = rotate270(scaledRGBA)
	}
	blitImage(img, scaledRGBA, x, y)
	return nil
}

func drawQRCode(img *image.RGBA, x, y, moduleSize int, data string) error {
	if moduleSize <= 0 {
		moduleSize = 6
	}
	if strings.TrimSpace(data) == "" {
		return fmt.Errorf("qrcode data 不能为空")
	}

	code, err := qr.Encode(data, qr.M, qr.Auto)
	if err != nil {
		return fmt.Errorf("生成二维码失败: %w", err)
	}
	targetW := code.Bounds().Dx() * moduleSize
	targetH := code.Bounds().Dy() * moduleSize
	scaled, err := barcode.Scale(code, targetW, targetH)
	if err != nil {
		return fmt.Errorf("二维码缩放失败: %w", err)
	}
	blitImage(img, toRGBA(scaled), x, y)
	return nil
}

func setPixelSafe(img *image.RGBA, x, y int, c color.Color) {
	if image.Pt(x, y).In(img.Bounds()) {
		img.Set(x, y, c)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func toRGBA(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

func fontSupportsRune(face font.Face, r rune) bool {
	_, _, ok := face.GlyphBounds(r)
	return ok
}
