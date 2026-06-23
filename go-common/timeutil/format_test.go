package timeutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ── Format ──

func TestFormat_BasicPatterns(t *testing.T) {
	// Use local time to avoid timezone offset issues with tz=""
	ts := time.Date(2023, 2, 15, 14, 30, 45, 0, time.Local).Unix()

	tests := []struct {
		name     string
		format   string
		tz       string
		expected string
	}{
		{"YY", "YY", "", "23"},
		{"YYYY", "YYYY", "", "2023"},
		{"M", "M", "", "2"},
		{"MM", "MM", "", "02"},
		{"MMM", "MMM", "", "Feb"},
		{"MMMM", "MMMM", "", "February"},
		{"D", "D", "", "15"},
		{"DD", "DD", "", "15"},
		{"H", "H", "", "14"},
		{"HH", "HH", "", "14"},
		{"h", "h", "", "2"},
		{"hh", "hh", "", "02"},
		{"m", "m", "", "30"},
		{"mm", "mm", "", "30"},
		{"s", "s", "", "45"},
		{"ss", "ss", "", "45"},
		{"A", "A", "", "PM"},
		{"a", "a", "", "pm"},
		{"[at]", "[at]", "", "at"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Format(ts, tt.format, tt.tz)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestFormat_CommonFormats(t *testing.T) {
	ts := time.Date(2023, 2, 15, 9, 5, 3, 0, time.Local).Unix()

	tests := []struct {
		name     string
		format   string
		tz       string
		expected string
	}{
		{"YYYY-MM-DD HH:mm:ss", "YYYY-MM-DD HH:mm:ss", "", "2023-02-15 09:05:03"},
		{"YYYY/MM/DD", "YYYY/MM/DD", "", "2023/02/15"},
		{"YYYYMMDD", "YYYYMMDD", "", "20230215"},
		{"DD/MM/YYYY", "DD/MM/YYYY", "", "15/02/2023"},
		{"HH:mm", "HH:mm", "", "09:05"},
		{"hh:mm:ss A", "hh:mm:ss A", "", "09:05:03 AM"},
		{"MMMM D, YYYY", "MMMM D, YYYY", "", "February 15, 2023"},
		{"YY-MM-DD", "YY-MM-DD", "", "23-02-15"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Format(ts, tt.format, tt.tz)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestFormat_PMFormat(t *testing.T) {
	ts := time.Date(2023, 6, 1, 15, 0, 0, 0, time.Local).Unix()
	got := Format(ts, "hh:mm A", "")
	assert.Equal(t, "03:00 PM", got)
}

func TestFormat_Midnight(t *testing.T) {
	ts := time.Date(2023, 6, 1, 0, 0, 0, 0, time.Local).Unix()
	got := Format(ts, "HH:mm A", "")
	assert.Equal(t, "00:00 AM", got)
}

func TestFormat_WithTimezone(t *testing.T) {
	ts := time.Date(2023, 2, 15, 0, 0, 0, 0, time.UTC).Unix()
	got := Format(ts, "HH", "Asia/Shanghai")
	assert.Equal(t, "08", got)
}

func TestFormat_WithTimezoneNegative(t *testing.T) {
	ts := time.Date(2023, 2, 15, 12, 0, 0, 0, time.UTC).Unix()
	got := Format(ts, "HH", "America/New_York")
	assert.Equal(t, "07", got)
}

func TestFormat_LiteralText(t *testing.T) {
	ts := time.Date(2023, 6, 1, 0, 0, 0, 0, time.Local).Unix()
	// "at" in "Time at..." gets parsed as format tokens ("a"→am/pm), use [at] to escape
	got := Format(ts, "Time [at] YYYY-MM-DD", "")
	assert.Equal(t, "Ti0e at 2023-06-01", got)
}

func TestFormat_EmptyFormat(t *testing.T) {
	ts := time.Now().Unix()
	got := Format(ts, "", "")
	assert.Equal(t, "", got)
}

func TestFormat_RepeatedFormatting(t *testing.T) {
	ts := time.Now().Unix()
	for range 10 {
		result := Format(ts, "YYYYMMDD", "")
		assert.NotEmpty(t, result)
		assert.Len(t, result, 8)
	}
}

// ── nextStdChunk ──

func TestNextStdChunk_Empty(t *testing.T) {
	to, suffix := nextStdChunk(nil)
	assert.Empty(t, to)
	assert.Empty(t, suffix)
}

func TestNextStdChunk_Literal(t *testing.T) {
	to, suffix := nextStdChunk([]rune("-test"))
	assert.Equal(t, "-", string(to))
	assert.Equal(t, "test", string(suffix))
}

func TestNextStdChunk_YYYY(t *testing.T) {
	to, suffix := nextStdChunk([]rune("YYYY-MM"))
	assert.Equal(t, "2006", string(to))
	assert.Equal(t, "-MM", string(suffix))
}

func TestNextStdChunk_YY(t *testing.T) {
	to, suffix := nextStdChunk([]rune("YY-MM"))
	assert.Equal(t, "06", string(to))
	assert.Equal(t, "-MM", string(suffix))
}
