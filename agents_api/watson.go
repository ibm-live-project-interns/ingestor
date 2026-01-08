package main

// CallWatsonAI simulates calling IBM Watson.
// This is MOCKED until credentials are provided.
func CallWatsonAI(event Event) (UnifiedResponse, error) {

	// Simple rule-based logic to simulate AI
	switch event.Type {

	case "critical":
		return UnifiedResponse{
			Severity:          "high",
			Explanation:       "Critical system issue detected (mocked Watson response).",
			RecommendedAction: "Immediate investigation required.",
		}, nil

	case "warning":
		return UnifiedResponse{
			Severity:          "medium",
			Explanation:       "Warning detected in system behavior (mocked Watson response).",
			RecommendedAction: "Monitor the system closely.",
		}, nil

	default:
		return UnifiedResponse{
			Severity:          "low",
			Explanation:       "Informational event received (mocked Watson response).",
			RecommendedAction: "No immediate action required.",
		}, nil
	}
}
