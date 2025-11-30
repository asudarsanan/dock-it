// Package main provides a terminal-based Docker management interface.
// This file contains the Docker client wrapper and API interaction logic.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

const (
	defaultTimeout  = 5 * time.Second
	statsTimeout    = 2 * time.Second
	maxStatsWorkers = 4
)

// DockerClient wraps the Docker SDK client with high-level methods
type DockerClient struct {
	cli *client.Client
}

// ContainerInfo holds display information for a single container
type ContainerInfo struct {
	ID     string
	Name   string
	Image  string
	Status string
	State  string
	Ports  string
	CPU    string
	Memory string
	NetIO  string
}

// ImageInfo holds display information for a Docker image
type ImageInfo struct {
	ID      string
	Tag     string
	Size    string
	Created string
}

// NetworkInfo holds display information for a Docker network
type NetworkInfo struct {
	ID     string
	Name   string
	Driver string
	Scope  string
}

// VolumeInfo holds display information for a Docker volume
type VolumeInfo struct {
	Name       string
	Driver     string
	Mountpoint string
}

// ContainerStats holds formatted resource usage statistics
type ContainerStats struct {
	CPU    string
	Memory string
	NetIO  string
}

// NewDockerClient creates a new Docker client using environment variables
func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerClient{cli: cli}, nil
}

func timeoutCtx(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return context.WithTimeout(context.Background(), timeout)
}

func (d *DockerClient) ListContainers() ([]ContainerInfo, error) {
	ctx, cancel := timeoutCtx(defaultTimeout)
	defer cancel()

	containers, err := d.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var result []ContainerInfo
	runningContainers := make(map[string]int)

	for i, c := range containers {
		name := "<none>"
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		ports := "-"
		if len(c.Ports) > 0 {
			portList := make([]string, 0, len(c.Ports))
			for _, port := range c.Ports {
				if port.PublicPort > 0 {
					portList = append(portList, fmt.Sprintf("%d->%d", port.PublicPort, port.PrivatePort))
				}
			}
			if len(portList) > 0 {
				ports = strings.Join(portList, ", ")
			}
		}

		info := ContainerInfo{
			ID:     c.ID,
			Name:   name,
			Image:  c.Image,
			Status: c.Status,
			State:  c.State,
			Ports:  ports,
			CPU:    "-",
			Memory: "-",
			NetIO:  "-",
		}
		result = append(result, info)

		if c.State == "running" {
			runningContainers[c.ID] = i
		}
	}

	if len(runningContainers) > 0 {
		var wg sync.WaitGroup
		var mu sync.Mutex
		sem := make(chan struct{}, maxStatsWorkers)

		for id, idx := range runningContainers {
			wg.Add(1)
			go func(containerID string, index int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				statsCtx, cancelStats := timeoutCtx(statsTimeout)
				defer cancelStats()

				stats, err := d.getContainerStatsWithContext(statsCtx, containerID)
				if err == nil {
					mu.Lock()
					result[index].CPU = stats.CPU
					result[index].Memory = stats.Memory
					result[index].NetIO = stats.NetIO
					mu.Unlock()
				}
			}(id, idx)
		}

		wg.Wait()
	}

	return result, nil
}

// getContainerStats fetches container stats with a default background context
func (d *DockerClient) getContainerStats(id string) (*ContainerStats, error) {
	ctx := context.Background()
	return d.getContainerStatsWithContext(ctx, id)
}

