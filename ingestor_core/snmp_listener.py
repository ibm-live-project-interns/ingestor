# ingestor_core/snmp_listener.py

import socketserver
import threading
from .forward_to_normalizer import forward_to_normalizer

class SNMPTrapUDPHandler(socketserver.BaseRequestHandler):
    """
    Very simple UDP listener for SNMP traps.
    We don't parse full SNMP yet; we just store raw bytes for MVP.
    """

    def handle(self):
        data = self.request[0].strip()

        event = {
            "source": "snmp_trap",
            "remote_addr": self.client_address[0],
            "raw_trap_bytes_hex": data.hex()
        }

        # Forward to Normalizer
        _ = forward_to_normalizer(event)


def start_snmp_udp_server(host: str, port: int):
    server = socketserver.UDPServer((host, port), SNMPTrapUDPHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server
