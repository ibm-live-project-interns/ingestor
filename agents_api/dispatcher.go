package main

func DispatchEvent(event Event) UnifiedResponse {
	response, err := CallWatsonAI(event)
	if err != nil {
		// Safe fallback for demo / outages
		return UnifiedResponse{
			Severity:            "unknown",
			Explanation:         "AI processing failed: " + err.Error(),
			RecommendedAction:   "Check AI service or logs",
		}
	}
	return response
}
