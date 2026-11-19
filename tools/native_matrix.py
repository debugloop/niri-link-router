"""Test real Firefox routing on niri. This tool intentionally changes focus."""
import argparse
import http.server
import json
import os
import re
from pathlib import Path
import secrets
import socket
import subprocess
import tempfile
import threading
import time

from observe import Observer


def ipc(request):
    with socket.socket(socket.AF_UNIX) as sock:
        sock.settimeout(3)
        sock.connect(os.environ["NIRI_SOCKET"])
        sock.sendall((json.dumps(request) + "\n").encode())
        reply = json.loads(sock.makefile("rb").readline(4 * 1024 * 1024))
        if "Err" in reply:
            raise RuntimeError(reply["Err"])
        return reply["Ok"]


def focus(window):
    ipc({"Action": {"FocusWindow": {"id": window}}})
    time.sleep(0.2)


class Pages:
    def __enter__(self):
        self.requests = []
        pages = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def setup(self):
                super().setup()
                self.connection.settimeout(2)

            def do_GET(self):
                if not self.path.startswith("/nfr-"):
                    self.send_response(204)
                    self.end_headers()
                    return
                marker = self.path[1:]
                pages.requests.append(marker)
                data = ("<!doctype html><meta charset=utf-8><title>" + marker + "</title><h1>" + marker + "</h1><p>Router test. You can close this tab.</p>").encode()
                self.send_response(200)
                self.send_header("Content-Type", "text/html; charset=utf-8")
                self.send_header("Content-Length", str(len(data)))
                self.send_header("Cache-Control", "no-store")
                self.end_headers()
                self.wfile.write(data)

            def log_message(self, *_):
                pass

        self.server = http.server.HTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        return self

    def url(self, marker):
        return f"http://127.0.0.1:{self.server.server_port}/{marker}"

    def __exit__(self, *_):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(3)


def run_case(router, pages, name, source, target, recent, launcher=None, fallback=None, entry="direct"):
    if recent is not None:
        focus(recent)
    focus(source)
    marker = "nfr-" + name + "-" + secrets.token_hex(4)
    url = pages.url(marker)
    options = ["--fallback-workspace", fallback] if fallback else []
    if entry != "xdg-open":
        options += ["--source-window-id", str(source)]
    decision = json.loads(subprocess.check_output([str(router), "open", "--dry-run", *options, url], text=True))
    if decision["niri_window_id"] != target:
        raise RuntimeError(f"wrong preflight destination for {name}: {decision}")
    command = [str(router), "open", *options, url]
    if launcher:
        command = launcher(command, url)
    before = ipc("Windows")["Windows"]
    with Observer() as observer:
        started = time.monotonic()
        completed = subprocess.run(command, capture_output=True, timeout=10)
        deadline = time.monotonic() + 8
        while time.monotonic() < deadline:
            windows = ipc("Windows")["Windows"]
            matches = [w for w in windows if marker in (w.get("title") or "")]
            if any(w["id"] == target for w in matches):
                break
            time.sleep(0.05)
        time.sleep(0.5)
    windows = ipc("Windows")["Windows"]
    focused = [w["id"] for w in windows if w["is_focused"]]
    matches = [w["id"] for w in windows if marker in (w.get("title") or "")]
    result = {"name": name, "routing_decision": decision, "source": source, "target": target,
              "most_recent_browser_before_source": recent, "transport": "native-messaging",
              "cli_returncode": completed.returncode, "http_requests": pages.requests.count(marker),
              "selected_title_windows": matches, "focused_windows": focused,
              "new_firefox_windows": [w["id"] for w in windows if w["app_id"] == "firefox" and w["id"] not in {w["id"] for w in before}],
              "invoked_at": started, "observer_error": observer.failure, "events": observer.events}
    result["passed"] = completed.returncode == 0 and result["http_requests"] == 1 and matches == [target] and focused == [source] and not result["new_firefox_windows"] and observer.failure is None
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--router", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--plan", type=Path, required=True)
    parser.add_argument("--runtime-dir", type=Path)
    parser.add_argument("--fallback-workspace")
    args = parser.parse_args()
    if args.runtime_dir:
        os.environ["XDG_RUNTIME_DIR"] = str(args.runtime_dir.resolve())
    if args.output.exists():
        parser.error("the output file must not exist")
    plan = json.loads(args.plan.read_text())
    if not isinstance(plan, list) or not plan:
        parser.error("the plan must be a nonempty JSON array")
    for case in plan:
        if not isinstance(case, dict) or not isinstance(case.get("name"), str) or not re.fullmatch(r"[A-Za-z0-9_-]{1,80}", case["name"]):
            parser.error("each case needs a short alphanumeric name")
        for field in ["source", "target"]:
            if type(case.get(field)) is not int or not 0 <= case[field] < 2**64:
                parser.error("each case needs nonnegative source and target compositor IDs")
        if "recent" in case and case["recent"] is not None and (type(case["recent"]) is not int or not 0 <= case["recent"] < 2**64):
            parser.error("recent must be a compositor ID or null")
        if case.get("entry", "direct") not in ["direct", "niri-spawn", "xdg-open"]:
            parser.error("unsupported transport or entry point")
    results = []
    with Pages() as pages:
        for case in plan:
            entry = case.get("entry", "direct")
            launch = None
            if entry == "niri-spawn":
                launch = lambda command, url: ["niri", "msg", "action", "spawn", "--", "env", "XDG_RUNTIME_DIR=" + os.environ["XDG_RUNTIME_DIR"], *command]
            elif entry == "xdg-open":
                launch = lambda command, url: ["xdg-open", url]
            result = run_case(args.router, pages, case["name"], case["source"], case["target"], case.get("recent"), launch, args.fallback_workspace, entry)
            results.append(result)
            args.output.write_text(json.dumps(results, indent=2) + "\n")
            print(case["name"], "PASS" if result["passed"] else "FAIL", flush=True)
            if not result["passed"]:
                raise RuntimeError("native routing failed or focus changed during the test. See the trace")


if __name__ == "__main__":
    main()
