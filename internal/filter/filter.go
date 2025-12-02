package filter

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dock-it/internal/docker"
)

// FilterType represents the type of filter being applied.
type FilterType string

const (
	FilterAge    FilterType = "age"
	FilterStatus FilterType = "status"
	FilterState  FilterType = "state"
	FilterName   FilterType = "name"
	FilterTag    FilterType = "tag"
	FilterSize   FilterType = "size"
	FilterDriver FilterType = "driver"
	FilterScope  FilterType = "scope"
)

// ComparisonOp represents comparison operators for filters.
type ComparisonOp string

const (
	OpEqual        ComparisonOp = "="
	OpNotEqual     ComparisonOp = "!="
	OpGreater      ComparisonOp = ">"
	OpLess         ComparisonOp = "<"
	OpGreaterEqual ComparisonOp = ">="
	OpLessEqual    ComparisonOp = "<="
	OpContains     ComparisonOp = "~"
	OpNotContains  ComparisonOp = "!~"
	OpRegex        ComparisonOp = "=~"
)

// Criterion represents a single filter criterion.
type Criterion struct {
	Type     FilterType
	Op       ComparisonOp
	Value    string
	Duration time.Duration // For age filters
	Bytes    int64         // For size filters
	Regex    *regexp.Regexp
}

// Filter holds multiple filter criteria and a simple search term.
type Filter struct {
	Criteria   []Criterion
	SearchTerm string // Simple search across all fields like k9s
}

// New creates a new empty filter.
func New() *Filter {
	return &Filter{
		Criteria:   []Criterion{},
		SearchTerm: "",
	}
}

// ParseFilter parses a filter string and returns a Filter.
// If the input contains operators (=, >, <, ~), it's treated as advanced filter criteria.
// Otherwise, it's treated as a simple search term that matches across all fields (like k9s).
// Supported advanced formats:
//   - age>1h, age<30m
//   - status=running, state=exited
//   - name~redis, name=mycontainer
//   - tag~ubuntu, tag=latest
//   - size>100MB
//   - driver=bridge
func ParseFilter(input string) (*Filter, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return New(), nil
	}

	f := New()

	// Check if this looks like an advanced filter (contains operators)
	hasOperators := strings.ContainsAny(input, "=><~")

	if !hasOperators {
		// Simple search mode - just store the search term
		f.SearchTerm = strings.ToLower(input)
		return f, nil
	}

	// Advanced filter mode - parse criteria
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		criterion, err := parseCriterion(part)
		if err != nil {
			return nil, fmt.Errorf("parse criterion '%s': %w", part, err)
		}
		f.Criteria = append(f.Criteria, criterion)
	}

	return f, nil
}

func parseCriterion(input string) (Criterion, error) {
	var c Criterion

	// Try to match operators in order of length (longest first to avoid prefix issues)
	operators := []ComparisonOp{OpGreaterEqual, OpLessEqual, OpNotEqual, OpNotContains, OpRegex, OpEqual, OpGreater, OpLess, OpContains}

	var op ComparisonOp
	var splitIdx int
	var opLen int

	for _, testOp := range operators {
		idx := strings.Index(input, string(testOp))
		if idx > 0 {
			op = testOp
			splitIdx = idx
			opLen = len(string(testOp))
			break
		}
	}

	if op == "" {
		return c, fmt.Errorf("no valid operator found in: %s", input)
	}

	filterType := strings.TrimSpace(input[:splitIdx])
	value := strings.TrimSpace(input[splitIdx+opLen:])

	if filterType == "" || value == "" {
		return c, fmt.Errorf("invalid filter format: %s", input)
	}

	c.Type = FilterType(filterType)
	c.Op = op
	c.Value = value

	// Parse special values
	switch c.Type {
	case FilterAge:
		dur, err := parseDuration(value)
		if err != nil {
			return c, fmt.Errorf("parse age duration: %w", err)
		}
		c.Duration = dur
	case FilterSize:
		bytes, err := parseBytes(value)
		if err != nil {
			return c, fmt.Errorf("parse size: %w", err)
		}
		c.Bytes = bytes
	}

	// Compile regex for regex operators
	if op == OpRegex || op == OpContains || op == OpNotContains {
		pattern := value
		if op == OpContains || op == OpNotContains {
			pattern = "(?i)" + regexp.QuoteMeta(value) // Case-insensitive substring
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return c, fmt.Errorf("compile regex: %w", err)
		}
		c.Regex = re
	}

	return c, nil
}

