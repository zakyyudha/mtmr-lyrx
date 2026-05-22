package marquee

import (
	"regexp"
	"strings"
	"unicode"
)

// Config controls marquee display behavior.
type Config struct {
	Width     int    // display cell width
	Separator string // separator between end and start of scrolling line
}

// Formatter formats lyric lines for Touch Bar display.
type Formatter struct {
	Config Config
}

// ansiRE matches ANSI escape sequences.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// StripANSI removes ANSI escape sequences from s.
func StripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// SingleLine replaces CR and LF with spaces.
func SingleLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// runeWidth returns the display cell width of a single rune.
// Wide characters (CJK, fullwidth) count as 2 cells; most others as 1.
// Control characters count as 0.
func runeWidth(r rune) int {
	if r < 32 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	// Wide Unicode ranges (simplified — covers common CJK and fullwidth)
	if isWide(r) {
		return 2
	}
	return 1
}

// isWide returns true for runes that occupy 2 terminal cells.
func isWide(r rune) bool {
	// Fullwidth and wide ranges per Unicode East Asian Width
	switch {
	case r >= 0x1100 && r <= 0x115F: // Hangul Jamo
		return true
	case r >= 0x2E80 && r <= 0x303E: // CJK Radicals, Kangxi, etc.
		return true
	case r >= 0x3041 && r <= 0x33BF: // Hiragana, Katakana, CJK symbols
		return true
	case r >= 0x33FF && r <= 0xA4CF: // CJK Unified Ideographs Extension A
		return true
	case r >= 0xA960 && r <= 0xA97F: // Hangul Jamo Extended-A
		return true
	case r >= 0xAC00 && r <= 0xD7FF: // Hangul Syllables
		return true
	case r >= 0xF900 && r <= 0xFAFF: // CJK Compatibility Ideographs
		return true
	case r >= 0xFE10 && r <= 0xFE1F: // Vertical forms
		return true
	case r >= 0xFE30 && r <= 0xFE6F: // CJK Compatibility Forms
		return true
	case r >= 0xFF01 && r <= 0xFF60: // Fullwidth Forms
		return true
	case r >= 0xFFE0 && r <= 0xFFE6: // Fullwidth Signs
		return true
	case r >= 0x1B000 && r <= 0x1B0FF: // Kana Supplement
		return true
	case r >= 0x1F004 && r <= 0x1F0CF: // Mahjong/Playing Cards
		return true
	case r >= 0x1F300 && r <= 0x1F9FF: // Misc Symbols, Emoticons, etc.
		return true
	case r >= 0x20000 && r <= 0x2FFFD: // CJK Unified Ideographs Extension B-F
		return true
	case r >= 0x30000 && r <= 0x3FFFD: // CJK Unified Ideographs Extension G+
		return true
	}
	// Combining marks have zero width
	if unicode.Is(unicode.Mn, r) {
		return false
	}
	return false
}

// StringWidth returns the display cell width of a string.
func StringWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

// CellSlice returns a substring of s starting at display cell `start`
// with at most `width` display cells. Pads with spaces if needed.
func CellSlice(s string, start int, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	var result []rune
	cell := 0
	resultCells := 0

	for _, r := range runes {
		rw := runeWidth(r)
		if cell < start {
			cell += rw
			continue
		}
		if resultCells+rw > width {
			break
		}
		result = append(result, r)
		resultCells += rw
	}

	// Pad right with spaces to fill width
	out := string(result)
	for resultCells < width {
		out += " "
		resultCells++
	}
	return out
}

// Frame returns the display string for the given tick.
// Short lines are padded to exactly Config.Width cells.
// Long lines scroll left, cycling through line+separator+line.
func (f Formatter) Frame(line string, tick int) string {
	if f.Config.Width <= 0 {
		return ""
	}

	// Sanitize
	line = StripANSI(line)
	line = SingleLine(line)

	lineWidth := StringWidth(line)

	if lineWidth <= f.Config.Width {
		// Short line: pad right to exactly Width cells
		return CellSlice(line, 0, f.Config.Width)
	}

	// Long line: build loop string = line + separator + line
	sep := f.Config.Separator
	if sep == "" {
		sep = " "
	}
	loop := line + sep + line
	loopWidth := StringWidth(loop)

	// Scroll position: advance by 1 cell per tick, wrap at loopWidth
	scrollPos := tick % loopWidth
	if scrollPos < 0 {
		scrollPos = 0
	}

	return CellSlice(loop, scrollPos, f.Config.Width)
}
