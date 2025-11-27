import requests

URLS = {
    "agents_api": "http://localhost:8081/events",
    "rag_connector": "http://localhost:8082/ingest",
    "context_retrieval": "http://localhost:8083/context"
}

def send_to_service(service, event):
    url = URLS[service]
    response = requests.post(url, json=event)
    return response.status_code
