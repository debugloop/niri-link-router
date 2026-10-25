"""Small Marionette client for the disposable native test browser."""
import json
import socket


class Marionette:
    def __init__(self, port):
        self.sock = socket.create_connection(("127.0.0.1", port), 10)
        self.number = 0
        self.read()
        self.call("WebDriver:NewSession", {"capabilities": {"alwaysMatch": {"acceptInsecureCerts": False}}})

    def read(self):
        prefix = b""
        while not prefix.endswith(b":"):
            part = self.sock.recv(1)
            if not part:
                raise EOFError("Marionette connection closed")
            prefix += part
            if len(prefix) > 12:
                raise ValueError("Marionette frame prefix exceeds bound")
        length = int(prefix[:-1])
        if not 0 <= length <= 16 * 1024 * 1024:
            raise ValueError("Marionette frame exceeds bound")
        body = b""
        while len(body) < length:
            part = self.sock.recv(length - len(body))
            if not part:
                raise EOFError("Marionette connection closed")
            body += part
        return json.loads(body)

    def call(self, name, args=None):
        self.number += 1
        body = json.dumps([0, self.number, name, args or {}]).encode()
        self.sock.sendall(str(len(body)).encode() + b":" + body)
        reply = self.read()
        if reply[0] != 1 or reply[1] != self.number:
            raise ValueError("Unexpected Marionette reply")
        if reply[2]:
            raise RuntimeError(reply[2])
        return reply[3]

    def js(self, script, args=None, async_=False):
        reply = self.call("WebDriver:ExecuteAsyncScript" if async_ else "WebDriver:ExecuteScript", {
            "script": script, "args": args or [], "newSandbox": True,
            "sandbox": "default", "scriptTimeout": 10000,
        })
        return reply.get("value", reply) if isinstance(reply, dict) else reply

    def close(self):
        self.sock.close()
