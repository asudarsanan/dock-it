package docker

import (
	"testing"
	"time"
)

func TestShortImageID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"sha256WithLongDigest", "sha256:1234567890abcdef", "1234567890ab"},
		{"sha256ShortDigest", "sha256:abc", "abc"},
		{"noPrefixLong", "abcdef1234567890", "abcdef123456"},
		{"noPrefixShort", "tiny", "tiny"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shortImageID(tt.input)
			if got != tt.want {
				t.Fatalf("shortImageID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatRelativeDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input time.Duration
		want  string
	}{
		{"justNow", 30 * time.Second, "just now"},
		{"oneMinute", time.Minute, "1m ago"},
		{"multiMinutes", 5 * time.Minute, "5m ago"},
		{"hours", 3 * time.Hour, "3h ago"},
		{"days", 72 * time.Hour, "3d ago"},
		{"weeks", 15 * 24 * time.Hour, "2w ago"},
		{"months", 90 * 24 * time.Hour, "3mo ago"},
		{"years", 800 * 24 * time.Hour, "2y ago"},
		{"negative", -2 * time.Hour, "2h ago"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatRelativeDuration(tt.input)
			if got != tt.want {
				t.Fatalf("formatRelativeDuration(%s) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTimeoutCtx(t *testing.T) {
	t.Parallel()

	checkDeadline := func(t *testing.T, d time.Duration, tolerance time.Duration) {
		ctx, cancel := timeoutCtx(d)
		t.Cleanup(cancel)

		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatalf("deadline not set for duration %s", d)
		}

		remaining := time.Until(deadline)
		if remaining > d+tolerance {
			t.Fatalf("deadline too far: got %s want <= %s", remaining, d+tolerance)
		}
	}

	t.Run("customDuration", func(t *testing.T) {
		t.Parallel()
		checkDeadline(t, 200*time.Millisecond, 50*time.Millisecond)
	})

	t.Run("defaultDuration", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := timeoutCtx(0)
		t.Cleanup(cancel)

		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatalf("expected deadline for default duration")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("deadline already expired")
		}
		if remaining < defaultTimeout-50*time.Millisecond || remaining > defaultTimeout+50*time.Millisecond {
			t.Fatalf("default deadline outside tolerance: got %s", remaining)
		}
	})
}

func TestFormatAsJSON(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		input := struct {
			A string
			B int
		}{A: "foo", B: 42}
		got, err := formatAsJSON(input)
		if err != nil {
			t.Fatalf("formatAsJSON() unexpected error: %v", err)
		}
		want := "{\n  \"A\": \"foo\",\n  \"B\": 42\n}"
		if got != want {
			t.Fatalf("formatAsJSON() = %q, want %q", got, want)
		}
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		_, err := formatAsJSON(make(chan int))
		if err == nil {
			t.Fatalf("expected error for unsupported type")
		}
	})
}
