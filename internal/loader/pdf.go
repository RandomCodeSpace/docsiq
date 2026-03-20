package loader

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func init() {
	// Disable pdfcpu's config directory lookup to avoid "config not found"
	// errors when ~/.config/pdfcpu/ does not exist.
	model.ConfigPath = "disable"
}

type PDFLoader struct{}

func (l *PDFLoader) Supports(ext string) bool { return ext == ".pdf" }

func (l *PDFLoader) Load(path string) (*RawDocument, error) {
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed

	// Extract raw content streams to a temp directory
	tmpDir, err := os.MkdirTemp("", "pdfcpu-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	if err := api.ExtractContentFile(path, tmpDir, nil, conf); err != nil {
		return nil, fmt.Errorf("pdf extract content: %w", err)
	}

	// Read all extracted content stream files and parse text operators
	var sb strings.Builder
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		f, err := os.Open(filepath.Join(tmpDir, entry.Name()))
		if err != nil {
			continue
		}
		extractPDFText(f, &sb)
		f.Close()
	}

	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return &RawDocument{
		Path:    path,
		Title:   title,
		DocType: "pdf",
		Content: sb.String(),
	}, nil
}

// extractPDFText parses a PDF content stream and extracts text from Tj / TJ operators.
func extractPDFText(r io.Reader, sb *strings.Builder) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	inBT := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "BT":
			inBT = true
		case line == "ET":
			inBT = false
			sb.WriteString("\n")
		case inBT && strings.HasSuffix(line, " Tj"):
			// (text) Tj
			text := extractParenText(strings.TrimSuffix(line, " Tj"))
			sb.WriteString(text)
			sb.WriteString(" ")
		case inBT && strings.HasSuffix(line, " TJ"):
			// [(text) ...] TJ
			text := extractArrayText(strings.TrimSuffix(line, " TJ"))
			sb.WriteString(text)
			sb.WriteString(" ")
		case inBT && strings.HasSuffix(line, "'"):
			// (text) '  (move to next line and show)
			text := extractParenText(strings.TrimSuffix(line, "'"))
			sb.WriteString("\n")
			sb.WriteString(text)
			sb.WriteString(" ")
		}
	}
}

func extractParenText(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		return s[1 : len(s)-1]
	}
	return ""
}

func extractArrayText(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") {
		return ""
	}
	var sb strings.Builder
	inParen := false
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '(' && !inParen:
			inParen = true
			depth = 1
		case c == '(' && inParen:
			depth++
			sb.WriteByte(c)
		case c == ')' && inParen:
			depth--
			if depth == 0 {
				inParen = false
			} else {
				sb.WriteByte(c)
			}
		case inParen:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}
