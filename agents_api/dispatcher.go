package main

func DispatchEvent(event Event) UnifiedResponse {
	// Find relevant CVEs for RAG context
	relevantCVEs := FindRelevantCVEs(event.Message)

	response, err := CallWatsonAI(event, relevantCVEs)
	if err != nil {
		// Safe fallback for demo / outages
		return UnifiedResponse{
			Severity:          "unknown",
			Explanation:       "AI processing failed: " + err.Error(),
			RecommendedAction: "Check AI service or logs",
		}
	}
	return response
}
