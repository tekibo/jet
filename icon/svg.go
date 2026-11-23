package icon

import (
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type SVGNode struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Content []byte     `xml:",innerxml"`
	Nodes   []SVGNode  `xml:",any"`
}

type ParsedPath struct {
	Data      string
	FillColor string
}

type ParsedSVG struct {
	Width          float64
	Height         float64
	ViewportWidth  float64
	ViewportHeight float64
	Paths          []ParsedPath
}

// IsSVG returns true if the file path has an .svg extension or contains SVG xml header.
func IsSVG(filePath string) bool {
	if strings.HasSuffix(strings.ToLower(filePath), ".svg") {
		return true
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}
	content := strings.ToLower(string(data[:min(len(data), 512)]))
	return strings.Contains(content, "<svg")
}

// ParseSVG parses basic SVG files extracting viewBox and path definitions.
func ParseSVG(filePath string) (*ParsedSVG, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read svg file: %w", err)
	}

	content := string(data)
	parsed := &ParsedSVG{
		Width:          24,
		Height:         24,
		ViewportWidth:  24,
		ViewportHeight: 24,
	}

	// Extract viewBox
	viewBoxRegex := regexp.MustCompile(`viewBox\s*=\s*["']([0-9.\s,-]+)["']`)
	if match := viewBoxRegex.FindStringSubmatch(content); len(match) > 1 {
		parts := strings.Fields(strings.ReplaceAll(match[1], ",", " "))
		if len(parts) == 4 {
			if w, err := strconv.ParseFloat(parts[2], 64); err == nil {
				parsed.ViewportWidth = w
				parsed.Width = w
			}
			if h, err := strconv.ParseFloat(parts[3], 64); err == nil {
				parsed.ViewportHeight = h
				parsed.Height = h
			}
		}
	}

	// Extract paths
	pathRegex := regexp.MustCompile(`<path\s+([^>]+)/?>`)
	attrDRegex := regexp.MustCompile(`d\s*=\s*["']([^"']+)["']`)
	attrFillRegex := regexp.MustCompile(`fill\s*=\s*["']([^"']+)["']`)

	matches := pathRegex.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		attrs := m[1]
		dMatch := attrDRegex.FindStringSubmatch(attrs)
		if len(dMatch) < 2 {
			continue
		}
		pathData := strings.TrimSpace(dMatch[1])
		fillColor := "#FF000000"

		fillMatch := attrFillRegex.FindStringSubmatch(attrs)
		if len(fillMatch) > 1 {
			rawFill := strings.TrimSpace(fillMatch[1])
			if rawFill != "" && rawFill != "none" {
				fillColor = NormalizeHexColor(rawFill)
			}
		}

		parsed.Paths = append(parsed.Paths, ParsedPath{
			Data:      pathData,
			FillColor: fillColor,
		})
	}

	return parsed, nil
}

// ToVectorDrawable converts parsed SVG paths into an Android VectorDrawable XML.
func (s *ParsedSVG) ToVectorDrawable(dpW, dpH float64) string {
	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
	sb.WriteString(fmt.Sprintf("<vector xmlns:android=\"http://schemas.android.com/apk/res/android\"\n"))
	sb.WriteString(fmt.Sprintf("    android:width=\"%.0fdp\"\n", dpW))
	sb.WriteString(fmt.Sprintf("    android:height=\"%.0fdp\"\n", dpH))
	sb.WriteString(fmt.Sprintf("    android:viewportWidth=\"%.1f\"\n", s.ViewportWidth))
	sb.WriteString(fmt.Sprintf("    android:viewportHeight=\"%.1f\">\n", s.ViewportHeight))

	for _, p := range s.Paths {
		sb.WriteString("    <path\n")
		sb.WriteString(fmt.Sprintf("        android:fillColor=\"%s\"\n", p.FillColor))
		sb.WriteString(fmt.Sprintf("        android:pathData=\"%s\" />\n", p.Data))
	}

	sb.WriteString("</vector>\n")
	return sb.String()
}

// NormalizeHexColor converts SVG colors like #fff, #1e88e5, or red into 8-digit Android ARGB (#AARRGGBB).
func NormalizeHexColor(col string) string {
	col = strings.TrimSpace(col)
	if strings.HasPrefix(col, "#") {
		hex := col[1:]
		if len(hex) == 3 {
			// #RGB -> #FFRRGGBB
			return fmt.Sprintf("#FF%c%c%c%c%c%c", hex[0], hex[0], hex[1], hex[1], hex[2], hex[2])
		}
		if len(hex) == 6 {
			// #RRGGBB -> #FFRRGGBB
			return "#FF" + strings.ToUpper(hex)
		}
		if len(hex) == 8 {
			return "#" + strings.ToUpper(hex)
		}
	}

	// Simple named fallbacks
	switch strings.ToLower(col) {
	case "white":
		return "#FFFFFFFF"
	case "black":
		return "#FF000000"
	case "transparent":
		return "#00000000"
	default:
		return "#FF000000"
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
