"""A stand-in for the app, for scripts/test-gate-check.sh only: answers like a hosted app behind the login
gate (mode "gated") or breaks one rule of the gate (the other modes). Binds 127.0.0.1 on a free port and
prints it. Not part of the product."""
import json
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

mode = sys.argv[1]
# "flaky": the first two requests to /api/auth/options get no answer (connection closed), then it is the gated app.
# "mute": /api/health answers, every other request gets no answer (an app that went away mid-check).
dropped = 0
APP = b'<html><div id="root"></div></html>'
UNAUTH = json.dumps({"error": "sign in", "code": "unauthenticated"}).encode()


class H(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def send(self, code, body=b"", ctype="application/json", extra=None):
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        for k, v in (extra or {}).items():
            self.send_header(k, v)
        self.end_headers()
        self.wfile.write(body)

    def handle_any(self):
        global dropped
        p = self.path.split("?")[0]
        if (mode == "mute" and p != "/api/health") or (mode == "flaky" and p == "/api/auth/options" and dropped < 2):
            dropped += 1
            self.close_connection = True
            return
        html = "text/html" in (self.headers.get("Accept") or "")
        if p == "/api/health":
            login = "dev" if mode == "devlogin" else "google"
            return self.send(200, json.dumps({"status": "ok", "commit": "x", "version": "1", "login": login}).encode())
        if p == "/api/auth/options":
            return self.send(200, json.dumps({"google": True, "devLogin": mode == "devroute"}).encode())
        if p == "/api/auth/dev":
            return self.send(200 if mode == "devroute" else 404, b"{}")
        if p == "/api/spells/names" and mode == "opencatalogue":
            return self.send(200, b"[]")
        if p == "/api/characters" and self.command == "POST" and mode == "openpost":
            return self.send(201, b"{}")
        if p.startswith("/api/"):
            return self.send(401, UNAUTH)
        if p == "/login":
            return self.send(200, APP, "text/html")
        if html and mode == "openapp" and p == "/":
            return self.send(200, APP, "text/html")
        if html:
            return self.send(302, b"", extra={"Location": "/login"})
        return self.send(404, b"{}")

    do_GET = do_POST = handle_any


s = HTTPServer(("127.0.0.1", 0), H)
print(s.server_address[1], flush=True)
s.serve_forever()