// parseDuration parses duration strings like "1h", "30m", "2d", "1w"
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// Check for custom units (d=days, w=weeks, mo=months, y=years)
	if strings.HasSuffix(s, "d") {
		val, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil {
			return 0, err
		}
		return time.Duration(val * 24 * float64(time.Hour)), nil
	}
	if strings.HasSuffix(s, "w") {
		val, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil {
			return 0, err
		}
		return time.Duration(val * 7 * 24 * float64(time.Hour)), nil
	}
	if strings.HasSuffix(s, "mo") {
		val, err := strconv.ParseFloat(s[:len(s)-2], 64)
		if err != nil {
			return 0, err
		}
		return time.Duration(val * 30 * 24 * float64(time.Hour)), nil
	}
	if strings.HasSuffix(s, "y") {
		val, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil {
			return 0, err
		}
		return time.Duration(val * 365 * 24 * float64(time.Hour)), nil
	}

	// Use standard time.ParseDuration for h, m, s
	return time.ParseDuration(s)
}

// parseBytes parses size strings like "100MB", "1.5GB", "512KB"
func parseBytes(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}

	multipliers := []struct {
		suffix     string
		multiplier int64
	}{
		{"TB", 1024 * 1024 * 1024 * 1024},
		{"GB", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"KB", 1024},
		{"B", 1},
	}

	for _, m := range multipliers {
		if strings.HasSuffix(s, m.suffix) {
			valStr := strings.TrimSpace(s[:len(s)-len(m.suffix)])
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil {
				return 0, err
			}
			return int64(val * float64(m.multiplier)), nil
		}
	}

	// Try parsing as plain number (bytes)
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %s", s)
	}
	return val, nil
}

// MatchContainer checks if a container matches all filter criteria.
func (f *Filter) MatchContainer(c docker.ContainerInfo) bool {
	// Simple search mode - match across all fields
	if f.SearchTerm != "" {
		searchLower := f.SearchTerm
		return strings.Contains(strings.ToLower(c.Name), searchLower) ||
			strings.Contains(strings.ToLower(c.Image), searchLower) ||
			strings.Contains(strings.ToLower(c.Status), searchLower) ||
			strings.Contains(strings.ToLower(c.State), searchLower) ||
			strings.Contains(strings.ToLower(c.ID), searchLower)
	}

	// Advanced filter mode - check criteria
	for _, criterion := range f.Criteria {
		if !matchContainerCriterion(c, criterion) {
			return false
		}
	}
	return true
}

func matchContainerCriterion(c docker.ContainerInfo, criterion Criterion) bool {
	switch criterion.Type {
	case FilterAge:
		age := time.Since(c.Created)
		return compareNumeric(float64(age), criterion.Op, float64(criterion.Duration))
	case FilterStatus:
		return compareString(c.Status, criterion.Op, criterion.Value, criterion.Regex)
	case FilterState:
		return compareString(c.State, criterion.Op, criterion.Value, criterion.Regex)
	case FilterName:
		return compareString(c.Name, criterion.Op, criterion.Value, criterion.Regex)
	default:
		return true
	}
}

// MatchImage checks if an image matches all filter criteria.
func (f *Filter) MatchImage(img docker.ImageInfo) bool {
	// Simple search mode - match across all fields
	if f.SearchTerm != "" {
		searchLower := f.SearchTerm
		return strings.Contains(strings.ToLower(img.Tag), searchLower) ||
			strings.Contains(strings.ToLower(img.ID), searchLower) ||
			strings.Contains(strings.ToLower(img.Size), searchLower)
	}

	// Advanced filter mode - check criteria
	for _, criterion := range f.Criteria {
		if !matchImageCriterion(img, criterion) {
			return false
		}
	}
	return true
}

func matchImageCriterion(img docker.ImageInfo, criterion Criterion) bool {
	switch criterion.Type {
	case FilterAge:
		age := time.Since(img.Created)
		return compareNumeric(float64(age), criterion.Op, float64(criterion.Duration))
	case FilterName, FilterTag:
		return compareString(img.Tag, criterion.Op, criterion.Value, criterion.Regex)
	case FilterSize:
		// Parse size from string (e.g., "100.50 MB")
		sizeStr := strings.Fields(img.Size)
		if len(sizeStr) >= 1 {
			val, err := strconv.ParseFloat(sizeStr[0], 64)
			if err == nil {
				bytes := int64(val * 1024 * 1024) // Assuming MB
				return compareNumeric(float64(bytes), criterion.Op, float64(criterion.Bytes))
			}
		}
		return true
	default:
		return true
	}
}

