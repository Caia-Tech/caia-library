package activities

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
	"go.temporal.io/sdk/activity"
)

func FetchDocumentActivity(ctx context.Context, url string) (workflows.FetchResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Fetching document", "url", url)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return workflows.FetchResult{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "CAIA-Library/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return workflows.FetchResult{}, fmt.Errorf("failed to fetch document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return workflows.FetchResult{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	content, err := io.ReadAll(io.LimitReader(resp.Body, 100*1024*1024)) // 100MB limit
	if err != nil {
		return workflows.FetchResult{}, fmt.Errorf("failed to read response: %w", err)
	}

	// Get content type from response header
	contentType := resp.Header.Get("Content-Type")
	
	logger.Info("Document fetched successfully", "url", url, "size", len(content), "contentType", contentType)
	return workflows.FetchResult{
		Content:     content,
		ContentType: contentType,
	}, nil
}