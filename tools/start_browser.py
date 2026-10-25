"""Start an isolated disposable Firefox profile. This tool changes desktop focus."""
import argparse
import json
import os
from pathlib import Path
import socket
import subprocess
import time

from marionette import Marionette


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--state-dir", type=Path, required=True)
    parser.add_argument("--native-host", type=Path, required=True)
    parser.add_argument("--single-window", action="store_true")
    parser.add_argument("--runtime-dir", type=Path)
    args = parser.parse_args()
    state = args.state_dir.resolve()
    state.mkdir(mode=0o700)
    profile = state / "profile"
    profile.mkdir(mode=0o700)
    home, config = state / "home", state / "config"
    runtime = args.runtime_dir.resolve() if args.runtime_dir else state / "runtime"
    for path in [home, config, runtime]:
        path.mkdir(mode=0o700, exist_ok=True)
    addon_id = json.loads((Path(__file__).resolve().parents[1] / "extension/manifest.json").read_text())["browser_specific_settings"]["gecko"]["id"]
    manifest = {"name": "niri_firefox_router", "description": "Router native test host",
                "path": str(args.native_host.resolve()), "type": "stdio",
                "allowed_extensions": [addon_id]}
    for directory in [home / ".mozilla/native-messaging-hosts", config / "mozilla/native-messaging-hosts"]:
        directory.mkdir(parents=True)
        (directory / "niri_firefox_router.json").write_text(json.dumps(manifest))
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    preferences = {"marionette.port": port, "browser.shell.checkDefaultBrowser": False,
                   "browser.startup.homepage": "about:blank", "browser.aboutwelcome.enabled": False,
                   "datareporting.policy.dataSubmissionPolicyBypassNotification": True}
    (profile / "user.js").write_text("\n".join("user_pref(" + json.dumps(k) + ", " + json.dumps(v) + ");" for k, v in preferences.items()))
    env = dict(os.environ, HOME=str(home), XDG_CONFIG_HOME=str(config), XDG_RUNTIME_DIR=str(runtime), MOZ_HOME=str(home / ".mozilla"))
    display = os.environ.get("WAYLAND_DISPLAY", "")
    if display and not os.path.isabs(display):
        env["WAYLAND_DISPLAY"] = str(Path(os.environ["XDG_RUNTIME_DIR"]) / display)
    with (state / "firefox.log").open("w") as log:
        process = subprocess.Popen(["firefox", "--no-remote", "--profile", str(profile), "--marionette", "--remote-allow-system-access", "about:blank"], env=env, stdout=log, stderr=log)
    connection = {"profile": str(profile), "port": port, "pid": process.pid, "runtime_dir": str(runtime)}
    (state / "connection.json").write_text(json.dumps(connection))
    try:
        deadline = time.monotonic() + 20
        while True:
            try:
                client = Marionette(port)
                break
            except OSError:
                if time.monotonic() >= deadline or process.poll() is not None:
                    raise RuntimeError("test Firefox did not start")
                time.sleep(0.1)
        client.call("Addon:Install", {"path": str(Path(__file__).resolve().parents[1] / "extension"), "temporary": True})
        client.call("Marionette:SetContext", {"value": "chrome"})
        if not args.single_window:
            client.js("OpenBrowserWindow(); return true;")
        client.close()
        print(json.dumps(connection))
    except BaseException:
        process.terminate()
        process.wait(timeout=10)
        raise


if __name__ == "__main__":
    main()
