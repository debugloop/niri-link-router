import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
import zipfile

SOURCE = Path(__file__).resolve().parents[1] / "tools/fetch_signed_addon.py"
spec = importlib.util.spec_from_file_location("fetch_signed_addon", SOURCE)
collector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(collector)
ID = "niri-firefox-router@danieln.de"
URL = "https://addons.mozilla.org/firefox/downloads/file/123/router.xpi"


def xpi(identity=ID, version="0.1.0"):
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, "w") as archive:
        archive.writestr("manifest.json", json.dumps({"version": version, "browser_specific_settings": {"gecko": {"id": identity}}}))
        archive.writestr("META-INF/fixture", "Test fixture, not a real Mozilla signature")
    return stream.getvalue()


def metadata(data):
    return {"url": URL, "hash": "sha256:" + hashlib.sha256(data).hexdigest(), "status": "public", "is_mozilla_signed_extension": False}


class FetchTests(unittest.TestCase):
    def test_collect_preserves_the_archive_bytes(self):
        data = xpi()
        with tempfile.TemporaryDirectory() as directory:
            result = collector.collect(ID, "0.1.0", metadata(data), directory, lambda url, limit: data)
            self.assertEqual((Path(directory) / result["filename"]).read_bytes(), data)
            self.assertTrue(result["hash"].startswith("sha256-"))
            self.assertEqual(json.loads((Path(directory) / "addon.json").read_text()), result)
            self.assertIn(hashlib.sha256(data).hexdigest(), (Path(directory) / "SHA256SUMS").read_text())

    def test_bad_hash_or_identity_writes_no_artifact(self):
        for data, changes in [(xpi(), {"hash": "sha256:" + "0" * 64}), (xpi("other@invalid"), {}), (xpi(version="0.2.0"), {}), (xpi(), {"status": "awaiting_review"})]:
            file = dict(metadata(data), **changes)
            with tempfile.TemporaryDirectory() as directory:
                with self.assertRaises(ValueError):
                    collector.collect(ID, "0.1.0", file, directory, lambda url, limit: data)
                self.assertEqual(list(Path(directory).iterdir()), [])

    def test_immutable_mozilla_https_url_required(self):
        data = xpi()
        for url in ["http://addons.mozilla.org/firefox/downloads/file/123/a.xpi", "https://evil.example/a.xpi", "https://addons.mozilla.org/firefox/downloads/latest/a.xpi", "https://addons.mozilla.org@evil.example/a.xpi"]:
            with self.assertRaises(ValueError):
                collector.collect(ID, "0.1.0", dict(metadata(data), url=url), "/unused", lambda url, limit: data)
        self.assertFalse(collector.allowed_url("https://addons.mozilla.org:8080/file"))

    def test_find_exact_public_version(self):
        data = xpi()
        page = {"results": [{"version": "0.2.0", "file": metadata(data)}, {"version": "0.1.0", "file": metadata(data)}], "next": None}
        self.assertEqual(collector.find_version(ID, "0.1.0", lambda url, limit: json.dumps(page).encode())["url"], URL)
        self.assertIsNone(collector.find_version(ID, "0.0.1", lambda url, limit: json.dumps(page).encode()))

    def test_pending_version_is_not_collected(self):
        page = {"results": [{"version": "0.1.0", "file": {"status": "awaiting_review"}}], "next": None}
        self.assertIsNone(collector.find_version(ID, "0.1.0", lambda url, limit: json.dumps(page).encode()))

    def test_pagination_cannot_leave_the_addon_endpoint(self):
        page = {"results": [], "next": "https://evil.example/metadata"}
        with self.assertRaises(ValueError):
            collector.find_version(ID, "0.1.0", lambda url, limit: json.dumps(page).encode())


if __name__ == "__main__":
    unittest.main()
