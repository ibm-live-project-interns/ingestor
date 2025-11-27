# ingestor_core/forward_to_normalizer.py

import requests
from .config import CONFIG

NORMALIZER_URL = CONFIG["normalizer"]["url"]

def forward_to_normalizer(event: dict) -> dict:
    """
    Sends the incoming event (from syslog/snmp/API) to the Normalization Engine.
    For now, we just POST JSON to NORMALIZER_URL.
    """
    try:
        resp = requests.post(NORMALIZER_URL, json=event, timeout=5)
        return resp.json()
    except Exception as e:
        # Normalizer may not exist yet; we still return a safe structure
        return {"status": "normalizer_unreachable", "error": str(e)}
