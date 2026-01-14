package tray

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"math"
	"runtime"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/mrcode/nightscout-tray/internal/models"
	"golang.org/x/image/font/gofont/goregular"
)

const (
	osWindows        = "windows"
	statusUrgentLow  = "urgent_low"
	statusUrgentHigh = "urgent_high"
	statusLow        = "low"
	statusHigh       = "high"
)

type IconGenerator struct {
	history []float64
}

func NewIconGenerator() *IconGenerator {
	return &IconGenerator{
		history: make([]float64, 0, 24),
	}
}

func (g *IconGenerator) AddHistory(val float64) {
	g.history = append(g.history, val)
	if len(g.history) > 24 {
		g.history = g.history[1:]
	}
}

func (g *IconGenerator) ClearHistory() {
	g.history = make([]float64, 0, 24)
}

func (g *IconGenerator) GenerateTooltip(status *models.GlucoseStatus, settings *models.Settings) string {
	var valueStr string
	if settings.Unit == "mmol/L" {
		valueStr = fmt.Sprintf("%.1f", status.ValueMmol)
	} else {
		valueStr = fmt.Sprintf("%d", status.Value)
	}

	if runtime.GOOS == osWindows {
		sparkline := g.generateCompactSparkline()
		if sparkline != "" {
			staleIndicator := ""
			if status.IsStale {
				staleIndicator = " ⚠"
			}
			return fmt.Sprintf("%s%s %s\n%s\n%s %s",
				valueStr, settings.Unit, status.Trend,
				sparkline,
				formatCompactStatus(status.Status),
				formatCompactDuration(status.StaleMinutes)+staleIndicator)
		}
		return fmt.Sprintf("%s%s %s\n%s %s",
			valueStr, settings.Unit, status.Trend,
			formatCompactStatus(status.Status),
			formatCompactDuration(status.StaleMinutes))
	}

	sparkline := g.generateMultiLineSparkline()
	tooltip := fmt.Sprintf("%s %s %s\n%s\nStatus: %s\nUpdated: %s ago",
		valueStr, settings.Unit, status.Trend,
		sparkline,
		formatStatus(status.Status),
		formatDuration(status.StaleMinutes))
	if status.IsStale {
		tooltip += "\n⚠️ No fresh data"
	}
	return tooltip
}

func (g *IconGenerator) GenerateIcon(text string, direction string, status *models.GlucoseStatus) []byte {
	// Wails v3 systray uses PNG on all platforms
	var width, height float64 = 32, 32
	radius := width / 4

	dc := gg.NewContext(int(width), int(height))
	dc.SetRGBA(0, 0, 0, 0)
	dc.Clear()

	bgHex := getStatusColor(status)
	r, ge, b := parseHexColor(bgHex)

	dc.SetRGB255(int(r), int(ge), int(b))
	dc.DrawRoundedRectangle(0, 0, width, height, radius)
	dc.Fill()

	brightness := (int(r)*299 + int(ge)*587 + int(b)*114) / 1000
	if brightness > 128 {
		dc.SetColor(color.Black)
	} else {
		dc.SetColor(color.White)
	}

	fontSize := height * 0.5
	if err := loadFont(dc, fontSize); err == nil {
		dc.DrawStringAnchored(text, width/2, height/2-2, 0.5, 0.5)
	}

	if direction != "" {
		arrowSize := height * 0.3
		drawArrow(dc, width/2, height-5, arrowSize, direction)
	}

	// Wails v3 uses PNG for systray icons on all platforms
	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil
	}
	return buf.Bytes()
}

func (g *IconGenerator) generateCompactSparkline() string {
	if len(g.history) < 2 {
		return ""
	}
	minVal, maxVal := g.getMinMax()
	rangeVal := maxVal - minVal
	if rangeVal == 0 {
		rangeVal = 1
	}

	var topLine, bottomLine bytes.Buffer
	topLine.WriteRune(' ')
	for _, val := range g.history {
		normalized := (val - minVal) / rangeVal
		height := normalized * 4.0
		var topChar, bottomChar rune
		if height >= 4 {
			topChar, bottomChar = '⣿', '⣿'
		} else if height >= 3.5 {
			topChar, bottomChar = '⣶', '⣿'
		} else if height >= 3 {
			topChar, bottomChar = '⣤', '⣿'
		} else if height >= 2.5 {
			topChar, bottomChar = '⣀', '⣿'
		} else if height >= 2 {
			topChar, bottomChar = '⠀', '⣿'
		} else if height >= 1.5 {
			topChar, bottomChar = '⠀', '⣶'
		} else if height >= 1 {
			topChar, bottomChar = '⠀', '⣤'
		} else if height >= 0.5 {
			topChar, bottomChar = '⠀', '⣀'
		} else {
			topChar, bottomChar = '⠀', '⣀'
		}
		topLine.WriteRune(topChar)
		bottomLine.WriteRune(bottomChar)
	}
	return topLine.String() + "\n" + bottomLine.String()
}

