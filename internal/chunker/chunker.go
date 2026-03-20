package chunker

import (
	"unicode/utf8"

	"github.com/tmc/langchaingo/textsplitter"
)

type Chunk struct {
	Index   int
	Content string
	Tokens  int
}

type Chunker struct {
	splitter textsplitter.RecursiveCharacter
}

func New(chunkSize, chunkOverlap int) *Chunker {
	return &Chunker{
		splitter: textsplitter.NewRecursiveCharacter(
			textsplitter.WithChunkSize(chunkSize),
			textsplitter.WithChunkOverlap(chunkOverlap),
			textsplitter.WithSeparators([]string{"\n\n", "\n", ". ", " ", ""}),
		),
	}
}

func (c *Chunker) Split(text string) []Chunk {
	parts, err := c.splitter.SplitText(text)
	if err != nil {
		return []Chunk{{Index: 0, Content: text, Tokens: estimateTokens(text)}}
	}
	chunks := make([]Chunk, len(parts))
	for i, p := range parts {
		chunks[i] = Chunk{Index: i, Content: p, Tokens: estimateTokens(p)}
	}
	return chunks
}

func estimateTokens(text string) int {
	return utf8.RuneCountInString(text) / 4
}
