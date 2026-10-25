import configparser
import importlib.util
import os
from pathlib import Path
import tempfile
import shutil
import subprocess
import sys
import json
import unittest
from unittest.mock import patch

SOURCE = Path(__file__).resolve().parents[1] / "tools/install_user.py"
spec = importlib.util.spec_from_file_location("install_user", SOURCE)
installer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(installer)


class InstallTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.environment = patch.dict(os.environ, {"HOME": str(self.root), "XDG_CONFIG_HOME": str(self.root / "config"), "XDG_DATA_HOME": str(self.root / "data"), "XDG_STATE_HOME": str(self.root / "state")})
        self.environment.start()
        self.addCleanup(self.environment.stop)
        self.binary = self.root / "built-command"
        self.binary.write_bytes(b"command bytes")
        self.native_host = self.root / "built-native-host"
        self.native_host.write_bytes(b"native host bytes")

    def test_install_and_uninstall(self):
        executable, desktop, mime, record = installer.locations()
        installer.install(self.binary, self.native_host, True)
        self.assertEqual(executable.read_bytes(), self.binary.read_bytes())
        self.assertEqual(executable.stat().st_mode & 0o777, 0o755)
        self.assertIn(" open %u", desktop.read_text())
        self.assertIn("Name=niri Link Router\n", desktop.read_text())
        host = executable.with_name("niri-firefox-router-native-host")
        self.assertEqual(host.read_bytes(), self.native_host.read_bytes())
        manifest = self.root / ".mozilla/native-messaging-hosts/niri_firefox_router.json"
        config_manifest = self.root / "config/mozilla/native-messaging-hosts/niri_firefox_router.json"
        self.assertEqual(manifest.read_bytes(), config_manifest.read_bytes())
        self.assertEqual(json.loads(manifest.read_text())["path"], str(host))
        self.assertEqual(json.loads(manifest.read_text())["allowed_extensions"], [installer.ADDON_ID])
        self.assertEqual(json.loads(manifest.read_text())["description"], "niri Link Router native bridge")
        self.assertNotIn("ext+nfr", desktop.read_text() + mime.read_text())
        self.assertTrue(record.exists())
        installer.uninstall()
        for file in [executable, host, manifest, config_manifest, desktop, mime, record]:
            self.assertFalse(file.exists())

    def test_restore_existing_desktop_defaults(self):
        *_, mime, record = installer.locations()
        mime.parent.mkdir(parents=True)
        original = b"# Keep these defaults\n[Default Applications]\nx-scheme-handler/mailto=mail.desktop;\nx-scheme-handler/http=old.desktop;\n"
        mime.write_bytes(original)
        mime.chmod(0o640)
        installer.install(self.binary, self.native_host, True)
        config = configparser.ConfigParser()
        config.read(mime)
        self.assertEqual(config["Default Applications"]["x-scheme-handler/mailto"], "mail.desktop;")
        self.assertEqual(config["Default Applications"]["x-scheme-handler/https"], installer.DESKTOP + ";")
        installer.uninstall()
        self.assertEqual(mime.read_bytes(), original)
        self.assertEqual(mime.stat().st_mode & 0o777, 0o640)

    def test_reinstall_preserves_original_backup(self):
        installer.install(self.binary, self.native_host, True)
        self.binary.write_bytes(b"new command")
        installer.install(self.binary, self.native_host, True)
        installer.uninstall()
        self.assertFalse(installer.locations()[0].exists())

    def test_no_default_registration_without_option(self):
        installer.install(self.binary, self.native_host)
        self.assertFalse(installer.locations()[2].exists())
        installer.uninstall()

    def test_refuse_managed_symlink(self):
        mime = installer.locations()[2]
        mime.parent.mkdir(parents=True)
        managed = self.root / "managed-config"
        managed.write_bytes(b"[Default Applications]\n")
        mime.symlink_to(managed)
        with self.assertRaises(ValueError):
            installer.install(self.binary, self.native_host, True)
        self.assertEqual(managed.read_bytes(), b"[Default Applications]\n")
        self.assertFalse(installer.locations()[0].exists())

    def test_uninstall_refuses_to_destroy_manual_changes(self):
        installer.install(self.binary, self.native_host, True)
        mime = installer.locations()[2]
        mime.write_text(mime.read_text() + "# User change\n")
        with self.assertRaises(ValueError):
            installer.uninstall()
        self.assertTrue(installer.locations()[0].exists())

    def test_xdg_mime_finds_the_desktop_executable(self):
        if not shutil.which("xdg-mime"):
            self.skipTest("xdg-mime is unavailable")
        installer.install(self.binary, self.native_host, True)
        with patch.dict(os.environ, {"XDG_CURRENT_DESKTOP": "niri"}):
            result = subprocess.check_output(["xdg-mime", "query", "default", "x-scheme-handler/http"], text=True, timeout=5)
        self.assertEqual(result.strip(), installer.DESKTOP)

    def test_xdg_open_preserves_url_arguments(self):
        if not shutil.which("xdg-open"):
            self.skipTest("xdg-open is unavailable")
        record = self.root / "arguments.json"
        self.binary.write_text("#!" + sys.executable + "\nimport json,sys\nfrom pathlib import Path\nPath(" + repr(str(record)) + ").write_text(json.dumps(sys.argv[1:]))\n")
        installer.install(self.binary, self.native_host, True)
        url = "http://example.invalid/a+b?q=%25&b=+;x#fragment"
        with patch.dict(os.environ, {"XDG_CURRENT_DESKTOP": "niri", "BROWSER": "/bin/false", "NIXOS_XDG_OPEN_USE_PORTAL": "", "DISPLAY": ":99"}):
            subprocess.run(["xdg-open", url], check=True, capture_output=True, timeout=5)
        self.assertEqual(json.loads(record.read_text()), ["open", url])

    def test_partial_install_rolls_back(self):
        executable, desktop, mime, record = installer.locations()
        real = installer.write_atomic
        def fail_desktop(path, *args):
            if path == desktop:
                raise OSError("test write failure")
            return real(path, *args)
        with patch.object(installer, "write_atomic", side_effect=fail_desktop):
            with self.assertRaises(OSError):
                installer.install(self.binary, self.native_host, True)
        self.assertFalse(executable.exists())
        self.assertFalse(record.exists())


if __name__ == "__main__":
    unittest.main()
