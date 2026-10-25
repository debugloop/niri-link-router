"""Install the personal command and optional niri-only HTTP/HTTPS defaults."""
import argparse
import base64
import configparser
import hashlib
import io
import json
import os
import re
from pathlib import Path
import tempfile


DESKTOP = "niri-firefox-router.desktop"
HOST = "niri_firefox_router"
MANIFEST = json.loads((Path(__file__).resolve().parents[1] / "extension/manifest.json").read_text())
ADDON_ID = MANIFEST["browser_specific_settings"]["gecko"]["id"]
ADDON_NAME = MANIFEST["name"]


def locations():
    home = Path.home()
    config = Path(os.environ.get("XDG_CONFIG_HOME", home / ".config"))
    data = Path(os.environ.get("XDG_DATA_HOME", home / ".local/share"))
    state = Path(os.environ.get("XDG_STATE_HOME", home / ".local/state"))
    return home / ".local/bin/niri-firefox-router", data / "applications" / DESKTOP, config / "niri-mimeapps.list", state / "niri-firefox-router/install.json"


def digest(data):
    return hashlib.sha256(data).hexdigest()


def write_atomic(path, data, mode):
    path = Path(path)
    if path.is_symlink() or str(path.resolve()).startswith("/nix/store/"):
        raise ValueError("Refuse to replace a managed link: " + str(path))
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=".nfr-", dir=path.parent)
    try:
        with os.fdopen(fd, "wb") as out:
            out.write(data)
            os.fchmod(out.fileno(), mode)
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def defaults(data):
    config = configparser.ConfigParser(interpolation=None)
    config.optionxform = str
    if data:
        config.read_string(data.decode())
    if "Default Applications" not in config:
        config.add_section("Default Applications")
    for scheme in ["http", "https"]:
        config["Default Applications"]["x-scheme-handler/" + scheme] = DESKTOP + ";"
    out = io.StringIO()
    config.write(out, space_around_delimiters=False)
    return out.getvalue().encode()


def check_installed(state):
    for path, entry in state["files"].items():
        file = Path(path)
        if file.is_symlink() or not file.is_file() or digest(file.read_bytes()) != entry["installed_sha256"]:
            raise ValueError("Installed file changed. Refuse to overwrite it: " + path)


def restore(state):
    for path, entry in state["files"].items():
        if entry["before"] is None:
            Path(path).unlink(missing_ok=True)
        else:
            write_atomic(path, base64.b64decode(entry["before"]), entry["before_mode"])


def install(binary, native_host, default_browser=False):
    executable, desktop, mime, record = locations()
    old_state = json.loads(record.read_text()) if record.exists() else None
    if old_state:
        check_installed(old_state)
    elif executable.exists() or desktop.exists():
        raise ValueError("An untracked installation exists. Refuse to overwrite it")
    if not re.fullmatch(r"[A-Za-z0-9_./-]+", str(executable)):
        raise ValueError("This personal installer requires a home path without spaces or shell characters")
    host_executable = executable.with_name("niri-firefox-router-native-host")
    manifests = [Path.home() / ".mozilla/native-messaging-hosts" / (HOST + ".json"),
                 Path(os.environ.get("XDG_CONFIG_HOME", Path.home() / ".config")) / "mozilla/native-messaging-hosts" / (HOST + ".json")]
    manifest = (json.dumps({"name": HOST, "description": ADDON_NAME + " native bridge",
                            "path": str(host_executable), "type": "stdio", "allowed_extensions": [ADDON_ID]}) + "\n").encode()
    if not old_state and (host_executable.exists() or any(path.exists() for path in manifests)):
        raise ValueError("An untracked native host exists. Refuse to overwrite it")
    desktop_text = ("[Desktop Entry]\nType=Application\nName=" + ADDON_NAME + "\nExec=" + str(executable) + " open %u\nTerminal=false\nNoDisplay=true\nMimeType=x-scheme-handler/http;x-scheme-handler/https;\n").encode()
    changes = {executable: (Path(binary).read_bytes(), 0o755), host_executable: (Path(native_host).read_bytes(), 0o755), desktop: (desktop_text, 0o644)}
    changes.update({path: (manifest, 0o644) for path in manifests})
    if default_browser:
        changes[mime] = (defaults(mime.read_bytes() if mime.exists() else b""), 0o600)
    if old_state and str(mime) in old_state["files"] and mime not in changes:
        changes[mime] = (mime.read_bytes(), 0o600)
    state = {"files": {}}
    for path, (data, mode) in changes.items():
        if path.is_symlink() or str(path.resolve()).startswith("/nix/store/"):
            raise ValueError("Refuse to replace a managed link: " + str(path))
        previous = old_state["files"].get(str(path)) if old_state else None
        state["files"][str(path)] = {"before": previous["before"] if previous else base64.b64encode(path.read_bytes()).decode() if path.exists() else None,
                                   "before_mode": previous["before_mode"] if previous else path.stat().st_mode & 0o777 if path.exists() else mode,
                                   "installed_sha256": digest(data)}
    rollback = {"files": {str(path): {"before": base64.b64encode(path.read_bytes()).decode() if path.exists() else None, "before_mode": path.stat().st_mode & 0o777 if path.exists() else mode} for path, (_, mode) in changes.items()}}
    try:
        for path, (data, mode) in changes.items():
            write_atomic(path, data, mode)
        write_atomic(record, (json.dumps(state, indent=2) + "\n").encode(), 0o600)
    except BaseException:
        restore(rollback)
        raise
    return executable


def uninstall():
    *_, record = locations()
    state = json.loads(record.read_text())
    check_installed(state)
    restore(state)
    record.unlink()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=["install", "uninstall"])
    parser.add_argument("--binary", type=Path)
    parser.add_argument("--native-host", type=Path)
    parser.add_argument("--default-browser", action="store_true")
    args = parser.parse_args()
    if args.action == "install":
        if args.binary is None or args.native_host is None:
            parser.error("install needs --binary and --native-host")
        print("Installed:", install(args.binary, args.native_host, args.default_browser))
    else:
        uninstall()
        print("Removed the installation and restored previous defaults.")


if __name__ == "__main__":
    main()
