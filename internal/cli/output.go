package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
)

// RenderTable prints headers and rows as a tab-aligned table to stdout.
func RenderTable(headers []string, rows [][]string) {
	RenderTableTo(os.Stdout, headers, rows)
}

// RenderTableTo prints a tab-aligned table to the given writer.
func RenderTableTo(w io.Writer, headers []string, rows [][]string) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "No results.")
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(tw, "\t")
		}
		fmt.Fprint(tw, h)
	}
	fmt.Fprintln(tw)

	for _, row := range rows {
		for i, col := range row {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, col)
		}
		fmt.Fprintln(tw)
	}
	tw.Flush()
}

// RenderJSON prints data as indented JSON to stdout.
func RenderJSON(data any) {
	RenderJSONTo(os.Stdout, data)
}

// RenderJSONTo prints data as indented JSON to the given writer.
func RenderJSONTo(w io.Writer, data any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

// Truncate shortens a string to maxLen, appending "..." if truncated.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// FormatTime formats a time string for table display (first 19 chars of ISO format).
func FormatTime(t string) string {
	if len(t) > 19 {
		return t[:19]
	}
	return t
}
