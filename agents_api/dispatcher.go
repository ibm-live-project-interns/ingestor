package main

func DispatchEvent(event Event) UnifiedResponse {
    switch event.Type {
    case "critical":
        return UnifiedResponse{
            Severity:          "high",
            Explanation:       "Critical event received. Detailed AI analysis pending.",
            RecommendedAction: "Immediate attention recommended.",
        }
    case "warning":
        return UnifiedResponse{
            Severity:          "medium",
            Explanation:       "Warning event detected.",
            RecommendedAction: "Monitor the system.",
        }
    default:
        return UnifiedResponse{
            Severity:          "low",
            Explanation:       "Informational event.",
            RecommendedAction: "No action required.",
        }
    }
}
