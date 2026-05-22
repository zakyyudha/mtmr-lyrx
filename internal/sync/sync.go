package sync

import (
	"sort"

	"github.com/zakyyudha/mtmr-lyrx/internal/lrc"
)

// ActiveLine returns the index of the currently active lyric line given
// a playback position and timing offset (both in milliseconds).
// Returns -1 if the document is empty or position is before the first line.
func ActiveLine(doc lrc.Document, positionMS int, offsetMS int) int {
	if len(doc.Lines) == 0 {
		return -1
	}

	effective := positionMS + offsetMS

	// Binary search: find first line whose TimeMS > effective.
	// The active line is the one just before that.
	idx := sort.Search(len(doc.Lines), func(i int) bool {
		return doc.Lines[i].TimeMS > effective
	}) - 1

	if idx < 0 {
		return -1
	}
	return idx
}

// ActiveText returns the text of the currently active lyric line.
// Returns placeholder when no line is active or the active line has empty text.
func ActiveText(doc lrc.Document, positionMS int, offsetMS int, placeholder string) string {
	idx := ActiveLine(doc, positionMS, offsetMS)
	if idx < 0 {
		return placeholder
	}
	text := doc.Lines[idx].Text
	if text == "" {
		return placeholder
	}
	return text
}
