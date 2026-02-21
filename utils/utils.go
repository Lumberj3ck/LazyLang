package utils

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func GetProjectPath() string {
	d, err := os.UserHomeDir()

	if err != nil {
		d = "."
	}
	return filepath.Join(d, ".config", "lazylang")
}

var (
	bufSize = 1024 * 64 // Size of the buffer used for downloading the model
)

func DownloadModel(ctx context.Context, url string, dir string, downloadReport chan int64) error {
	// Create HTTP client
	client := http.Client{
		Timeout: time.Minute * 20,
	}

	// Create voices directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create voices directory: %w", err)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		model := filepath.Base(url)
		return fmt.Errorf("%s: %s", model, resp.Status)
	}

	// If output file exists and is the same size as the model, skip
	path := filepath.Join(dir, filepath.Base(url))
	if info, err := os.Stat(path); err == nil && info.Size() == resp.ContentLength {
		log.Printf("Skipping %s as it already exists", url)
		return nil
	}

	// Create file
	w, err := os.Create(path)
	if err != nil {
		return err
	}
	defer w.Close()

	// Progressively download the model
	data := make([]byte, bufSize)
	count := int64(0)
	ticker := time.NewTicker(time.Millisecond * 500)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if len(downloadReport) == cap(downloadReport) {
				<-downloadReport
			}
			downloadReport <- count * 100 / resp.ContentLength
		default:
			// Read body
			n, readErr := resp.Body.Read(data)
			if n > 0 {
				k, err := w.Write(data[:n])
				if err != nil {
					return err
				}
				count += int64(k)
			}

			log.Println("New data")
			if readErr != nil {
				return err
			}
		}
	}
}
