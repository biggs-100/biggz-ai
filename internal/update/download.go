package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DownloadAndExtract downloads an archive from url, extracts the binary named
// binaryName from it, and writes the binary to destDir. It supports tar.gz
// (Unix) and zip (Windows) archive formats, detecting the format from the URL
// extension.
//
// Returns the full path to the extracted binary.
func DownloadAndExtract(ctx context.Context, url, destDir, binaryName string) (string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("create dest dir: %w", err)
	}

	// Download to a temp file.
	tmpFile, err := os.CreateTemp("", "biggz-download-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("create download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return "", fmt.Errorf("download: status %d", resp.StatusCode)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("save download: %w", err)
	}
	tmpFile.Close()

	// Reopen for extraction.
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("read temp file: %w", err)
	}

	// Detect archive format from URL.
	switch {
	case strings.HasSuffix(strings.ToLower(url), ".tar.gz") || strings.HasSuffix(strings.ToLower(url), ".tgz"):
		return extractTarGz(data, destDir, binaryName)
	case strings.HasSuffix(strings.ToLower(url), ".zip"):
		return extractZip(data, destDir, binaryName)
	default:
		// Try tar.gz as default.
		path, err := extractTarGz(data, destDir, binaryName)
		if err == nil {
			return path, nil
		}
		return "", fmt.Errorf("unsupported archive format: %s", url)
	}
}

// extractTarGz extracts a binary named binaryName from a gzipped tar archive.
func extractTarGz(data []byte, destDir, binaryName string) (string, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar read: %w", err)
		}

		// Look for the binary by name (last path component).
		name := filepath.Base(header.Name)
		if name != binaryName {
			continue
		}

		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeSymlink {
			continue
		}

		destPath := filepath.Join(destDir, binaryName)
		outFile, err := os.Create(destPath)
		if err != nil {
			return "", fmt.Errorf("create binary file: %w", err)
		}

		if _, err := io.Copy(outFile, tr); err != nil {
			outFile.Close()
			return "", fmt.Errorf("write binary: %w", err)
		}
		outFile.Close()

		// Preserve executable permission.
		if header.Mode&0111 != 0 {
			os.Chmod(destPath, 0755)
		}

		return destPath, nil
	}

	return "", fmt.Errorf("binary %q not found in archive", binaryName)
}

// extractZip extracts a binary named binaryName from a zip archive.
func extractZip(data []byte, destDir, binaryName string) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("zip reader: %w", err)
	}

	for _, f := range zr.File {
		name := filepath.Base(f.Name)
		if name != binaryName {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open zip entry: %w", err)
		}
		defer rc.Close()

		destPath := filepath.Join(destDir, binaryName)
		outFile, err := os.Create(destPath)
		if err != nil {
			return "", fmt.Errorf("create binary file: %w", err)
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			outFile.Close()
			return "", fmt.Errorf("write binary: %w", err)
		}
		outFile.Close()

		return destPath, nil
	}

	return "", fmt.Errorf("binary %q not found in archive", binaryName)
}

// ExtractArchive extracts a binary named binaryName from an archive in data.
// It auto-detects tar.gz and zip formats, trying tar.gz first, then zip.
// Returns the full path to the extracted binary in destDir.
func ExtractArchive(data []byte, destDir, binaryName string) (string, error) {
	path, err := extractTarGz(data, destDir, binaryName)
	if err == nil {
		return path, nil
	}
	path, err = extractZip(data, destDir, binaryName)
	if err == nil {
		return path, nil
	}
	return "", fmt.Errorf("binary %q not found in archive (tried tar.gz and zip)", binaryName)
}