// MatchNetwork checks if a network matches all filter criteria.
func (f *Filter) MatchNetwork(net docker.NetworkInfo) bool {
	// Simple search mode - match across all fields
	if f.SearchTerm != "" {
		searchLower := f.SearchTerm
		return strings.Contains(strings.ToLower(net.Name), searchLower) ||
			strings.Contains(strings.ToLower(net.ID), searchLower) ||
			strings.Contains(strings.ToLower(net.Driver), searchLower) ||
			strings.Contains(strings.ToLower(net.Scope), searchLower)
	}

	// Advanced filter mode - check criteria
	for _, criterion := range f.Criteria {
		if !matchNetworkCriterion(net, criterion) {
			return false
		}
	}
	return true
}

func matchNetworkCriterion(net docker.NetworkInfo, criterion Criterion) bool {
	switch criterion.Type {
	case FilterAge:
		if !net.Created.IsZero() {
			age := time.Since(net.Created)
			return compareNumeric(float64(age), criterion.Op, float64(criterion.Duration))
		}
		return true
	case FilterName:
		return compareString(net.Name, criterion.Op, criterion.Value, criterion.Regex)
	case FilterDriver:
		return compareString(net.Driver, criterion.Op, criterion.Value, criterion.Regex)
	case FilterScope:
		return compareString(net.Scope, criterion.Op, criterion.Value, criterion.Regex)
	default:
		return true
	}
}

// MatchVolume checks if a volume matches all filter criteria.
func (f *Filter) MatchVolume(vol docker.VolumeInfo) bool {
	// Simple search mode - match across all fields
	if f.SearchTerm != "" {
		searchLower := f.SearchTerm
		return strings.Contains(strings.ToLower(vol.Name), searchLower) ||
			strings.Contains(strings.ToLower(vol.Driver), searchLower) ||
			strings.Contains(strings.ToLower(vol.Mountpoint), searchLower)
	}

	// Advanced filter mode - check criteria
	for _, criterion := range f.Criteria {
		if !matchVolumeCriterion(vol, criterion) {
			return false
		}
	}
	return true
}

func matchVolumeCriterion(vol docker.VolumeInfo, criterion Criterion) bool {
	switch criterion.Type {
	case FilterAge:
		if !vol.Created.IsZero() {
			age := time.Since(vol.Created)
			return compareNumeric(float64(age), criterion.Op, float64(criterion.Duration))
		}
		return true
	case FilterName:
		return compareString(vol.Name, criterion.Op, criterion.Value, criterion.Regex)
	case FilterDriver:
		return compareString(vol.Driver, criterion.Op, criterion.Value, criterion.Regex)
	default:
		return true
	}
}

func compareString(actual string, op ComparisonOp, expected string, regex *regexp.Regexp) bool {
	switch op {
	case OpEqual:
		return strings.EqualFold(actual, expected)
	case OpNotEqual:
		return !strings.EqualFold(actual, expected)
	case OpContains:
		return regex != nil && regex.MatchString(actual)
	case OpNotContains:
		return regex == nil || !regex.MatchString(actual)
	case OpRegex:
		return regex != nil && regex.MatchString(actual)
	default:
		return false
	}
}

func compareNumeric(actual float64, op ComparisonOp, expected float64) bool {
	switch op {
	case OpEqual:
		return actual == expected
	case OpNotEqual:
		return actual != expected
	case OpGreater:
		return actual > expected
	case OpLess:
		return actual < expected
	case OpGreaterEqual:
		return actual >= expected
	case OpLessEqual:
		return actual <= expected
	default:
		return false
	}
}

// String returns a human-readable representation of the filter.
func (f *Filter) String() string {
	if f.SearchTerm != "" {
		return f.SearchTerm
	}

	if len(f.Criteria) == 0 {
		return ""
	}

	parts := make([]string, len(f.Criteria))
	for i, c := range f.Criteria {
		parts[i] = fmt.Sprintf("%s%s%s", c.Type, c.Op, c.Value)
	}
	return strings.Join(parts, ", ")
}

// IsEmpty returns true if the filter has no criteria and no search term.
func (f *Filter) IsEmpty() bool {
	return len(f.Criteria) == 0 && f.SearchTerm == ""
}
