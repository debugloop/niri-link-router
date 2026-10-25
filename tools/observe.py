"""Record bounded niri events around a command. This is a test tool, not a service."""

import argparse
import json
import os
import socket
import subprocess
import threading
import time


def redact(value):
    if isinstance(value, dict):
        return {k: redact(v) for k, v in value.items() if k not in {"title", "url"}}
    if isinstance(value, list):
        return [redact(v) for v in value]
    return value


class Observer:
    def __enter__(self):
        self.events = []
        self.ready = threading.Event()
        self.failure = None
        self.sock = socket.socket(socket.AF_UNIX)
        self.sock.settimeout(3)
        self.sock.connect(os.environ["NIRI_SOCKET"])
        self.sock.sendall(b'"EventStream"\n')
        self.stream = self.sock.makefile("rb")
        if json.loads(self.stream.readline()) != {"Ok": "Handled"}:
            raise RuntimeError("niri rejected event stream")
        self.sock.settimeout(None)
        self.thread = threading.Thread(target=self.read, daemon=True)
        self.thread.start()
        if not self.ready.wait(3):
            self.__exit__(None, None, None)
            raise RuntimeError("niri initial events did not arrive")
        return self

    def read(self):
        initial = set()
        try:
            for line in self.stream:
                event = json.loads(line)
                self.events.append({"at": time.monotonic(), "event": redact(event)})
                initial.update(event)
                if {"WorkspacesChanged", "WindowsChanged"} <= initial:
                    self.ready.set()
        except (OSError, ValueError) as error:
            self.failure = str(error)

    def __exit__(self, *_):
        self.sock.shutdown(socket.SHUT_RDWR)
        self.thread.join(3)
        self.stream.close()
        self.sock.close()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--seconds", type=float, default=3)
    parser.add_argument("--output", required=True)
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    if not 0 < args.seconds <= 30 or not args.command:
        parser.error("supply a command and a duration from 0 to 30 seconds")
    command = args.command[1:] if args.command[0] == "--" else args.command
    with Observer() as observer:
        started = time.monotonic()
        result = subprocess.run(command, timeout=15, capture_output=True)
        time.sleep(args.seconds)
    with open(args.output, "x") as out:
        json.dump({"invoked_at": started, "returncode": result.returncode,
                   "observer_error": observer.failure, "events": observer.events}, out, indent=2)
    print(json.dumps({"returncode": result.returncode, "events": len(observer.events), "output": args.output}))


if __name__ == "__main__":
    main()
