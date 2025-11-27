# ingestor_core/app.py

from fastapi import FastAPI, HTTPException
from .config import CONFIG
from .forward_to_normalizer import forward_to_normalizer
from .syslog_listener import start_syslog_udp_server
from .snmp_listener import start_snmp_udp_server

app = FastAPI(title="Ingestor Core Service")

@app.on_event("startup")
def startup_event():
    """
    On startup, launch Syslog and SNMP UDP listeners based on config.
    """
    syslog_conf = CONFIG["syslog"]
    snmp_conf = CONFIG["snmp"]

    app.state.syslog_server = None
    app.state.snmp_server = None

    if syslog_conf["enabled"] and syslog_conf["protocol"].lower() == "udp":
        app.state.syslog_server = start_syslog_udp_server(
            syslog_conf["host"], syslog_conf["port"]
        )

    if snmp_conf["enabled"]:
        app.state.snmp_server = start_snmp_udp_server(
            snmp_conf["host"], snmp_conf["port"]
        )

@app.on_event("shutdown")
def shutdown_event():
    """
    Gracefully stop Syslog and SNMP servers when API shuts down.
    """
    if getattr(app.state, "syslog_server", None):
        app.state.syslog_server.shutdown()

    if getattr(app.state, "snmp_server", None):
        app.state.snmp_server.shutdown()


@app.post("/ingest/metadata")
async def ingest_metadata(metadata: dict):
    """
    API endpoint for receiving metadata from datasource or other services.
    """
    try:
        resp = forward_to_normalizer({
            "source": "metadata_api",
            "metadata": metadata,
        })
        return {
            "status": "received",
            "forwarded_to": "normalizer",
            "normalizer_response": resp
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
