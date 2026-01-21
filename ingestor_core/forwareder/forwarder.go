package forwarder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// Forward sends a routed event to the Event Router
func Forward(event models.RoutedEvent, eventRouterURL string) (string, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("failed to marshal routed event: %w", err)
	}

	url := eventRouterURL + "/route"
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return "", fmt.Errorf("failed to call event router: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read router response: %w", err)
	}

	return string(bodyBytes), nil
}