func (g *IconGenerator) generateMultiLineSparkline() string {
	if len(g.history) < 2 {
		return ""
	}
	height := 10
	minVal, maxVal := g.getMinMax()
	buffer := 10.0
	minVal = math.Max(0, minVal-buffer)
	maxVal += buffer
	rangeVal := maxVal - minVal

	blocks := []rune{'⠀', '⣀', '⣤', '⣶', '⣿'}
	subBlocksPerLine := 4.0


rows := make([][]rune, height)
	width := len(g.history)
	for i := 0; i < height; i++ {
	
rows[i] = make([]rune, width)
		for j := 0; j < width; j++ {
		
rows[i][j] = '⠀'
		}
	}

	for x, val := range g.history {
		normalized := (val - minVal) / rangeVal
		totalSubBlocks := normalized * float64(height) * subBlocksPerLine
		for y := 0; y < height; y++ {
			lineIdx := height - 1 - y
			lineStart := float64(y) * subBlocksPerLine
			lineEnd := float64(y+1) * subBlocksPerLine
			if totalSubBlocks >= lineEnd {
			
rows[lineIdx][x] = '⣿'
			} else if totalSubBlocks > lineStart {
				remainder := int(math.Round(totalSubBlocks - lineStart))
				if remainder < 0 { remainder = 0 }
				if remainder >= len(blocks) { remainder = len(blocks) - 1 }
			
rows[lineIdx][x] = blocks[remainder]
			}
		}
	}

	var result bytes.Buffer
	result.WriteString("\n")
	result.WriteString(fmt.Sprintf("Max: %.0f\n", maxVal))
	for i := 0; i < height; i++ {
		result.WriteString(string(rows[i]))
		result.WriteString("\n")
	}
	result.WriteString(fmt.Sprintf("Min: %.0f", minVal))
	return result.String()
}

func (g *IconGenerator) getMinMax() (float64, float64) {
	if len(g.history) == 0 { return 0, 0 }
	minV, maxV := g.history[0], g.history[0]
	for _, v := range g.history {
		if v < minV { minV = v }
		if v > maxV { maxV = v }
	}
	return minV, maxV
}

// Helpers

func formatStatus(status string) string {
	switch status {
	case statusUrgentLow: return "Urgent Low"
	case statusUrgentHigh: return "Urgent High"
	case statusLow: return "Low"
	case statusHigh: return "High"
	case "normal": return "In Range"
	default: return status
	}
}

func formatDuration(minutes int) string {
	if minutes < 1 { return "just now" }
	if minutes == 1 { return "1 minute" }
	if minutes < 60 { return fmt.Sprintf("%d minutes", minutes) }
	hours := minutes / 60
	if hours == 1 { return "1 hour" }
	return fmt.Sprintf("%d hours", hours)
}

func formatCompactStatus(status string) string {
	switch status {
	case statusUrgentLow: return "🔻URGENT"
	case statusUrgentHigh: return "🔺URGENT"
	case statusLow: return "↓Low"
	case statusHigh: return "↑High"
	case "normal": return "✓OK"
	default: return status
	}
}

func formatCompactDuration(minutes int) string {
	if minutes < 1 { return "now" }
	if minutes < 60 { return fmt.Sprintf("%dm", minutes) }
	return fmt.Sprintf("%dh", minutes/60)
}

func getStatusColor(status *models.GlucoseStatus) string {
	if status == nil { return "#808080" }
	if status.StaleMinutes > 7 { return "#9ca3af" }
	switch status.Status {
	case "urgent_low", "urgent_high": return "#ef4444"
	case statusLow: return "#f97316"
	case statusHigh: return "#facc15"
	default: return "#4ade80"
	}
}

func parseHexColor(hex string) (r, g, b byte) {
	if len(hex) == 7 && hex[0] == '#' {
		_, _ = fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	}
	return
}

func loadFont(dc *gg.Context, size float64) error {
	font, err := truetype.Parse(goregular.TTF)
	if err != nil { return err }
	face := truetype.NewFace(font, &truetype.Options{Size: size})
	dc.SetFontFace(face)
	return nil
}

func drawArrow(dc *gg.Context, x, y, size float64, direction string) {
	dc.Push()
	defer dc.Pop()
	dc.Translate(x, y)
	var angle float64
	switch direction {
	case "DoubleUp", "SingleUp": angle = 0
	case "FortyFiveUp": angle = 45
	case "Flat": angle = 90
	case "FortyFiveDown": angle = 135
	case "DoubleDown", "SingleDown": angle = 180
	default: return
	}
	dc.Rotate(gg.Radians(angle))
	if direction == "DoubleUp" || direction == "DoubleDown" {
		drawSingleArrow(dc, 0, -size/4, size*0.8)
		drawSingleArrow(dc, 0, size/4, size*0.8)
	} else {
		drawSingleArrow(dc, 0, 0, size)
	}
}

func drawSingleArrow(dc *gg.Context, ox, oy, s float64) {
	w := s * 0.5
	dc.NewSubPath()
	dc.MoveTo(ox, oy-s/2)
	dc.LineTo(ox+w/2, oy)
	dc.LineTo(ox+w/6, oy)
	dc.LineTo(ox+w/6, oy+s/2)
	dc.LineTo(ox-w/6, oy+s/2)
	dc.LineTo(ox-w/6, oy)
	dc.LineTo(ox-w/2, oy)
	dc.ClosePath()
	dc.Fill()
}

