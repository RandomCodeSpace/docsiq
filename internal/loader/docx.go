package loader

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type DOCXLoader struct{}

func (l *DOCXLoader) Supports(ext string) bool { return ext == ".docx" }

func (l *DOCXLoader) Load(path string) (*RawDocument, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("docx open: %w", err)
	}
	defer r.Close()

	var content string
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			content, err = extractDocxText(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			break
		}
	}

	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("docx extract: no text content found in %s", path)
	}

	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return &RawDocument{
		Path:    path,
		Title:   title,
		DocType: "docx",
		Content: content,
	}, nil
}

func extractDocxText(r interface{ Read([]byte) (int, error) }) (string, error) {
	dec := xml.NewDecoder(r)
	var sb strings.Builder
	inText := false
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return sb.String(), fmt.Errorf("docx xml decode: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inText = true
			} else if t.Name.Local == "p" {
				sb.WriteString("\n")
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inText = false
			}
		case xml.CharData:
			if inText {
				sb.Write(t)
			}
		}
	}
	return sb.String(), nil
}
