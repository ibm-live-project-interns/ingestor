# ingestor_core/config.py

CONFIG = {
    "syslog": {
        "enabled": True,
        "protocol": "udp",        # for now we support UDP; TCP later
        "host": "0.0.0.0",
        "port": 5514
    },
    "snmp": {
        "enabled": True,
        "host": "0.0.0.0",
        "port": 9162
    },
    "api": {
        "host": "0.0.0.0",
        "port": 8000
    },
    "normalizer": {
        "url": "http://localhost:8002/normalize"  # to be implemented by normalizer team
    }
}
