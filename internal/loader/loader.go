package loader

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrBinaryFile is returned when a file appears to contain binary content.
var ErrBinaryFile = fmt.Errorf("file appears to be binary")

// isBinary reads the first 8 KB of a file and returns true if it contains
// null bytes or a high ratio of non-printable bytes — standard heuristic
// used by git, file(1), and similar tools.
func isBinary(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return false, err
	}
	buf = buf[:n]

	// Null byte → binary
	if bytes.IndexByte(buf, 0x00) >= 0 {
		return true, nil
	}

	// >30 % non-printable, non-whitespace bytes → binary
	nonPrint := 0
	for _, b := range buf {
		if b < 0x09 || (b > 0x0d && b < 0x20) || b == 0x7f {
			nonPrint++
		}
	}
	return float64(nonPrint)/float64(len(buf)) > 0.30, nil
}

// RawDocument is the output of loading a file.
type RawDocument struct {
	Path    string
	Title   string
	DocType string
	Content string // full extracted text
}

// DocumentLoader can load a file and return its text.
type DocumentLoader interface {
	Load(path string) (*RawDocument, error)
	Supports(ext string) bool
}

var registry []DocumentLoader

func init() {
	registry = []DocumentLoader{
		&PDFLoader{},
		&DOCXLoader{},
		&MarkdownLoader{},
		&TXTLoader{},
	}
}

// knownBinaryFormats are file extensions for formats that are inherently binary
// but have dedicated loaders that know how to parse them.
var knownBinaryFormats = map[string]bool{
	".pdf":  true,
	".docx": true,
}

// Load dispatches to the correct loader by file extension.
// Returns ErrBinaryFile (non-fatal) if the file looks like a binary.
func Load(path string) (*RawDocument, error) {
	ext := strings.ToLower(filepath.Ext(path))

	// Skip binary detection for formats that are inherently binary but have
	// dedicated parsers (PDF, DOCX).
	if !knownBinaryFormats[ext] {
		binary, err := isBinary(path)
		if err != nil {
			return nil, err
		}
		if binary {
			return nil, fmt.Errorf("%w: %s", ErrBinaryFile, path)
		}
	}

	for _, l := range registry {
		if l.Supports(ext) {
			return l.Load(path)
		}
	}
	return nil, fmt.Errorf("no loader for extension: %s", ext)
}

// SupportedExtensions returns all supported file extensions.
func SupportedExtensions() []string {
	var exts []string
	for _, l := range registry {
		for _, ext := range []string{".pdf", ".docx", ".md", ".txt"} {
			if l.Supports(ext) {
				exts = append(exts, ext)
			}
		}
	}
	return exts
}
