from fastapi import FastAPI
from pydantic import BaseModel
from agents_api.dispatcher import dispatch_event


app = FastAPI(title="Core Agents API Service")

# Event validation
class Event(BaseModel):
    type: str
    message: str
    severity: str | None = None

@app.post("/events")
def handle_event(event: Event):
    event_dict = event.dict()
    print("[Agents API] Incoming event →", event_dict)

    # Call dispatcher
    result = dispatch_event(event_dict)

    print("[Agents API] Dispatcher output →", result)
    return result
