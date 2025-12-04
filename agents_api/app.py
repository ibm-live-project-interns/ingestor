from fastapi import FastAPI
from pydantic import BaseModel

app = FastAPI(title="Core Agents API Service")

# STEP 1: Event validation
class Event(BaseModel):
    type: str
    message: str
    severity: str | None = None

# STEP 2: Main endpoint to receive events
@app.post("/events")
def handle_event(event: Event):
    # STEP 3: Basic logging
    print("[Core Agents API] Received event:", event.dict())

    # STEP 4: Unified placeholder response (MVP)
    response = {
        "severity": "medium",
        "explanation": "This is a placeholder response from Agents API.",
        "recommended_action": "Further processing (RAG/LLM) not implemented yet."
    }

    return response
