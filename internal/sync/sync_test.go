package sync

import (
	"testing"

	"github.com/zakyyudha/mtmr-lyrx/internal/lrc"
)

func makeDoc(times []int, texts []string) lrc.Document {
	doc := lrc.Document{Tags: map[string]string{}}
	for i, t := range times {
		doc.Lines = append(doc.Lines, lrc.Line{TimeMS: t, Text: texts[i]})
	}
	return doc
}

func TestActiveLineEmptyDoc(t *testing.T) {
	doc := lrc.Document{Tags: map[string]string{}}
	if got := ActiveLine(doc, 1000, 0); got != -1 {
		t.Errorf("expected -1 for empty doc, got %d", got)
	}
}

func TestActiveLineBeforeFirst(t *testing.T) {
	doc := makeDoc([]int{5000, 10000}, []string{"A", "B"})
	if got := ActiveLine(doc, 0, 0); got != -1 {
		t.Errorf("expected -1 before first line, got %d", got)
	}
}

func TestActiveLineExactTimestamp(t *testing.T) {
	doc := makeDoc([]int{1000, 2000, 3000}, []string{"A", "B", "C"})
	if got := ActiveLine(doc, 2000, 0); got != 1 {
		t.Errorf("expected index 1 at exact timestamp 2000, got %d", got)
	}
}

func TestActiveLineBetweenLines(t *testing.T) {
	doc := makeDoc([]int{1000, 3000, 5000}, []string{"A", "B", "C"})
	if got := ActiveLine(doc, 2000, 0); got != 0 {
		t.Errorf("expected index 0 between 1000 and 3000, got %d", got)
	}
}

func TestActiveLinePastLast(t *testing.T) {
	doc := makeDoc([]int{1000, 2000}, []string{"A", "B"})
	if got := ActiveLine(doc, 99999, 0); got != 1 {
		t.Errorf("expected last index 1 past end, got %d", got)
	}
}

func TestActiveLinePositiveOffset(t *testing.T) {
	doc := makeDoc([]int{5000, 10000}, []string{"A", "B"})
	// position 4000 + offset 1500 = effective 5500 → line 0
	if got := ActiveLine(doc, 4000, 1500); got != 0 {
		t.Errorf("expected index 0 with positive offset, got %d", got)
	}
}

func TestActiveLineNegativeOffset(t *testing.T) {
	doc := makeDoc([]int{1000, 5000}, []string{"A", "B"})
	// position 5000 + offset -2000 = effective 3000 → line 0
	if got := ActiveLine(doc, 5000, -2000); got != 0 {
		t.Errorf("expected index 0 with negative offset, got %d", got)
	}
}

func TestActiveTextPlaceholder(t *testing.T) {
	doc := lrc.Document{Tags: map[string]string{}}
	got := ActiveText(doc, 0, 0, "♪")
	if got != "♪" {
		t.Errorf("expected placeholder ♪, got %q", got)
	}
}

func TestActiveTextEmptyLine(t *testing.T) {
	doc := makeDoc([]int{1000}, []string{""})
	got := ActiveText(doc, 1000, 0, "♪")
	if got != "♪" {
		t.Errorf("expected placeholder for empty line text, got %q", got)
	}
}

func TestActiveTextReturnsText(t *testing.T) {
	doc := makeDoc([]int{1000, 2000}, []string{"Hello", "World"})
	got := ActiveText(doc, 2000, 0, "♪")
	if got != "World" {
		t.Errorf("expected World, got %q", got)
	}
}
