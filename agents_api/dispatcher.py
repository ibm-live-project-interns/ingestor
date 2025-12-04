# dispatcher.py

def dispatch_event(event: dict):
    """
    Dispatcher decides how to handle events.
    This is an MVP with simple rule-based routing.
    Later we will integrate RAG, LLM, Watson etc.
    """

    event_type = event.get("type", "unknown")

    # Simple decision logic (MVP)
    if event_type.lower() == "critical":
        severity = "high"
        explanation = "Critical event received. Detailed AI analysis pending."
        action = "Immediate attention recommended."
    
    elif event_type.lower() == "warning":
        severity = "medium"
        explanation = "Warning event received. Monitoring advised."
        action = "Check logs and device health soon."

    else:
        severity = "low"
        explanation = "Informational or unknown event type."
        action = "No immediate action required."

    # Unified response format
    return {
        "severity": severity,
        "explanation": explanation,
        "recommended_action": action
    }
