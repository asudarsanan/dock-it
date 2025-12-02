package filter

import (
	"testing"
	"time"

	"dock-it/internal/docker"
)

func TestParseFilter(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantErr        bool
		wantSearchTerm string
		wantCriteria   int
	}{
		{"empty", "", false, "", 0},
		{"simple search", "redis", false, "redis", 0},
		{"simple search multi word", "my container", false, "my container", 0},
		{"age greater", "age>1h", false, "", 1},
		{"age less", "age<30m", false, "", 1},
		{"status equal", "status=running", false, "", 1},
		{"name contains", "name~redis", false, "", 1},
		{"multiple criteria", "age>1h,status=running", false, "", 2},
		{"size filter", "size>100MB", false, "", 1},
		{"driver filter", "driver=bridge", false, "", 1},
		{"invalid empty value", "age>", true, "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := ParseFilter(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFilter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if f.SearchTerm != tt.wantSearchTerm {
					t.Errorf("ParseFilter() SearchTerm = %v, want %v", f.SearchTerm, tt.wantSearchTerm)
				}
				if len(f.Criteria) != tt.wantCriteria {
					t.Errorf("ParseFilter() Criteria count = %v, want %v", len(f.Criteria), tt.wantCriteria)
				}
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"hours", "1h", time.Hour, false},
		{"minutes", "30m", 30 * time.Minute, false},
		{"days", "2d", 48 * time.Hour, false},
		{"weeks", "1w", 7 * 24 * time.Hour, false},
		{"months", "1mo", 30 * 24 * time.Hour, false},
		{"years", "1y", 365 * 24 * time.Hour, false},
		{"invalid", "invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDuration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{"bytes", "100", 100, false},
		{"kilobytes", "1KB", 1024, false},
		{"megabytes", "100MB", 100 * 1024 * 1024, false},
		{"gigabytes", "1GB", 1024 * 1024 * 1024, false},
		{"decimal megabytes", "1.5MB", int64(1.5 * 1024 * 1024), false},
		{"invalid", "invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchContainer(t *testing.T) {
	now := time.Now()

	containers := []docker.ContainerInfo{
		{
			Name:    "redis-server",
			State:   "running",
			Created: now.Add(-2 * time.Hour),
		},
		{
			Name:    "nginx-proxy",
			State:   "exited",
			Created: now.Add(-30 * time.Minute),
		},
		{
			Name:    "postgres-db",
			State:   "running",
			Created: now.Add(-48 * time.Hour),
		},
	}

	tests := []struct {
		name      string
		filter    string
		container docker.ContainerInfo
		want      bool
	}{
		{"name contains match", "name~redis", containers[0], true},
		{"name contains no match", "name~mysql", containers[0], false},
		{"state match", "state=running", containers[0], true},
		{"state no match", "state=exited", containers[0], false},
		{"age greater match", "age>1h", containers[0], true},
		{"age greater no match", "age>3h", containers[0], false},
		{"age less match", "age<1h", containers[1], true},
		{"age less no match", "age<1h", containers[0], false},
		{"multiple criteria match", "state=running,age>1h", containers[0], true},
		{"multiple criteria no match", "state=running,age>3h", containers[0], false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := ParseFilter(tt.filter)
			if err != nil {
				t.Fatalf("ParseFilter() error = %v", err)
			}
			got := f.MatchContainer(tt.container)
			if got != tt.want {
				t.Errorf("MatchContainer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchImage(t *testing.T) {
	now := time.Now()

	images := []docker.ImageInfo{
		{
			Tag:     "ubuntu:latest",
			Size:    "100.50 MB",
			Created: now.Add(-24 * time.Hour),
		},
		{
			Tag:     "nginx:alpine",
			Size:    "50.25 MB",
			Created: now.Add(-5 * time.Hour),
		},
	}

	tests := []struct {
		name   string
		filter string
		image  docker.ImageInfo
		want   bool
	}{
		{"tag contains match", "tag~ubuntu", images[0], true},
		{"tag contains no match", "tag~redis", images[0], false},
		{"age greater match", "age>12h", images[0], true},
		{"age less match", "age<12h", images[1], true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := ParseFilter(tt.filter)
			if err != nil {
				t.Fatalf("ParseFilter() error = %v", err)
			}
			got := f.MatchImage(tt.image)
			if got != tt.want {
				t.Errorf("MatchImage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterIsEmpty(t *testing.T) {
	f1 := New()
	if !f1.IsEmpty() {
		t.Error("New filter should be empty")
	}

	f2, _ := ParseFilter("age>1h")
	if f2.IsEmpty() {
		t.Error("Filter with criteria should not be empty")
	}

	f3, _ := ParseFilter("redis")
	if f3.IsEmpty() {
		t.Error("Filter with search term should not be empty")
	}
}

func TestFilterString(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		want   string
	}{
		{"empty", "", ""},
		{"simple search", "redis", "redis"},
		{"single criterion", "age>1h", "age>1h"},
		{"multiple criteria", "age>1h,status=running", "age>1h, status=running"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := ParseFilter(tt.filter)
			if err != nil {
				t.Fatalf("ParseFilter() error = %v", err)
			}
			got := f.String()
			if got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleSearchContainer(t *testing.T) {
	container := docker.ContainerInfo{
		ID:      "abc123",
		Name:    "my-redis-container",
		Image:   "redis:latest",
		Status:  "Up 2 hours",
		State:   "running",
		Created: time.Now().Add(-24 * time.Hour),
	}

	tests := []struct {
		name   string
		search string
		want   bool
	}{
		{"match name", "redis", true},
		{"match name case insensitive", "REDIS", true},
		{"match image", "latest", true},
		{"match status", "Up", true},
		{"match state", "running", true},
		{"match ID", "abc", true},
		{"no match", "postgres", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := ParseFilter(tt.search)
			got := f.MatchContainer(container)
			if got != tt.want {
				t.Errorf("MatchContainer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleSearchImage(t *testing.T) {
	img := docker.ImageInfo{
		ID:      "sha256:abc",
		Tag:     "ubuntu:22.04",
		Size:    "100.50 MB",
		Created: time.Now().Add(-48 * time.Hour),
	}

	tests := []struct {
		name   string
		search string
		want   bool
	}{
		{"match tag", "ubuntu", true},
		{"match tag case insensitive", "UBUNTU", true},
		{"match version", "22.04", true},
		{"match ID", "abc", true},
		{"match size", "100", true},
		{"no match", "redis", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := ParseFilter(tt.search)
			got := f.MatchImage(img)
			if got != tt.want {
				t.Errorf("MatchImage() = %v, want %v", got, tt.want)
			}
		})
	}
}
