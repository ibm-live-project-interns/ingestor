package health

import (
	"net/http"
	"time"
)

func CheckHTTPHealth(url string) string {
	client := http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get(url + "/health")
	if err != nil {
		return "unreachable"
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "unhealthy"
	}

	return "healthy"
}
