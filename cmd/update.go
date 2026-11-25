package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	maxUpdateArchiveBytes  = 256 * 1024 * 1024 // 256 MiB
	maxUpdateBinaryBytes   = 256 * 1024 * 1024 // 256 MiB
	maxUpdateMetadataBytes = 1 * 1024 * 1024   // 1 MiB
)

func validateUpdateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid update URL %q: %w", rawURL, err)
	}
	if u.Host == "" {
		return fmt.Errorf("invalid update URL %q: missing host", rawURL)
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return nil
	case "http":
		host := strings.ToLower(u.Hostname())
		if host == "localhost" {
			return nil
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return nil
		}
		return fmt.Errorf("update URL must use HTTPS (got %q)", rawURL)
	default:
		return fmt.Errorf("unsupported update URL scheme %q", u.Scheme)
	}
}

func newUpdateHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return validateUpdateURL(req.URL.String())
		},
	}
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the CLI to the latest version",
	RunE:  runUpdate,
}

func runUpdate(cmd *cobra.Command, args []string) error {
	baseURL := strings.TrimRight(cfg.BaseURL, "/") + "/api/install/cli"
	if err := validateUpdateURL(baseURL); err != nil {
		return err
	}

	metaClient := newUpdateHTTPClient(30 * time.Second)
	downloadClient := newUpdateHTTPClient(5 * time.Minute)

	// 1. Check latest version
	fmt.Print("Checking for updates... ")
	resp, err := metaClient.Get(baseURL + "/latest/version")
	if err != nil {
		return fmt.Errorf("checking version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("version check failed (status %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUpdateMetadataBytes))
	if err != nil {
		return fmt.Errorf("reading version: %w", err)
	}
	latest := strings.TrimSpace(string(body))

	currentVersion := strings.TrimPrefix(Version, "v")
	if currentVersion == latest {
		fmt.Printf("already up to date (v%s)\n", latest)
		return nil
	}

	fmt.Printf("v%s available (current: v%s)\n", latest, currentVersion)

	// 2. Determine archive name
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	var archiveName string
	if goos == "windows" {
		archiveName = fmt.Sprintf("clank_%s_%s.zip", goos, goarch)
	} else {
		archiveName = fmt.Sprintf("clank_%s_%s.tar.gz", goos, goarch)
	}

	// 3. Download archive and checksums
	fmt.Printf("Downloading %s... ", archiveName)

	archiveURL := baseURL + "/latest/download/" + archiveName
	archiveResp, err := downloadClient.Get(archiveURL)
	if err != nil {
		return fmt.Errorf("downloading archive: %w", err)
	}
	defer archiveResp.Body.Close()

	if archiveResp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed (status %d)", archiveResp.StatusCode)
	}

	tmpDir, err := os.MkdirTemp("", "clank-update-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, archiveName)
	f, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(f, hasher), io.LimitReader(archiveResp.Body, maxUpdateArchiveBytes+1))
	if err != nil {
		f.Close()
		return fmt.Errorf("saving archive: %w", err)
	}
	f.Close()
	if written > maxUpdateArchiveBytes {
		return fmt.Errorf("downloaded archive exceeds maximum allowed size (%d bytes)", maxUpdateArchiveBytes)
	}
	actualHash := hex.EncodeToString(hasher.Sum(nil))
	fmt.Println("done")

	// 4. Verify checksum
	fmt.Print("Verifying checksum... ")
	checksumResp, err := metaClient.Get(baseURL + "/latest/download/checksums.txt")
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}
	defer checksumResp.Body.Close()

	if checksumResp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading checksums failed (status %d)", checksumResp.StatusCode)
	}

	checksumBody, err := io.ReadAll(io.LimitReader(checksumResp.Body, maxUpdateMetadataBytes))
	if err != nil {
		return fmt.Errorf("reading checksums: %w", err)
	}

	expectedHash := ""
	for _, line := range strings.Split(string(checksumBody), "\n") {
		if strings.Contains(line, archiveName) {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				expectedHash = strings.TrimPrefix(parts[0], "*")
			}
			break
		}
	}

	if expectedHash == "" {
		return fmt.Errorf("archive not found in checksums.txt")
	}

	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}
	fmt.Println("OK")

	// 5. Extract binary
	fmt.Print("Installing... ")
	var binaryData []byte
	if goos == "windows" {
		binaryData, err = extractFromZip(archivePath, "clank.exe")
	} else {
		binaryData, err = extractFromTarGz(archivePath, "clank")
	}
	if err != nil {
		return fmt.Errorf("extracting binary: %w", err)
	}

	// 6. Replace current binary
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding current binary: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}

	// Write new binary to temp file next to current binary
	newPath := exePath + ".new"
	if err := os.WriteFile(newPath, binaryData, 0755); err != nil {
		return fmt.Errorf("writing new binary: %w", err)
	}

	// On Windows, can't replace a running binary — rename the old one first
	if goos == "windows" {
		oldPath := exePath + ".old"
		os.Remove(oldPath) // ignore error (may not exist)
		if err := os.Rename(exePath, oldPath); err != nil {
			os.Remove(newPath)
			return fmt.Errorf("backing up old binary: %w", err)
		}
		if err := os.Rename(newPath, exePath); err != nil {
			// Try to restore
			os.Rename(oldPath, exePath)
			return fmt.Errorf("replacing binary: %w", err)
		}
		os.Remove(oldPath) // best effort cleanup
	} else {
		if err := os.Rename(newPath, exePath); err != nil {
			os.Remove(newPath)
			return fmt.Errorf("replacing binary: %w", err)
		}
	}

	fmt.Println("done")
	fmt.Printf("Updated to v%s\n", latest)
	return nil
}

func extractFromTarGz(archivePath, targetName string) ([]byte, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(hdr.Name) == targetName && hdr.Typeflag == tar.TypeReg {
			return readBoundedBinary(tr, maxUpdateBinaryBytes)
		}
	}
	return nil, fmt.Errorf("binary %q not found in archive", targetName)
}

func extractFromZip(archivePath, targetName string) ([]byte, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	for _, f := range r.File {
		if filepath.Base(f.Name) == targetName && !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return readBoundedBinary(rc, maxUpdateBinaryBytes)
		}
	}
	return nil, fmt.Errorf("binary %q not found in archive", targetName)
}

func readBoundedBinary(r io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("extracted binary exceeds maximum allowed size (%d bytes)", maxBytes)
	}
	return data, nil
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
