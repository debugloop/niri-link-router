"use strict";
const service = NFR.createService(browser);
let port;
let generation = 0;
let retryTimer;
let state = { status: "disconnected" };
const retryDelays = [250, 1000, 4000, 16000];
function setState(status) {
  state = { status };
  void browser.storage.session.set({ bridgeState: status }).catch(() => {});
}

function connect(attempt = 0) {
  clearTimeout(retryTimer);
  const current = ++generation;
  if (port) { try { port.disconnect(); } catch { /* The previous host can already be gone. */ } }
  setState("connecting");
  let channel;
  function disconnected() {
    void browser.runtime.lastError;
    if (current !== generation) return;
    port = undefined;
    setState("disconnected");
    if (attempt < retryDelays.length) retryTimer = setTimeout(() => connect(attempt + 1), retryDelays[attempt]);
  }
  try {
    channel = browser.runtime.connectNative("niri_firefox_router");
    port = channel;
    channel.onDisconnect.addListener(disconnected);
    channel.onMessage.addListener((message) => {
      if (current !== generation) return;
      if (message?.version === 1 && message.type === "ready" && Number.isInteger(message.firefox_pid) && message.firefox_pid > 0) {
        setState("connected");
        attempt = 0;
      } else if (state.status === "connected" && message?.type === "open") {
        void service.open(message).then((reply) => {
          try { channel.postMessage(reply); } catch { /* A lost acknowledgement must not repeat delivery. */ }
        });
      } else {
        try { channel.disconnect(); } catch { /* The connection can close first. */ }
      }
    });
    channel.postMessage({ version: 1, type: "hello" });
  } catch { disconnected(); }
}

browser.runtime.onMessage.addListener((message, sender) => {
  if (sender.id !== browser.runtime.id || sender.url !== browser.runtime.getURL("options.html")) return undefined;
  if (message?.type === "bridge.status") return Promise.resolve({ ...state });
  if (message?.type === "bridge.reconnect") { connect(); return Promise.resolve({ ...state }); }
  return undefined;
});

async function label(win) {
  if (win.type !== "normal" || win.incognito) return;
  try {
    const { titleLabel = "" } = await browser.storage.local.get("titleLabel");
    if (!NFR.validTitleLabel(titleLabel)) return;
    await browser.windows.update(win.id, { titlePreface: `[nfr:${win.id}] ${titleLabel ? titleLabel + " " : ""}` });
  } catch { /* A window can close during enumeration. */ }
}
async function enumerate() {
  try { await Promise.all((await browser.windows.getAll({ windowTypes: ["normal"] })).map(label)); }
  catch { /* A later window event can restore its label. */ }
}
browser.windows.onCreated.addListener(label);
browser.storage.onChanged.addListener((changes, area) => {
  if (area === "local" && changes.titleLabel) return enumerate();
  return undefined;
});
void (async () => {
  try { await browser.storage.local.remove("key"); await browser.storage.session.remove("nonces"); }
  catch { /* Legacy values have no role in the native transport. */ }
  connect();
  await enumerate();
})();
