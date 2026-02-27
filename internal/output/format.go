package output

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

// Table prints a formatted table to stdout.
func Table(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Header
	fmt.Fprintln(w, strings.Join(headers, "\t"))

	// Separator
	sep := make([]string, len(headers))
	for i, h := range headers {
		sep[i] = strings.Repeat("-", len(h))
	}
	fmt.Fprintln(w, strings.Join(sep, "\t"))

	// Rows
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}

	w.Flush()
}

// StatusColor returns the status string with ANSI color codes.
func StatusColor(status string) string {
	switch status {
	case "active", "online":
		return "\033[32m" + status + "\033[0m" // green
	case "failed", "offline":
		return "\033[31m" + status + "\033[0m" // red
	case "deploying", "health_checking", "building", "cloning":
		return "\033[33m" + status + "\033[0m" // yellow
	case "pending", "queued":
		return "\033[90m" + status + "\033[0m" // dim
	case "superseded", "rolled_back", "cancelled":
		return "\033[90m" + status + "\033[0m" // dim
	default:
		return status
	}
}

// TimeSince parses an ISO timestamp and returns a human-readable relative time.
func TimeSince(iso string) string {
	if iso == "" {
		return "-"
	}

	// Try common ISO formats.
	var t time.Time
	var err error
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.000000",
	} {
		t, err = time.Parse(layout, iso)
		if err == nil {
			break
		}
	}
	if err != nil {
		return iso
	}

	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// ShortID returns the first 8 characters of a UUID.
func ShortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
