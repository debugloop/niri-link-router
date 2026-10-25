# Privacy statement for niri Link Router

This statement covers the Firefox extension and its niri-firefox-router companion application.

## Data use

The companion command receives an HTTP or HTTPS URL that you ask it to open.
It reads local niri workspace and window information to select a Firefox window.
It resolves the Firefox process to its local profile so it can contact the correct native host.
The native host passes the requested URL and destination window ID to the extension on the same computer.
The extension asks Firefox to open that URL.

The extension does not read the contents of web pages, browser history, passwords, or cookies.
It does not send browsing data to the developer or an analytics service.
It has no analytics, advertising, or remote application service.
Firefox contacts the requested website normally, under that website's privacy policy.
Firefox and AMO can also contact Mozilla for normal extension installation and updates.

## Local storage

The extension stores an optional window label and up to 32 delivery records in the Firefox profile.
Each record contains only a timestamp and a result: `opened`, `rejected`, or `uncertain`.
These records do not contain URLs, page titles, or page contents.

Session storage holds temporary request IDs and expiry times to prevent duplicate delivery.
It also holds the native-bridge connection status.
Request URLs are not stored in these records.
Firefox can retain opened URLs in its own history and session data under your browser settings.

The native host uses private runtime sockets and ownership locks.
Their names contain a hash derived from the local profile path, not the URL.
Normal diagnostic output does not include request URLs or native-message payloads.
No URL history or persistent routing cache belongs to the companion application.

## Visible labels

The extension adds a numeric `[nfr:<id>]` prefix to Firefox window titles.
If you set a profile-wide label, that label also appears in window titles.
Other desktop applications and screen-sharing tools can see these titles under your desktop permissions.
Do not put secrets in the label.

## Your controls

You choose which links to send through the companion command.
The extension does not support private browsing windows.
You can change or clear the optional label in the extension settings.
You can disable or remove the extension through Firefox's add-on manager.
Remove its stored profile data through Firefox if you also want to remove local settings and records.

When routing is unavailable, the command can pass the URL to Firefox normally.
Firefox then chooses the destination profile and window.
Disable the companion command's URL-handler integration if you want to stop that behavior too.
