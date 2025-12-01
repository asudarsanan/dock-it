package logs

import (
	"strings"
	"time"
)

var logLevelColors = map[string]string{
	"TRACE":   "lightblue",
	"DEBUG":   "blue",
	"INFO":    "green",
	"WARN":    "yellow",
	"WARNING": "yellow",
	"ERROR":   "red",
	"ERR":     "red",
	"FATAL":   "red",
	"PANIC":   "red",
}

// Colorize applies timestamp and level colors using tview markup.
func Colorize(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}

	lines := strings.Split(raw, "\n")
	var b strings.Builder
	for i, line := range lines {
		if line == "" {
			if i < len(lines)-1 {
				b.WriteByte('\n')
			}
			continue
		}
		b.WriteString(formatLogLine(line))
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func formatLogLine(line string) string {
	ts, remainder := extractTimestamp(line)
	level, rest := extractLevel(remainder)

	var b strings.Builder
	if ts != "" {
		b.WriteString("[gray]")
		b.WriteString(ts)
		b.WriteString("[-] ")
	}
	if level != "" {
		color := logLevelColors[level]
		b.WriteString("[")
		b.WriteString(color)
		b.WriteString("::b]")
		b.WriteString(level)
		b.WriteString("[-] ")
	}

	b.WriteString(rest)
	return b.String()
}

func extractTimestamp(line string) (string, string) {
	trimmed := strings.TrimLeft(line, " \t")
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "", line
	}

	token := fields[0]
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.000",
	}
	for _, layout := range layouts {
		if _, err := time.Parse(layout, token); err == nil {
			remainder := strings.TrimLeft(trimmed[len(token):], " \t")
			return token, remainder
		}
	}
	return "", line
}

func extractLevel(line string) (string, string) {
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" {
		return "", trimmed
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "", trimmed
	}

	token := fields[0]
	cleaned := strings.Trim(token, "[]:")
	upper := strings.ToUpper(cleaned)
	if _, ok := logLevelColors[upper]; ok {
		remainder := strings.TrimLeft(trimmed[len(token):], " \t-:")
		return upper, remainder
	}

	return "", trimmed
}
