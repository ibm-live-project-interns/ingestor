# ingestor_core/syslog_listener.py

import socketserver
import threading
from .forward_to_normalizer import forward_to_normalizer

class SyslogUDPHandler(socketserver.BaseRequestHandler):
    """
    Handles incoming Syslog messages over UDP.
    Each message is treated as a raw log line and wrapped into an event dict.
    """

    def handle(self):
        data = self.request[0].strip()
        message = data.decode("utf-8", errors="ignore")

        event = {
            "source": "syslog",
            "remote_addr": self.client_address[0],
            "raw_message": message
        }

        # Forward to Normalizer
        _ = forward_to_normalizer(event)


def start_syslog_udp_server(host: str, port: int):
    server = socketserver.UDPServer((host, port), SyslogUDPHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server
