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

// NewDockerClient creates a new Docker client using environment variables
func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerClient{cli: cli}, nil
}

func (d *DockerClient) ListContainers() ([]ContainerInfo, error) {
	containers, err := d.cli.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var result []ContainerInfo

	// Collect running container IDs for concurrent stats fetching
	runningContainers := make(map[string]int) // map[containerID]index

	for i, c := range containers {
		name := "<none>"
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		// Format ports
		ports := ""
		if len(c.Ports) > 0 {
			portList := []string{}
			for _, port := range c.Ports {
				if port.PublicPort > 0 {
					portList = append(portList, fmt.Sprintf("%d->%d", port.PublicPort, port.PrivatePort))
				}
			}
			if len(portList) > 0 {
				ports = strings.Join(portList, ", ")
			} else {
				ports = "-"
			}
		} else {
			ports = "-"
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

	// Fetch stats concurrently for all running containers
	if len(runningContainers) > 0 {
		var wg sync.WaitGroup
		var mu sync.Mutex

		for id, idx := range runningContainers {
			wg.Add(1)
			go func(containerID string, index int) {
				defer wg.Done()

				statsCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

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

		wg.Wait() // Wait for all stats to be fetched (or timeout)
	}

	return result, nil
}

// ContainerStats holds formatted resource usage statistics
type ContainerStats struct {
	CPU    string
	Memory string
	NetIO  string
}

// getContainerStats fetches container stats with a default background context
func (d *DockerClient) getContainerStats(id string) (*ContainerStats, error) {
	ctx := context.Background()
	return d.getContainerStatsWithContext(ctx, id)
}

// getContainerStatsWithContext fetches container stats with a provided context for timeout/cancellation
func (d *DockerClient) getContainerStatsWithContext(ctx context.Context, id string) (*ContainerStats, error) {
	stats, err := d.cli.ContainerStats(ctx, id, false) // false = single stat, not stream
	if err != nil {
		return nil, err
	}
	defer stats.Body.Close()

	var v map[string]interface{}
	if err := json.NewDecoder(stats.Body).Decode(&v); err != nil {
		return nil, err
	}

	// Calculate CPU percentage
	cpuPercent := 0.0
	if cpuStats, ok := v["cpu_stats"].(map[string]interface{}); ok {
		if preCpuStats, ok := v["precpu_stats"].(map[string]interface{}); ok {
			cpuDelta := 0.0
			systemDelta := 0.0

			if cpuUsage, ok := cpuStats["cpu_usage"].(map[string]interface{}); ok {
				if totalUsage, ok := cpuUsage["total_usage"].(float64); ok {
					if preCpuUsage, ok := preCpuStats["cpu_usage"].(map[string]interface{}); ok {
						if preTotalUsage, ok := preCpuUsage["total_usage"].(float64); ok {
							cpuDelta = totalUsage - preTotalUsage
						}
					}
				}
			}

			if systemUsage, ok := cpuStats["system_cpu_usage"].(float64); ok {
				if preSystemUsage, ok := preCpuStats["system_cpu_usage"].(float64); ok {
					systemDelta = systemUsage - preSystemUsage
				}
			}

			if systemDelta > 0.0 && cpuDelta > 0.0 {
				cpuPercent = (cpuDelta / systemDelta) * 100.0
			}
		}
	}

	// Calculate memory usage
	memUsage := 0.0
	memLimit := 0.0
	if memStats, ok := v["memory_stats"].(map[string]interface{}); ok {
		if usage, ok := memStats["usage"].(float64); ok {
			memUsage = usage
		}
		if limit, ok := memStats["limit"].(float64); ok {
			memLimit = limit
		}
	}

	memPercent := 0.0
	if memLimit > 0 {
		memPercent = (memUsage / memLimit) * 100.0
	}

	// Calculate network I/O
	rxBytes := 0.0
	txBytes := 0.0
	if networks, ok := v["networks"].(map[string]interface{}); ok {
		for _, netStats := range networks {
			if netMap, ok := netStats.(map[string]interface{}); ok {
				if rx, ok := netMap["rx_bytes"].(float64); ok {
					rxBytes += rx
				}
				if tx, ok := netMap["tx_bytes"].(float64); ok {
					txBytes += tx
				}
			}
		}
	}

	return &ContainerStats{
		CPU:    fmt.Sprintf("%.2f%%", cpuPercent),
		Memory: fmt.Sprintf("%.2f%%", memPercent),
		NetIO:  fmt.Sprintf("%.1fMB/%.1fMB", rxBytes/(1024*1024), txBytes/(1024*1024)),
	}, nil
}

func (d *DockerClient) StartContainer(id string) error {
	return d.cli.ContainerStart(context.Background(), id, container.StartOptions{})
}

func (d *DockerClient) StopContainer(id string) error {
	timeout := 10
	return d.cli.ContainerStop(context.Background(), id, container.StopOptions{Timeout: &timeout})
}

func (d *DockerClient) RestartContainer(id string) error {
	timeout := 10
	return d.cli.ContainerRestart(context.Background(), id, container.StopOptions{Timeout: &timeout})
}

func (d *DockerClient) RemoveContainer(id string) error {
	return d.cli.ContainerRemove(context.Background(), id, container.RemoveOptions{})
}

func (d *DockerClient) GetContainerLogs(id string, tail string) (string, error) {
	ctx := context.Background()
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

	buf := make([]byte, 1024*1024) // 1MB buffer
	n, err := out.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}

	return string(buf[:n]), nil
}

func (d *DockerClient) ExecContainer(id string) error {
	return nil // Placeholder - will be implemented with actual shell execution
}

func (d *DockerClient) ListImages() ([]ImageInfo, error) {
	images, err := d.cli.ImageList(context.Background(), image.ListOptions{All: true})
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
		created := fmt.Sprintf("%d days ago", img.Created/86400)

		info := ImageInfo{
			ID:      img.ID[7:19], // Short ID
			Tag:     tag,
			Size:    size,
			Created: created,
		}
		result = append(result, info)
	}

	return result, nil
}

func (d *DockerClient) ListNetworks() ([]NetworkInfo, error) {
	networks, err := d.cli.NetworkList(context.Background(), network.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []NetworkInfo
	for _, net := range networks {
		info := NetworkInfo{
			ID:     net.ID[:12],
			Name:   net.Name,
			Driver: net.Driver,
			Scope:  net.Scope,
		}
		result = append(result, info)
	}

	return result, nil
}

func (d *DockerClient) ListVolumes() ([]VolumeInfo, error) {
	volumes, err := d.cli.VolumeList(context.Background(), volume.ListOptions{})
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
	_, err := d.cli.ImageRemove(context.Background(), id, image.RemoveOptions{})
	return err
}

func (d *DockerClient) RemoveNetwork(id string) error {
	return d.cli.NetworkRemove(context.Background(), id)
}

func (d *DockerClient) RemoveVolume(name string) error {
	return d.cli.VolumeRemove(context.Background(), name, false)
}
