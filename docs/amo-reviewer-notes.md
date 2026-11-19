# AMO reviewer notes

Before submission, replace `SOURCE_REVISION` with the published commit ID for this version.
Do not paste this instruction paragraph into AMO.
The sections below form the reviewer note.

## Purpose and environment

niri Link Router is the Firefox component of niri-firefox-router 0.2.3.
It opens externally requested links in a selected Firefox window on Linux with the niri compositor.
It requires a separate companion application and a registered native-messaging host.
It does not work as a standalone extension or on Android, Windows, or macOS.
No account, credentials, subscription, or test login is required.

Project and companion source: <https://github.com/debugloop/niri-link-router>
Source revision for this submission: `SOURCE_REVISION`

The extension requires Firefox 142 or later on native Wayland.
Private browsing windows and sandboxed native-messaging portals are not supported.

## Source and build

The XPI contains the complete readable extension source from `extension/`.
There is no minification, transpilation, bundling, generated JavaScript, or third-party JavaScript dependency.
No extension build step is required beyond ZIP packaging.

The companion application contains two Go executables with standard-library dependencies only.
Its source is in `cmd/` and `internal/` in the matching repository revision.
The repository includes build instructions, automated tests, and the native-host manifest generator.
The extension does not download or execute a companion binary.
The user installs that application separately.

## Permissions

- `nativeMessaging`: Connect to the registered `niri_firefox_router` native host.
  Receive a requested URL and window ID, then return a request ID and delivery status.
- `storage`: Keep an optional profile label, 32 URL-free delivery records, and temporary duplicate-request state.

The extension needs no host permissions or broad `tabs` permission.
It creates tabs but does not read page contents, history, passwords, or cookies.
It has no content scripts, web-accessible resources, remote scripts, or custom protocol handler.

The data-collection declaration is `none`.
The extension does not send browsing data to the developer or a remote service.
URLs enter the extension from the local companion application at the user's request.
Native replies contain only the protocol version, request ID, and delivery status.
Firefox contacts requested websites normally when it opens their pages.

## Native host and local transport

Firefox starts one native host per participating profile through `runtime.connectNative`.
The native-host registration allows only the submitted extension ID.
The host identifies its Firefox ancestor and locked profile through local Linux process and profile information.
The command independently resolves the destination Firefox process to the same profile.

The command and host exchange bounded messages through a private Unix socket under `XDG_RUNTIME_DIR`.
Both endpoints check local ownership and Unix peer credentials.
The socket has mode 0600.
There is no TCP listener, remote server, independent system service, or periodic polling.
The native port controls the host's lifetime.
The extension makes a bounded number of reconnection attempts and exposes a manual reconnect button.

The extension accepts only validated HTTP and HTTPS requests with a window ID, random request ID, and short expiry.
It rejects private or non-normal destination windows.
It reserves request IDs before it creates tabs to prevent duplicate delivery.
After uncertain submission, the command does not retry blindly.
Normal logs and local receipts omit URLs.

## Visible behavior

Normal Firefox windows receive a `[nfr:<id>]` title prefix.
This identifies the Firefox-local window ID to the compositor-side command.
An optional profile-wide label follows the prefix.
The label is a display setting, not a secret or authentication mechanism.
Window Titler must remain disabled because it can overwrite this prefix.

The new tab is selected and Firefox takes focus, including a workspace switch when required.
There is no temporary extension page or protocol-handler tab.
The command uses ordinary Firefox handling if routing is unavailable before submission.
In that case, Firefox chooses the destination window.

## Manual test

Use a disposable Linux account or niri session.
The manual installer below writes user-level files, so do not run it over a managed installation.
It does not change default URL handlers unless `--default-browser` is supplied.
These tests intentionally change desktop focus.

### Install the companion application

From the matching source revision, with Go 1.23 or later and Python 3:

```sh
mkdir -p dist
CGO_ENABLED=0 go build -trimpath -o dist/ \
  ./cmd/niri-firefox-router ./cmd/niri-firefox-router-native-host
python3 tools/install_user.py install \
  --binary dist/niri-firefox-router \
  --native-host dist/niri-firefox-router-native-host
```

Use a regular, non-sandboxed Firefox package on native Wayland.
Install the submitted extension in the review profile.
For a source-only test before signing, temporarily load `extension/manifest.json` through `about:debugging`.
A temporary installation does not test restart persistence.

Open the extension settings through `about:addons`.
The native bridge must report **Connected**.
Normal window titles must start with `[nfr:<id>]`.
If the host is unavailable, the settings report **Disconnected**.
The settings also offer **Reconnect native bridge**.

### Current workspace

1. Open two normal Firefox windows in the same profile.
2. Put them on different niri workspaces.
3. Focus a terminal on the workspace with the first Firefox window.
4. Run this command:

```sh
./dist/niri-firefox-router open 'https://example.org/?nfr-test=local'
```

The first Firefox window must receive one selected tab and take focus.
The second window must not receive the tab, even if it previously had focus.

### Fallback workspace

Focus a terminal on a workspace with one Firefox window.
Give that workspace a test name:

```sh
niri msg action set-workspace-name nfr-review-web
```

Switch to a workspace with no Firefox window.
Run these commands from a terminal there:

```sh
./dist/niri-firefox-router open --dry-run \
  --fallback-workspace nfr-review-web 'https://example.org/'
./dist/niri-firefox-router open \
  --fallback-workspace nfr-review-web 'https://example.org/?nfr-test=fallback'
```

The dry run must show `normal_launch: false` and a positive `firefox_window_id`.
The actual open must select the new tab and focus Firefox on `nfr-review-web`.
The command must report that the extension acknowledged tab creation.

### Settings and restart

1. Set the optional label to `review` in the extension settings.
2. Check that normal window titles show `[nfr:<id>] review `.
3. Check that delivery records contain only timestamps and statuses.
4. With a signed installation, close all windows for the review profile.
5. Restart Firefox with that profile.
6. Check that the extension remains installed and the bridge reconnects.
7. Repeat the current-workspace and fallback tests.

To test profile separation, install the extension in a second ordinary profile.
Repeat the tests with windows from both profiles on different workspaces.
Only the selected destination profile must receive the new tab.

## Cleanup

Remove the extension from the disposable review profile.
Remove the companion installation with:

```sh
python3 tools/install_user.py uninstall
```

The installer checks for changed files before it removes them.
Discard the disposable account or restore its test workspace names afterward.
