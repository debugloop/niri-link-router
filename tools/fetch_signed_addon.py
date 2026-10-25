"""Collect an approved listed AMO release without repacking its signed XPI."""
import argparse
import base64
import hashlib
import io
import json
from pathlib import Path
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile

API = "https://addons.mozilla.org/api/v5/"
HOSTS = {"addons.mozilla.org", "addons.cdn.mozilla.net"}


def allowed_url(url):
    value = urllib.parse.urlsplit(url)
    return value.scheme == "https" and value.netloc in HOSTS and not value.fragment


class SafeRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, message, headers, newurl):
        if not allowed_url(newurl):
            raise ValueError("Refuse a redirect outside Mozilla's HTTPS download hosts")
        return super().redirect_request(request, fp, code, message, headers, newurl)


def read_url(url, limit):
    if not allowed_url(url):
        raise ValueError("Refuse a URL outside Mozilla's HTTPS download hosts")
    request = urllib.request.Request(url, headers={"User-Agent": "niri-firefox-router-release/0.1"})
    with urllib.request.build_opener(SafeRedirect()).open(request, timeout=20) as response:
        data = response.read(limit + 1)
    if len(data) > limit:
        raise ValueError("Mozilla response exceeds the size bound")
    return data


def find_version(addon_id, version, reader=read_url):
    prefix = API + "addons/addon/" + urllib.parse.quote(addon_id, safe="") + "/versions/"
    url = prefix + "?page_size=50"
    for _ in range(5):
        try:
            page = json.loads(reader(url, 4 * 1024 * 1024))
        except urllib.error.HTTPError as error:
            if error.code == 404:
                return None
            raise
        for item in page["results"]:
            if item["version"] == version and item["file"]["status"] == "public":
                return item["file"]
        url = page.get("next")
        if not url:
            return None
        if not url.startswith(prefix) or not allowed_url(url):
            raise ValueError("Unexpected AMO pagination URL")
    return None


def collect(addon_id, version, file, output, reader=read_url):
    if not re.fullmatch(r"[0-9][0-9A-Za-z.+_-]{0,63}", version):
        raise ValueError("Invalid release version")
    if file.get("status") != "public":
        raise ValueError("The listed AMO file is not public")
    url = file["url"]
    if not allowed_url(url) or not re.match(r"^/firefox/downloads/file/[1-9][0-9]*/", urllib.parse.urlsplit(url).path):
        raise ValueError("AMO did not return an immutable file download URL")
    expected = file["hash"]
    if not re.fullmatch(r"sha256:[a-f0-9]{64}", expected):
        raise ValueError("AMO did not return a SHA256 file hash")
    data = reader(url, 16 * 1024 * 1024)
    digest = hashlib.sha256(data).digest()
    if expected != "sha256:" + digest.hex():
        raise ValueError("Downloaded XPI does not match AMO's hash")
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        info = archive.getinfo("manifest.json")
        if info.file_size > 65536:
            raise ValueError("XPI manifest exceeds the size bound")
        manifest = json.loads(archive.read(info))
    identity = manifest.get("browser_specific_settings", manifest.get("applications", {})).get("gecko", {}).get("id")
    if identity != addon_id or manifest.get("version") != version:
        raise ValueError("Downloaded XPI identity or version does not match the release")
    name = f"niri-firefox-router-{version}-signed.xpi"
    metadata = {"addonId": addon_id, "version": version, "url": url, "hash": "sha256-" + base64.b64encode(digest).decode(), "filename": name}
    output = Path(output)
    output.mkdir(parents=True, exist_ok=True)
    (output / name).write_bytes(data)
    (output / "addon.json").write_text(json.dumps(metadata, indent=2) + "\n")
    (output / "SHA256SUMS").write_text(digest.hex() + "  " + name + "\n")
    return metadata


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--version", required=True)
    parser.add_argument("--output", type=Path, default=Path("signed-artifacts"))
    parser.add_argument("--wait", type=int, default=0, help="bounded approval wait in seconds, at most 1800")
    args = parser.parse_args()
    if not 0 <= args.wait <= 1800:
        parser.error("wait must be from 0 to 1800 seconds")
    manifest = json.loads((Path(__file__).resolve().parents[1] / "extension/manifest.json").read_text())
    if args.version != manifest["version"]:
        parser.error("check out the matching version tag before collecting its add-on")
    addon_id = manifest["browser_specific_settings"]["gecko"]["id"]
    deadline = time.monotonic() + args.wait
    while True:
        file = find_version(addon_id, args.version)
        if file:
            metadata = collect(addon_id, args.version, file, args.output)
            print("Collected approved AMO version", metadata["version"])
            return 0
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            print("The requested listed version is not public yet. Mozilla review may still be pending.", file=sys.stderr)
            return 75
        time.sleep(min(30, remaining))


if __name__ == "__main__":
    sys.exit(main())
