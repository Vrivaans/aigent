package rag

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/dslipak/pdf"
	"github.com/tmc/langchaingo/documentloaders"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/textsplitter"
)

// ParseAndSplitDocument loads a document using LangChaingo loaders (or direct parser for PDFs) and splits it into chunks.
// It accepts dynamic chunkSize and chunkOverlap, falling back to 500 and 50 if zero or negative.
func ParseAndSplitDocument(ctx context.Context, r io.Reader, filename string, chunkSize, chunkOverlap int) ([]schema.Document, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	
	// Apply default values if parameters are invalid/zero
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if chunkOverlap < 0 {
		chunkOverlap = 50
	}

	splitter := textsplitter.NewRecursiveCharacter(
		textsplitter.WithChunkSize(chunkSize),
		textsplitter.WithChunkOverlap(chunkOverlap),
	)

	if ext == ".pdf" {
		// Read entire stream into memory to satisfy ReaderAt
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("failed to read pdf upload content: %w", err)
		}
		
		readerAt := bytes.NewReader(data)
		pdfReader, err := pdf.NewReader(readerAt, int64(len(data)))
		if err != nil {
			return nil, fmt.Errorf("failed to open PDF: %w", err)
		}
		
		plainTextReader, err := pdfReader.GetPlainText()
		if err != nil {
			return nil, fmt.Errorf("failed to extract plain text from PDF: %w", err)
		}
		
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(plainTextReader); err != nil {
			return nil, fmt.Errorf("failed to read extracted PDF text: %w", err)
		}
		
		docs := []schema.Document{
			{
				PageContent: buf.String(),
				Metadata:    map[string]interface{}{"source": filename},
			},
		}
		
		splitDocs, err := textsplitter.SplitDocuments(splitter, docs)
		if err != nil {
			return nil, fmt.Errorf("failed to split PDF text: %w", err)
		}
		return splitDocs, nil
	}

	var loader documentloaders.Loader
	switch ext {
	case ".html", ".htm":
		loader = documentloaders.NewHTML(r)
	case ".txt", ".md", ".json":
		loader = documentloaders.NewText(r)
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}

	docs, err := loader.LoadAndSplit(ctx, splitter)
	if err != nil {
		return nil, fmt.Errorf("failed to load and split document: %w", err)
	}

	return docs, nil
}
