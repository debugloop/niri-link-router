"""Exercise the compiled CLI with real sockets and no running browser."""
import hashlib
import json
import os
from pathlib import Path
import queue
import shutil
import socket
import struct
import subprocess
import sys
import tempfile
import threading
import time
import unittest

ROOT = Path(__file__).resolve().parents[1]


def exact(stream, count):
    data = b""
    while len(data) < count:
        chunk = stream.recv(count - len(data))
        if not chunk:
            raise EOFError("incomplete native frame")
        data += chunk
    return data


def receive(stream):
    size = struct.unpack("<I", exact(stream, 4))[0]
    if not 0 < size <= 65536:
        raise ValueError("invalid native frame size")
    return json.loads(exact(stream, size))


def send(stream, value):
    data = json.dumps(value).encode()
    stream.sendall(struct.pack("<I", len(data)) + data)


class Smoke(unittest.TestCase):
    def test_compiled_cli(self):
        with tempfile.TemporaryDirectory(prefix="nfr-smoke-") as directory:
            root = Path(directory)
            binary = root / "router"
            subprocess.run(["go", "build", "-o", str(binary), "./cmd/niri-firefox-router"], cwd=ROOT, check=True)
            profile = root / "config/mozilla/firefox/test"
            profile.mkdir(parents=True)
            peer_binary = root / "firefox-peer"
            shutil.copyfile(shutil.which("bash"), peer_binary)
            peer_binary.chmod(0o700)
            peer = subprocess.Popen([str(peer_binary), "-c", "read -r -t 60"], stdin=subprocess.PIPE)
            (profile / "lock").symlink_to("127.0.0.2:+" + str(peer.pid))
            (profile.parent / "profiles.ini").write_text("[Profile0]\nIsRelative=1\nPath=test\n")
            runtime = root / "run"
            runtime.mkdir(mode=0o700)
            sockets = runtime / "niri-firefox-router"
            sockets.mkdir(mode=0o700)
            native_path = sockets / (hashlib.sha256(str(profile.resolve()).encode()).hexdigest()[:32] + ".sock")
            niri_path = root / "niri"
            compositor, native = socket.socket(socket.AF_UNIX), socket.socket(socket.AF_UNIX)
            compositor.bind(str(niri_path)); compositor.listen()
            native.bind(str(native_path)); native.listen()
            stop = threading.Event()
            ipc_requests, deliveries = [], []
            outcomes = queue.Queue()

            def niri_worker():
                while not stop.is_set():
                    try:
                        conn, _ = compositor.accept()
                    except OSError:
                        return
                    with conn:
                        line = conn.makefile("rb").readline()
                        if not line:
                            continue
                        request = json.loads(line)
                        ipc_requests.append(request)
                        if request == "Workspaces":
                            ok = {"Workspaces": [{"id": 0, "idx": 1, "output": "A", "is_focused": False}, {"id": 1, "idx": 2, "output": "A", "is_focused": True, "name": "web"}]}
                        elif request == "Windows":
                            ok = {"Windows": [{"id": 900, "pid": peer.pid, "workspace_id": 0, "app_id": "firefox", "title": "[nfr:3] Test"}, {"id": 901, "pid": os.getpid(), "workspace_id": 0, "app_id": "launcher"}, {"id": 902, "pid": 123, "workspace_id": 1, "app_id": "firefox", "title": "[nfr:4] Test"}]}
                        else:
                            ok = "Handled"
                        conn.sendall((json.dumps({"Ok": ok}) + "\n").encode())

            def native_worker():
                while not stop.is_set():
                    try:
                        conn, _ = native.accept()
                    except OSError:
                        return
                    with conn:
                        try:
                            send(conn, {"version": 1, "type": "ready", "firefox_pid": peer.pid})
                            request = receive(conn)
                            deliveries.append(request)
                            status = outcomes.get(timeout=3)
                            send(conn, {"version": 1, "type": "result", "id": request["id"], "status": status})
                        except (OSError, EOFError, queue.Empty):
                            pass

            workers = [threading.Thread(target=fn, daemon=True) for fn in (niri_worker, native_worker)]
            for worker in workers:
                worker.start()
            env = dict(os.environ, NIRI_SOCKET=str(niri_path), XDG_CONFIG_HOME=str(root / "config"), XDG_RUNTIME_DIR=str(runtime), HOME=str(root))
            record = root / "ordinary.json"
            fake = root / "firefox"
            fake.write_text("#!" + sys.executable + "\nimport json,sys\nfrom pathlib import Path\nPath(" + repr(str(record)) + ").write_text(json.dumps(sys.argv[1:]))\n")
            fake.chmod(0o700)
            url = "https://example.org/a+b?q=%25#original"
            command = [str(binary), "open", "--firefox", str(fake), url]
            try:
                dry = subprocess.run([str(binary), "open", "--dry-run", url], env=env, check=True, capture_output=True, text=True)
                self.assertEqual(json.loads(dry.stdout)["firefox_window_id"], 3)
                self.assertNotIn(url, dry.stdout)
                self.assertEqual(deliveries, [])
                if shutil.which("xdg-open"):
                    applications = root / "data" / "applications"
                    applications.mkdir(parents=True)
                    (root / "config").mkdir(exist_ok=True)
                    (root / "config" / "mimeapps.list").write_text(
                        "[Default Applications]\nx-scheme-handler/https=nfr-test.desktop\n"
                    )
                    (applications / "nfr-test.desktop").write_text(
                        "[Desktop Entry]\nType=Application\nName=Router smoke test\n"
                        f"Exec={binary} open --dry-run %u\nMimeType=x-scheme-handler/https;\n"
                    )
                    desktop_env = dict(env, XDG_DATA_HOME=str(applications.parent), XDG_CURRENT_DESKTOP="generic",
                                       DE="generic", BROWSER="", DISPLAY=":999")
                    opened = subprocess.run(["xdg-open", url], env=desktop_env, check=True,
                                            capture_output=True, text=True, timeout=5)
                    self.assertEqual(json.loads(opened.stdout)["niri_window_id"], 900)
                outcomes.put("opened")
                result = subprocess.run(command, env=env, check=True, capture_output=True, text=True, timeout=5)
                self.assertIn("acknowledged tab creation", result.stdout)
                self.assertFalse(record.exists())
                self.assertEqual(len(deliveries), 1)
                self.assertEqual(deliveries[0]["url"], url)
                self.assertEqual(deliveries[0]["window_id"], 3)
                self.assertFalse(any("Action" in request for request in ipc_requests))
                self.assertTrue(deliveries[0]["active"])
                checked = subprocess.run(["node", "tests/verify-native.cjs"], cwd=ROOT, input=json.dumps(deliveries[0]), capture_output=True, text=True, check=True)
                self.assertEqual(json.loads(checked.stdout)["url"], url)
                outcomes.put("uncertain")
                result = subprocess.run(command, env=env, capture_output=True, text=True, timeout=5)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("uncertain", result.stderr)
                self.assertFalse(record.exists())
                self.assertEqual(len(deliveries), 2)
                outcomes.put("rejected")
                subprocess.run(command, env=env, check=True, capture_output=True, timeout=5)
                deadline = time.monotonic() + 2
                while not record.exists() and time.monotonic() < deadline:
                    time.sleep(0.01)
                self.assertEqual(json.loads(record.read_text()), [url])
                self.assertEqual(len(deliveries), 3)
            finally:
                stop.set()
                for path in [niri_path, native_path]:
                    try:
                        with socket.socket(socket.AF_UNIX) as wake:
                            wake.connect(str(path))
                    except OSError:
                        pass
                for worker in workers:
                    worker.join(3)
                compositor.close(); native.close()
                peer.terminate(); peer.wait(timeout=3); peer.stdin.close()


if __name__ == "__main__":
    unittest.main()
