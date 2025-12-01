package logs

import "testing"

func TestColorizeLogs(t *testing.T) {
	raw := "2025-01-01T10:00:00Z INFO service ready"
	want := "[gray]2025-01-01T10:00:00Z[-] [green::b]INFO[-] service ready"
	if got := Colorize(raw); got != want {
		t.Fatalf("Colorize() = %q, want %q", got, want)
	}
}

func TestColorizeLogsBracketedLevel(t *testing.T) {
	raw := "2025-01-01T10:00:00Z [ERROR] boom"
	want := "[gray]2025-01-01T10:00:00Z[-] [red::b]ERROR[-] boom"
	if got := Colorize(raw); got != want {
		t.Fatalf("Colorize() = %q, want %q", got, want)
	}
}

func TestColorizeLogsNoTimestamp(t *testing.T) {
	raw := "plain line"
	if got := Colorize(raw); got != raw {
		t.Fatalf("expected unchanged logs when no timestamp/level; got %q", got)
	}
}

func TestExtractTimestamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		ts    string
		rest  string
	}{
		{"rfc3339", "2025-01-01T10:00:00Z INFO", "2025-01-01T10:00:00Z", "INFO"},
		{"nano", "2025-01-01T10:00:00.123Z WARN", "2025-01-01T10:00:00.123Z", "WARN"},
		{"invalid", "INFO only", "", "INFO only"},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ts, rest := extractTimestamp(tc.input)
			if ts != tc.ts || rest != tc.rest {
				t.Fatalf("extractTimestamp(%q) = (%q,%q), want (%q,%q)", tc.input, ts, rest, tc.ts, tc.rest)
			}
		})
	}
}

func TestExtractLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		lvl   string
		rest  string
	}{
		{"plain", "INFO something", "INFO", "something"},
		{"brackets", "[warn]: msg", "WARN", "msg"},
		{"unknown", "notice msg", "", "notice msg"},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lvl, rest := extractLevel(tc.input)
			if lvl != tc.lvl || rest != tc.rest {
				t.Fatalf("extractLevel(%q) = (%q,%q), want (%q,%q)", tc.input, lvl, rest, tc.lvl, tc.rest)
			}
		})
	}
}

func TestFormatLogLine(t *testing.T) {
	line := "2025-01-01T10:00:00Z INFO ready"
	want := "[gray]2025-01-01T10:00:00Z[-] [green::b]INFO[-] ready"
	if got := formatLogLine(line); got != want {
		t.Fatalf("formatLogLine() = %q, want %q", got, want)
	}

	noLevel := "2025-01-01T10:00:00Z plain"
	wantNoLevel := "[gray]2025-01-01T10:00:00Z[-] plain"
	if got := formatLogLine(noLevel); got != wantNoLevel {
		t.Fatalf("formatLogLine() = %q, want %q", got, wantNoLevel)
	}
}
