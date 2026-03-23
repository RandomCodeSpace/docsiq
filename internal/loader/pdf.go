package loader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmc/langchaingo/documentloaders"
)

type PDFLoader struct{}

func (l *PDFLoader) Supports(ext string) bool { return ext == ".pdf" }

func (l *PDFLoader) Load(path string) (*RawDocument, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("pdf open: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("pdf stat: %w", err)
	}

	loader := documentloaders.NewPDF(f, info.Size())
	docs, err := loader.Load(context.Background())
	if err != nil {
		return nil, fmt.Errorf("pdf load: %w", err)
	}

	var sb strings.Builder
	for _, doc := range docs {
		sb.WriteString(doc.PageContent)
		sb.WriteString("\n")
	}

	content := sb.String()
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("pdf extract: no text content found in %s (may be image-only)", path)
	}

	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return &RawDocument{
		Path:    path,
		Title:   title,
		DocType: "pdf",
		Content: content,
	}, nil
}
