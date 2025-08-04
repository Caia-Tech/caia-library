package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/caiatech/caia-library/pkg/extractor"
)

func main() {
	// Test PDF URL (W3C sample PDF)
	pdfURL := "https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf"

	// Download PDF
	fmt.Printf("Downloading PDF from %s...\n", pdfURL)
	resp, err := http.Get(pdfURL)
	if err != nil {
		fmt.Printf("Error downloading PDF: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading PDF: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Downloaded %d bytes\n", len(content))
	fmt.Printf("First 10 bytes: %v\n", content[:10])
	fmt.Printf("As string: %q\n", string(content[:10]))

	// Extract text
	engine := extractor.NewEngine()
	text, metadata, err := engine.Extract(context.Background(), content, "pdf")
	if err != nil {
		fmt.Printf("Error extracting text: %v\n", err)
		os.Exit(1)
	}

	// Display results
	fmt.Println("\n=== Metadata ===")
	for key, value := range metadata {
		fmt.Printf("%s: %s\n", key, value)
	}

	fmt.Println("\n=== Extracted Text (first 500 chars) ===")
	if len(text) > 500 {
		fmt.Println(text[:500] + "...")
	} else {
		fmt.Println(text)
	}

	fmt.Printf("\n\nTotal text length: %d characters\n", len(text))
}