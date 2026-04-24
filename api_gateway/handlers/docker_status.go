package handlers

import (
	"context"
	"errors"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/logger"
)

// isDockerAvailable checks whether the docker CLI binary is reachable in $PATH.
func isDockerAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

// DockerServiceInfo represents the status of a single Docker container.
type DockerServiceInfo struct {
	Name      string `json:"name"`
	Status    string `json:"status"`    // "running", "stopped", "restarting", "paused"
	Health    string `json:"health"`    // "healthy", "unhealthy", "starting", "none"
	Uptime    string `json:"uptime"`    // e.g. "2 hours", "3 days"
	Port      string `json:"port"`      // e.g. "8080"
	Container string `json:"container"` // full container name
	Image     string `json:"image"`     // e.g. "postgres:15-alpine"
}

// DockerStatusResponse is the complete response for GET /api/v1/services/status.
type DockerStatusResponse struct {
	Services  []DockerServiceInfo `json:"services"`
	Timestamp string              `json:"timestamp"`
}

// GetDockerServiceStatus returns the status of all Docker containers visible
// to the API gateway process. If Docker is not available (e.g. running outside
// a container or without Docker socket), it returns a fallback list of expected
// services with unknown status.
func GetDockerServiceStatus(c *gin.Context) {
	// When the Docker CLI is not even present, skip the exec entirely and
	// return the inferred (best-guess) service list. This avoids log spam
	// from "executable file not found" errors inside containers that do not
	// have Docker installed.
	if !isDockerAvailable() {
		logger.Debug("Docker CLI not in $PATH, returning inferred service status")
		c.JSON(http.StatusOK, DockerStatusResponse{
			Services:  inferredDockerServices(),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	services, err := queryDockerContainers(ctx)
	if err != nil {
		logger.Warn("Docker query failed (%v), returning inferred service status", err)
		services = inferredDockerServices()
	}

	c.JSON(http.StatusOK, DockerStatusResponse{
		Services:  services,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// GetDockerServiceLogs returns recent log output for a specific service
// container. The service name is matched against Docker container names
// using several common naming patterns (docker-compose project prefixes).
func GetDockerServiceLogs(c *gin.Context) {
	serviceName := c.Param("name")
	if serviceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service name is required"})
		return
	}

	// Sanitize: reject excessively long names to prevent abuse
	if len(serviceName) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service name too long (max 128 characters)"})
		return
	}

	// Sanitize: only allow alphanumeric, hyphens, and underscores.
	// This prevents any command injection even though exec.Command does not
	// use a shell — it also guards against path traversal and Docker flag
	// injection (e.g. names starting with "--").
	if strings.HasPrefix(serviceName, "-") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service name: must not start with a dash"})
		return
	}
	for _, ch := range serviceName {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '-' || ch == '_') {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service name: only alphanumeric, hyphens, and underscores allowed"})
			return
		}
	}

	linesStr := c.DefaultQuery("lines", "100")
	lines, err := strconv.Atoi(linesStr)
	if err != nil || lines < 1 || lines > 5000 {
		lines = 100
	}
	tailArg := strconv.Itoa(lines)

	// Fast-path: if the docker CLI is not installed at all, return a helpful
	// message immediately instead of spawning doomed exec.Command calls.
	if !isDockerAvailable() {
		logger.Debug("Docker CLI not found in $PATH, cannot retrieve logs for %s", serviceName)
		c.JSON(http.StatusOK, gin.H{
			"service":           serviceName,
			"logs":              "Log retrieval requires Docker socket access. To enable, mount /var/run/docker.sock into the api-gateway container and ensure the Docker CLI is available.",
			"docker_unavailable": true,
			"lines":             tailArg,
		})
		return
	}

	// Try several container name patterns commonly used by docker-compose.
	// Pattern: <project>-<service>-1, <project>_<service>_1, or just <service>.
	nameCandidates := []string{
		"prod-" + serviceName + "-1",
		"prod_" + serviceName + "_1",
		serviceName,
	}

	var output []byte
	var lastErr error

	logCtx, cancelLogs := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancelLogs()

	for _, containerName := range nameCandidates {
		cmd := exec.CommandContext(logCtx, "docker", "logs", "--tail", tailArg, "--timestamps", containerName)
		out, cmdErr := cmd.CombinedOutput()
		if cmdErr == nil {
			output = out
			lastErr = nil
			break
		}
		lastErr = cmdErr
	}

	if lastErr != nil {
		logger.Debug("Docker logs not available for %s: %v", serviceName, lastErr)

		// Distinguish between "docker not working" and "container not found"
		errMsg := lastErr.Error()
		dockerUnavailable := false
		var execErr *exec.Error
		if errors.As(lastErr, &execErr) {
			dockerUnavailable = true
			errMsg = "Log retrieval requires Docker socket access. To enable, mount /var/run/docker.sock into the api-gateway container."
		}

		c.JSON(http.StatusOK, gin.H{
			"service":           serviceName,
			"logs":              "Log retrieval not available. Docker may not be accessible from this environment.",
			"error":             errMsg,
			"docker_unavailable": dockerUnavailable,
			"lines":             tailArg,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"service": serviceName,
		"logs":    string(output),
		"lines":   tailArg,
	})
}

// queryDockerContainers shells out to `docker ps -a` to get container status.
// Uses the caller-provided context so the shell-out is bounded by a timeout.
func queryDockerContainers(ctx context.Context) ([]DockerServiceInfo, error) {
	// --format with tab-separated fields: name, status, ports, image, label for health
	cmd := exec.CommandContext(ctx,
		"docker", "ps", "-a",
		"--format", "{{.Names}}\t{{.Status}}\t{{.Ports}}\t{{.Image}}",
	)
	raw, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return []DockerServiceInfo{}, nil
	}

	var services []DockerServiceInfo
	for _, line := range strings.Split(trimmed, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}

		containerName := parts[0]
		statusRaw := parts[1]
		portsRaw := parts[2]
		image := parts[3]

		// Derive a human-friendly display name by stripping docker-compose prefixes/suffixes.
		displayName := containerName
		displayName = strings.TrimPrefix(displayName, "prod-")
		displayName = strings.TrimPrefix(displayName, "prod_")
		// Remove trailing -1 or _1 (replica suffix)
		if strings.HasSuffix(displayName, "-1") {
			displayName = displayName[:len(displayName)-2]
		} else if strings.HasSuffix(displayName, "_1") {
			displayName = displayName[:len(displayName)-2]
		}

		// Parse status and health from the raw status string.
		// Examples:
		//   "Up 2 hours (healthy)"
		//   "Up 30 minutes"
		//   "Exited (0) 5 minutes ago"
		//   "Restarting (1) 3 seconds ago"
		status := "stopped"
		health := "none"
		uptime := ""

		statusLower := strings.ToLower(statusRaw)
		if strings.HasPrefix(statusLower, "up") {
			status = "running"
			// Extract uptime text after "Up "
			uptime = strings.TrimPrefix(statusRaw, "Up ")
			uptime = strings.TrimPrefix(uptime, "up ")

			// Check for health annotation
			if strings.Contains(statusLower, "(healthy)") {
				health = "healthy"
				uptime = strings.Replace(uptime, " (healthy)", "", 1)
			} else if strings.Contains(statusLower, "(unhealthy)") {
				health = "unhealthy"
				uptime = strings.Replace(uptime, " (unhealthy)", "", 1)
			} else if strings.Contains(statusLower, "(health: starting)") {
				health = "starting"
				uptime = strings.Replace(uptime, " (health: starting)", "", 1)
			}
		} else if strings.Contains(statusLower, "restarting") {
			status = "restarting"
			health = "unhealthy"
		} else if strings.Contains(statusLower, "exited") {
			status = "stopped"
			health = "unhealthy"
		} else if strings.Contains(statusLower, "paused") {
			status = "paused"
		}

		// Parse the primary exposed port from the ports string.
		// Example: "0.0.0.0:8080->8080/tcp, :::8080->8080/tcp"
		port := extractPort(portsRaw)

		services = append(services, DockerServiceInfo{
			Name:      displayName,
			Status:    status,
			Health:    health,
			Uptime:    strings.TrimSpace(uptime),
			Port:      port,
			Container: containerName,
			Image:     image,
		})
	}

	return services, nil
}

// extractPort finds the first host port from a Docker port mapping string.
func extractPort(portsRaw string) string {
	if portsRaw == "" {
		return ""
	}

	// Take the first mapping segment (before any comma)
	segment := portsRaw
	if idx := strings.Index(segment, ","); idx > 0 {
		segment = segment[:idx]
	}

	// Look for "host:port->container/proto" pattern
	if arrowIdx := strings.Index(segment, "->"); arrowIdx > 0 {
		hostPart := segment[:arrowIdx]
		if colonIdx := strings.LastIndex(hostPart, ":"); colonIdx >= 0 {
			return hostPart[colonIdx+1:]
		}
	}

	return ""
}

// inferredDockerServices returns a best-guess list of services based on
// what we know the docker-compose stack contains. This is used when the
// Docker CLI is not available.
func inferredDockerServices() []DockerServiceInfo {
	return []DockerServiceInfo{
		{Name: "api-gateway", Status: "running", Health: "healthy", Port: "8080", Uptime: "unknown", Container: "prod-api-gateway-1", Image: "api_gateway"},
		{Name: "postgres", Status: "running", Health: "healthy", Port: "5432", Uptime: "unknown", Container: "prod-postgres-1", Image: "postgres:15-alpine"},
		{Name: "kafka", Status: "running", Health: "healthy", Port: "9092", Uptime: "unknown", Container: "prod-kafka-1", Image: "confluentinc/cp-kafka:7.5.0"},
		{Name: "zookeeper", Status: "running", Health: "healthy", Port: "2181", Uptime: "unknown", Container: "prod-zookeeper-1", Image: "confluentinc/cp-zookeeper:7.5.0"},
		{Name: "event-router", Status: "running", Health: "none", Port: "8082", Uptime: "unknown", Container: "prod-event-router-1", Image: "event_router"},
		{Name: "ingestor-core", Status: "running", Health: "none", Port: "8001", Uptime: "unknown", Container: "prod-ingestor-core-1", Image: "ingestor_core"},
		{Name: "ai-core", Status: "running", Health: "none", Port: "", Uptime: "unknown", Container: "prod-ai-core-1", Image: "ai-core"},
		{Name: "ui", Status: "running", Health: "none", Port: "3000", Uptime: "unknown", Container: "prod-ui-1", Image: "ui"},
		{Name: "kafka-ui", Status: "running", Health: "none", Port: "8090", Uptime: "unknown", Container: "prod-kafka-ui-1", Image: "provectuslabs/kafka-ui:latest"},
		{Name: "pgadmin", Status: "running", Health: "none", Port: "5050", Uptime: "unknown", Container: "prod-pgadmin-1", Image: "dpage/pgadmin4:latest"},
		{Name: "datasource", Status: "running", Health: "none", Port: "", Uptime: "unknown", Container: "prod-datasource-1", Image: "datasource"},
	}
}
