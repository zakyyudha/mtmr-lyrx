package marquee

import (
	"strings"
	"testing"
)

func newFormatter(width int, sep string) Formatter {
	return Formatter{Config: Config{Width: width, Separator: sep}}
}

func TestFrameShortLinePadded(t *testing.T) {
	f := newFormatter(10, " · ")
	frame := f.Frame("Hello", 0)
	if StringWidth(frame) != 10 {
		t.Errorf("expected width 10, got %d: %q", StringWidth(frame), frame)
	}
	if !strings.HasPrefix(frame, "Hello") {
		t.Errorf("expected frame to start with Hello, got %q", frame)
	}
}

func TestFrameShortLineStable(t *testing.T) {
	f := newFormatter(10, " · ")
	frame0 := f.Frame("Hi", 0)
	frame5 := f.Frame("Hi", 5)
	if frame0 != frame5 {
		t.Errorf("short line should be stable across ticks: %q vs %q", frame0, frame5)
	}
}

func TestFrameLongLineScrolls(t *testing.T) {
	f := newFormatter(5, " | ")
	line := "Hello World"
	frame0 := f.Frame(line, 0)
	frame3 := f.Frame(line, 3)
	if frame0 == frame3 {
		t.Errorf("long line should scroll: tick 0 %q == tick 3 %q", frame0, frame3)
	}
}

func TestFrameLongLineWidth(t *testing.T) {
	f := newFormatter(8, " · ")
	line := "This is a very long lyric line that should scroll"
	for tick := 0; tick < 20; tick++ {
		frame := f.Frame(line, tick)
		w := StringWidth(frame)
		if w != 8 {
			t.Errorf("tick %d: expected width 8, got %d: %q", tick, w, frame)
		}
	}
}

func TestFrameNewlineBecomesSpace(t *testing.T) {
	f := newFormatter(10, " · ")
	frame := f.Frame("Hello\nWorld", 0)
	if strings.Contains(frame, "\n") {
		t.Errorf("frame should not contain newline: %q", frame)
	}
}

func TestFrameCRLFBecomesSpace(t *testing.T) {
	f := newFormatter(10, " · ")
	frame := f.Frame("Hello\r\nWorld", 0)
	if strings.Contains(frame, "\r") || strings.Contains(frame, "\n") {
		t.Errorf("frame should not contain CR/LF: %q", frame)
	}
}

func TestFrameANSIStripped(t *testing.T) {
	f := newFormatter(10, " · ")
	frame := f.Frame("\x1b[31mRed\x1b[0m", 0)
	if strings.Contains(frame, "\x1b") {
		t.Errorf("frame should not contain ANSI: %q", frame)
	}
	if !strings.HasPrefix(frame, "Red") {
		t.Errorf("expected Red after ANSI strip, got %q", frame)
	}
}

func TestFrameZeroWidth(t *testing.T) {
	f := newFormatter(0, " · ")
	frame := f.Frame("Hello", 0)
	if frame != "" {
		t.Errorf("expected empty frame for width 0, got %q", frame)
	}
}

func TestFrameSeparatorAppearsInScroll(t *testing.T) {
	f := newFormatter(5, ">>")
	line := "ABCDEFGHIJ" // 10 chars, wider than 5
	found := false
	for tick := 0; tick < 20; tick++ {
		frame := f.Frame(line, tick)
		if strings.Contains(frame, ">") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected separator >> to appear in scrolling frames")
	}
}

func TestStripANSI(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"\x1b[31mRed\x1b[0m", "Red"},
		{"\x1b[1;32mBold Green\x1b[0m", "Bold Green"},
		{"No ANSI", "No ANSI"},
		{"", ""},
	}
	for _, c := range cases {
		got := StripANSI(c.input)
		if got != c.expected {
			t.Errorf("StripANSI(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestSingleLine(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Hello\nWorld", "Hello World"},
		{"Hello\r\nWorld", "Hello World"},
		{"Hello\rWorld", "Hello World"},
		{"No newlines", "No newlines"},
	}
	for _, c := range cases {
		got := SingleLine(c.input)
		if got != c.expected {
			t.Errorf("SingleLine(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestStringWidthASCII(t *testing.T) {
	if w := StringWidth("Hello"); w != 5 {
		t.Errorf("expected 5, got %d", w)
	}
}

func TestStringWidthCJK(t *testing.T) {
	// Each CJK character is 2 cells
	w := StringWidth("日本語")
	if w != 6 {
		t.Errorf("expected 6 for 3 CJK chars, got %d", w)
	}
}

func TestFrameCJKWidth(t *testing.T) {
	f := newFormatter(6, " · ")
	// "日本語" = 6 cells, exactly fits
	frame := f.Frame("日本語", 0)
	if StringWidth(frame) != 6 {
		t.Errorf("expected width 6 for CJK frame, got %d: %q", StringWidth(frame), frame)
	}
}
