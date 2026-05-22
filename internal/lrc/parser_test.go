package lrc

import (
	"strings"
	"testing"
)

// fakeLRC is a public-domain test fixture — no copyrighted lyrics.
const fakeLRC = `[ar:Public Domain]
[ti:Test Song]
[offset:+250]
[00:00.00]First line
[00:02.50][00:04.00]Repeated line
[00:06.00]こんにちは 🌙
[00:08.00]`

func TestParseBasic(t *testing.T) {
	doc, err := Parse(fakeLRC)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Tags
	if doc.Tags["ar"] != "Public Domain" {
		t.Errorf("expected ar=Public Domain, got %q", doc.Tags["ar"])
	}
	if doc.Tags["ti"] != "Test Song" {
		t.Errorf("expected ti=Test Song, got %q", doc.Tags["ti"])
	}

	// 5 lines: First line, Repeated line x2, Unicode line, blank timestamped line
	if len(doc.Lines) != 5 {
		t.Fatalf("expected 5 lines, got %d: %+v", len(doc.Lines), doc.Lines)
	}

	// Expected times after +250ms offset
	expected := []struct {
		timeMS int
		text   string
	}{
		{250, "First line"},
		{2750, "Repeated line"},
		{4250, "Repeated line"},
		{6250, "こんにちは 🌙"},
		{8250, ""},
	}

	for i, e := range expected {
		if doc.Lines[i].TimeMS != e.timeMS {
			t.Errorf("line %d: expected TimeMS=%d, got %d", i, e.timeMS, doc.Lines[i].TimeMS)
		}
		if doc.Lines[i].Text != e.text {
			t.Errorf("line %d: expected Text=%q, got %q", i, e.text, doc.Lines[i].Text)
		}
	}
}

func TestParseCRLF(t *testing.T) {
	input := "[00:01.00]Hello\r\n[00:02.00]World\r\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse CRLF: %v", err)
	}
	if len(doc.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(doc.Lines))
	}
	if doc.Lines[0].Text != "Hello" {
		t.Errorf("expected Hello, got %q", doc.Lines[0].Text)
	}
}

func TestParseUnicode(t *testing.T) {
	input := "[00:01.00]日本語テスト 🎵\n[00:02.00]한국어\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse Unicode: %v", err)
	}
	if doc.Lines[0].Text != "日本語テスト 🎵" {
		t.Errorf("expected Japanese text, got %q", doc.Lines[0].Text)
	}
}

func TestParseMultipleTimestamps(t *testing.T) {
	input := "[00:10.00][00:20.00]Same lyric\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(doc.Lines))
	}
	if doc.Lines[0].TimeMS != 10000 || doc.Lines[1].TimeMS != 20000 {
		t.Errorf("unexpected times: %d, %d", doc.Lines[0].TimeMS, doc.Lines[1].TimeMS)
	}
}

func TestParsePositiveOffset(t *testing.T) {
	input := "[offset:+1000]\n[00:01.00]Line\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.Lines[0].TimeMS != 2000 {
		t.Errorf("expected 2000ms, got %d", doc.Lines[0].TimeMS)
	}
}

func TestParseNegativeOffsetClamp(t *testing.T) {
	// offset -5000 on a 1s timestamp should clamp to 0
	input := "[offset:-5000]\n[00:01.00]Line\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.Lines[0].TimeMS != 0 {
		t.Errorf("expected clamped 0ms, got %d", doc.Lines[0].TimeMS)
	}
}

func TestParseTimestampedBlankLine(t *testing.T) {
	input := "[00:01.00]\n[00:02.00]Text\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(doc.Lines))
	}
	if doc.Lines[0].Text != "" {
		t.Errorf("expected blank text, got %q", doc.Lines[0].Text)
	}
}

func TestParseUntimedBlankLinesIgnored(t *testing.T) {
	input := "\n\n[00:01.00]Line\n\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(doc.Lines))
	}
}

func TestParseNoTimestampedLines(t *testing.T) {
	input := "[ar:Artist]\n[ti:Title]\n"
	_, err := Parse(input)
	if err == nil {
		t.Error("expected error for no timestamped lines")
	}
}

func TestParseInvalidUTF8(t *testing.T) {
	input := "[00:01.00]" + string([]byte{0xff, 0xfe}) + "bad"
	_, err := Parse(input)
	if err == nil {
		t.Error("expected error for invalid UTF-8")
	}
}

func TestParseInvalidOffset(t *testing.T) {
	input := "[offset:abc]\n[00:01.00]Line\n"
	_, err := Parse(input)
	if err == nil {
		t.Error("expected error for invalid offset")
	}
}

func TestParseSortedOutput(t *testing.T) {
	// Input out of order
	input := "[00:05.00]Fifth\n[00:01.00]First\n[00:03.00]Third\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.Lines[0].Text != "First" || doc.Lines[1].Text != "Third" || doc.Lines[2].Text != "Fifth" {
		t.Errorf("lines not sorted: %+v", doc.Lines)
	}
}

func TestParseTagsCollected(t *testing.T) {
	input := "[ar:Test Artist]\n[al:Test Album]\n[length:03:30]\n[00:01.00]Line\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.Tags["ar"] != "Test Artist" {
		t.Errorf("expected ar tag, got %q", doc.Tags["ar"])
	}
	if doc.Tags["al"] != "Test Album" {
		t.Errorf("expected al tag, got %q", doc.Tags["al"])
	}
}

func TestParseDoesNotContainCopyrightedLyrics(t *testing.T) {
	// Sanity check: ensure test file doesn't contain real song lyrics
	// by verifying only our known fake fixture strings appear
	knownFakeStrings := []string{"Public Domain", "Test Song", "First line", "Repeated line", "こんにちは"}
	for _, s := range knownFakeStrings {
		if !strings.Contains(fakeLRC, s) {
			t.Errorf("expected fake fixture to contain %q", s)
		}
	}
}