// getContainerStatsWithContext fetches container stats with a provided context for timeout/cancellation
func (d *DockerClient) getContainerStatsWithContext(ctx context.Context, id string) (*ContainerStats, error) {
	statsResp, err := d.cli.ContainerStats(ctx, id, false)
	if err != nil {
		return nil, err
	}
	defer statsResp.Body.Close()

	var payload container.StatsResponse
	if err := json.NewDecoder(statsResp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	cpuDelta := float64(payload.CPUStats.CPUUsage.TotalUsage - payload.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(payload.CPUStats.SystemUsage - payload.PreCPUStats.SystemUsage)
	onlineCPUs := float64(payload.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 && len(payload.CPUStats.CPUUsage.PercpuUsage) > 0 {
		onlineCPUs = float64(len(payload.CPUStats.CPUUsage.PercpuUsage))
	}

	cpuPercent := 0.0
	if cpuDelta > 0 && systemDelta > 0 && onlineCPUs > 0 {
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}

	memUsage := float64(payload.MemoryStats.Usage)
	memLimit := float64(payload.MemoryStats.Limit)
	memPercent := 0.0
	if memLimit > 0 {
		memPercent = (memUsage / memLimit) * 100.0
	}

	rxBytes := 0.0
	txBytes := 0.0
	for _, netStats := range payload.Networks {
		rxBytes += float64(netStats.RxBytes)
		txBytes += float64(netStats.TxBytes)
	}

	return &ContainerStats{
		CPU:    fmt.Sprintf("%.2f%%", cpuPercent),
		Memory: fmt.Sprintf("%.2f%%", memPercent),
		NetIO:  fmt.Sprintf("%.1fMB/%.1fMB", rxBytes/(1024*1024), txBytes/(1024*1024)),
	}, nil
}

func (d *DockerClient) StartContainer(id string) error {
	ctx, cancel := timeoutCtx(defaultTimeout)
	defer cancel()
	return d.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (d *DockerClient) StopContainer(id string) error {
	ctx, cancel := timeoutCtx(defaultTimeout)
	defer cancel()
	timeout := 10
	return d.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (d *DockerClient) RestartContainer(id string) error {
	ctx, cancel := timeoutCtx(defaultTimeout)
	defer cancel()
	timeout := 10
	return d.cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (d *DockerClient) RemoveContainer(id string) error {
	ctx, cancel := timeoutCtx(defaultTimeout)
	defer cancel()
	return d.cli.ContainerRemove(ctx, id, container.RemoveOptions{})
}

func (d *DockerClient) GetContainerLogs(id string, tail string) (string, error) {
	ctx, cancel := timeoutCtx(defaultTimeout)
	defer cancel()
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
		Timestamps: false,
	}

	out, err := d.cli.ContainerLogs(ctx, id, options)
	if err != nil {
		return "", err
	}
	defer out.Close()

	data, err := io.ReadAll(out)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (d *DockerClient) ExecContainer(id string) error {
	return nil // Placeholder - will be implemented with actual shell execution
}

func (d *DockerClient) ListImages() ([]ImageInfo, error) {
	ctx, cancel := timeoutCtx(defaultTimeout)
	defer cancel()

	images, err := d.cli.ImageList(ctx, image.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var result []ImageInfo
	for _, img := range images {
		tag := "<none>"
		if len(img.RepoTags) > 0 {
			tag = img.RepoTags[0]
		}

		size := fmt.Sprintf("%.2f MB", float64(img.Size)/(1024*1024))
		created := formatRelativeDuration(time.Since(time.Unix(img.Created, 0)))

		info := ImageInfo{
			ID:      shortImageID(img.ID),
			Tag:     tag,
			Size:    size,
			Created: created,
		}
		result = append(result, info)
	}

	return result, nil
}

func (d *DockerClient) ListNetworks() ([]NetworkInfo, error) {
	ctx, cancel := timeoutCtx(defaultTimeout)
	defer cancel()

	networks, err := d.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []NetworkInfo
	for _, net := range networks {
		id := net.ID
		if len(id) > 12 {
			id = id[:12]
		}
		info := NetworkInfo{
			ID:     id,
			Name:   net.Name,
			Driver: net.Driver,
			Scope:  net.Scope,
		}
		result = append(result, info)
	}

	return result, nil
}

func (d *DockerClient) ListVolumes() ([]VolumeInfo, error) {
	ctx, cancel := timeoutCtx(defaultTimeout)
	defer cancel()

	volumes, err := d.cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []VolumeInfo
	for _, vol := range volumes.Volumes {
		info := VolumeInfo{
			Name:       vol.Name,
			Driver:     vol.Driver,
			Mountpoint: vol.Mountpoint,
		}
		result = append(result, info)
	}

	return result, nil
}

func (d *DockerClient) RemoveImage(id string) error {
	ctx, cancel := timeoutCtx(defaultTimeout)
	defer cancel()
	_, err := d.cli.ImageRemove(ctx, id, image.RemoveOptions{})
	return err
}

func (d *DockerClient) RemoveNetwork(id string) error {
	ctx, cancel := timeoutCtx(defaultTimeout)
	defer cancel()
	return d.cli.NetworkRemove(ctx, id)
}

func (d *DockerClient) RemoveVolume(name string) error {
	ctx, cancel := timeoutCtx(defaultTimeout)
	defer cancel()
	return d.cli.VolumeRemove(ctx, name, false)
}

func shortImageID(id string) string {
	const prefix = "sha256:"
	if strings.HasPrefix(id, prefix) {
		trimmed := id[len(prefix):]
		if len(trimmed) >= 12 {
			return trimmed[:12]
		}
		return trimmed
	}
	if len(id) >= 12 {
		return id[:12]
	}
	return id
}

func formatRelativeDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	if d < time.Minute {
		return "just now"
	}

	units := []struct {
		dur   time.Duration
		label string
	}{
		{time.Hour * 24 * 365, "y"},
		{time.Hour * 24 * 30, "mo"},
		{time.Hour * 24 * 7, "w"},
		{time.Hour * 24, "d"},
		{time.Hour, "h"},
		{time.Minute, "m"},
	}

	for _, unit := range units {
		if d >= unit.dur {
			value := d / unit.dur
			return fmt.Sprintf("%d%s ago", value, unit.label)
		}
	}

	return "just now"
}
