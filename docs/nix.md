# Nix integration

The flake exports a static Go package with both executables, an unsigned development XPI, checks, a development shell, and optional modules.
There is no systemd service or native pairing secret.
Firefox owns one native-host process per participating profile.

## Modules

Import either `homeManagerModules.default` or `nixosModules.default`:

```nix
programs.niri-firefox-router = {
  enable = true;
  defaultBrowser = true;
  fallbackWorkspace = "web";
};
```

`defaultBrowser` defaults to false.
When enabled, the module sets only HTTP and HTTPS defaults to `niri-firefox-router.desktop`.
Other MIME associations remain unchanged.
The module does not create profiles, change niri bindings, or write extension storage.

Home Manager installs native-host manifests in both `~/.mozilla/native-messaging-hosts` and the matching directory under `xdg.configHome`.
This covers traditional and XDG Firefox user paths.
The NixOS module uses `programs.firefox.nativeMessagingHosts.packages` to expose the package manifest through the Firefox wrapper.
A changed wrapper can require a browser restart.

Remove any older personal installation before relying on the package desktop entry.
A file under `~/.local/share/applications` shadows the package entry even if both have the same desktop ID.
Upgrade the native host and extension together.
For a locked local flake input, refresh it with `nix flake update niri-firefox-router`
from the consuming configuration before rebuilding; editing the checkout does not refresh its locked snapshot.

## Runtime configuration

Home Manager installs `bg-open URL` for opening a link without switching focus.
It forwards to `niri-firefox-router open --bg`.

`fallbackWorkspace` defaults to `"web"` and accepts a nonempty workspace name.
Home Manager writes `niri-firefox-router/config.json` under `xdg.configHome`.
The NixOS module writes `/etc/niri-firefox-router/config.json`.
A user file overrides the system file. The `--fallback-workspace NAME` option overrides both.
The command reads these files on every invocation, so a name change needs no browser restart.

For `open`, the launching window’s workspace wins when it is known and has an eligible window. The `focus` command checks the focused workspace first.
If selection or native connection setup fails before submission, `open` invokes Firefox normally with the original URL.
An explicit rejection before tab creation also permits this fallback.
The command never automatically retries an uncertain submission.

## Signed extension

For manual installation, install [niri Link Router from AMO](https://addons.mozilla.org/en-US/firefox/addon/niri-link-router/)
in each participating profile. Leave `extensionPackage` unset; Firefox supplies add-on updates.

For optional Nix provisioning, use the exact signed XPI and its flat-file hash.
The approved 0.2.0 artifact is:

```nix
{ pkgs, inputs, ... }: {
  programs.niri-firefox-router.extensionPackage =
    inputs.niri-firefox-router.lib.mkSignedExtension {
      inherit pkgs;
      url = "https://addons.mozilla.org/firefox/downloads/file/5252374/niri_link_router-0.2.0.xpi";
      hash = "sha256-IWxda2ETivGObqJ3piJx+CqEA7+x9BZduboKDxmgvDg=";
    };
}
```

The helper preserves the downloaded file unchanged and exposes the add-on ID.
Firefox checks the Mozilla signature. The helper does not make unsigned XPIs trusted.
Home Manager uses `programs.firefox.globalExtensions`. NixOS uses `ExtensionSettings` policies.
Both disable independent online updates for the router extension only.
Existing profiles and other extensions remain unchanged.

Without `extensionPackage`, install the extension separately.
Load temporary development add-ons again after Firefox exits.
The native host starts automatically when the extension connects. There is no pair command.

## Migration from 0.1

Install both new executables and their native-host registration, then reload or update the extension.
Its settings must report a connected native bridge before routing is assumed to work.
Do not keep old binaries or desktop overrides beside the Nix package.

The old master-key file and its persistence entry are unused by 0.2.
Remove them only after no old installation still needs them.
The new runtime needs only its generated configuration and the existing Firefox profile storage.
Sockets and ownership locks belong under `XDG_RUNTIME_DIR`, not persistent home storage.
