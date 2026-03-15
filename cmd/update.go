package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the CLI to the latest version",
	RunE:  runUpdate,
}

func runUpdate(cmd *cobra.Command, args []string) error {
	baseURL := cfg.BaseURL + "/api/install/cli"

	// 1. Check latest version
	fmt.Print("Checking for updates... ")
	resp, err := http.Get(baseURL + "/latest/version")
	if err != nil {
		return fmt.Errorf("checking version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("version check failed (status %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
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
	archiveResp, err := http.Get(archiveURL)
	if err != nil {
		return fmt.Errorf("downloading archive: %w", err)
	}
	defer archiveResp.Body.Close()

	if archiveResp.StatusCode != 200 {
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
	if _, err := io.Copy(io.MultiWriter(f, hasher), archiveResp.Body); err != nil {
		f.Close()
		return fmt.Errorf("saving archive: %w", err)
	}
	f.Close()
	actualHash := hex.EncodeToString(hasher.Sum(nil))
	fmt.Println("done")

	// 4. Verify checksum
	fmt.Print("Verifying checksum... ")
	checksumResp, err := http.Get(baseURL + "/latest/download/checksums.txt")
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}
	defer checksumResp.Body.Close()

	checksumBody, err := io.ReadAll(checksumResp.Body)
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
		if filepath.Base(hdr.Name) == targetName {
			return io.ReadAll(tr)
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
		if filepath.Base(f.Name) == targetName {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("binary %q not found in archive", targetName)
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
