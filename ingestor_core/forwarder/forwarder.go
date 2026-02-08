package forwarder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	log.Printf("[forwarder] Forwarding event to Event Router: %s (type=%s severity=%s source=%s)", url, event.EventType, event.Type, event.SourceHost)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return "", fmt.Errorf("failed to call event router at %s: %w", url, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read router response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[forwarder] ERROR: Event Router returned status %d: %s", resp.StatusCode, string(bodyBytes))
		return "", fmt.Errorf("event router returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	log.Printf("[forwarder] Event forwarded successfully (status=%d)", resp.StatusCode)
	return string(bodyBytes), nil
}
