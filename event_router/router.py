import json
from forwarder import send_to_service

with open("config.json") as f:
    ROUTES = json.load(f)

def route_event(event):
    event_type = event.get("type", "default")
    destination = ROUTES.get(event_type, ROUTES["default"])
    status = send_to_service(destination, event)
    return {"sent_to": destination, "status": status}
