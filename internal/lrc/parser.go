package lrc

import (
	"bufio"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Line represents a single timestamped lyric line.
type Line struct {
	TimeMS int
	Text   string
}

// Document holds the parsed result of an LRC string.
type Document struct {
	Tags  map[string]string
	Lines []Line
}

var (
	// timestampRe matches [mm:ss], [mm:ss.xx], [mm:ss.xxx]
	timestampRe = regexp.MustCompile(`^\[(\d{1,3}):([0-5]\d)(?:\.(\d{1,3}))?\]`)
	// tagRe matches [key:value] metadata tags
	tagRe = regexp.MustCompile(`^\[([A-Za-z][A-Za-z0-9_-]*):(.*)\]$`)
)

// Parse parses an LRC-formatted string (as returned by LRCLIB syncedLyrics)
// into a Document. Returns an error if the input has no valid timestamped lines,
// contains invalid UTF-8, or has a malformed offset tag.
func Parse(input string) (Document, error) {
	if !utf8.ValidString(input) {
		return Document{}, fmt.Errorf("lrc: input contains invalid UTF-8")
	}

	doc := Document{
		Tags: make(map[string]string),
	}

	offsetMS := 0
	hasTimestampedLines := false

	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := scanner.Text()
		// Normalize CRLF
		line = strings.TrimRight(line, "\r")

		if line == "" {
			continue
		}

		// Try to extract one or more leading timestamps
		timestamps, rest, isTimestamped := extractTimestamps(line)
		if isTimestamped {
			hasTimestampedLines = true
			for _, ts := range timestamps {
				doc.Lines = append(doc.Lines, Line{
					TimeMS: ts,
					Text:   rest,
				})
			}
			continue
		}

		// Try tag
		if m := tagRe.FindStringSubmatch(line); m != nil {
			key := strings.ToLower(strings.TrimSpace(m[1]))
			val := strings.TrimSpace(m[2])
			if key == "offset" {
				ms, err := parseOffset(val)
				if err != nil {
					return Document{}, fmt.Errorf("lrc: invalid offset %q: %w", val, err)
				}
				offsetMS = ms
			}
			doc.Tags[key] = val
		}
		// Unknown non-tag, non-timestamp lines are silently ignored
	}

	if !hasTimestampedLines {
		return Document{}, fmt.Errorf("lrc: no timestamped lyric lines found")
	}

	// Apply offset and clamp
	for i := range doc.Lines {
		t := doc.Lines[i].TimeMS + offsetMS
		if t < 0 {
			t = 0
		}
		doc.Lines[i].TimeMS = t
	}

	// Stable sort by TimeMS
	sort.SliceStable(doc.Lines, func(i, j int) bool {
		return doc.Lines[i].TimeMS < doc.Lines[j].TimeMS
	})

	return doc, nil
}

// extractTimestamps extracts all leading [mm:ss.xxx] tokens from a line.
// Returns the list of timestamps in ms, the remaining text, and whether any
// timestamps were found.
func extractTimestamps(line string) ([]int, string, bool) {
	var timestamps []int
	rest := line

	for {
		m := timestampRe.FindStringSubmatch(rest)
		if m == nil {
			break
		}
		ms, err := parseTimestamp(m[1], m[2], m[3])
		if err != nil {
			// malformed timestamp — not a timestamp line
			return nil, line, false
		}
		timestamps = append(timestamps, ms)
		rest = rest[len(m[0]):]
	}

	if len(timestamps) == 0 {
		return nil, line, false
	}
	return timestamps, rest, true
}

// parseTimestamp converts mm, ss, frac strings to milliseconds.
func parseTimestamp(mm, ss, frac string) (int, error) {
	minutes, err := strconv.Atoi(mm)
	if err != nil {
		return 0, fmt.Errorf("invalid minutes %q", mm)
	}
	seconds, err := strconv.Atoi(ss)
	if err != nil {
		return 0, fmt.Errorf("invalid seconds %q", ss)
	}
	if seconds > 59 {
		return 0, fmt.Errorf("seconds out of range: %d", seconds)
	}

	ms := (minutes*60+seconds) * 1000

	if frac != "" {
		if len(frac) > 3 {
			return 0, fmt.Errorf("fraction too long: %q", frac)
		}
		// Pad or truncate to 3 digits
		for len(frac) < 3 {
			frac += "0"
		}
		fracMS, err := strconv.Atoi(frac)
		if err != nil {
			return 0, fmt.Errorf("invalid fraction %q", frac)
		}
		ms += fracMS
	}

	return ms, nil
}

// parseOffset parses an LRC [offset:+250] or [offset:-250] value to milliseconds.
func parseOffset(val string) (int, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, fmt.Errorf("empty offset value")
	}
	ms, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("non-integer offset %q: %w", val, err)
	}
	return ms, nil
}
