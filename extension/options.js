"use strict";
async function refresh() {
  try {
    const result = await browser.runtime.sendMessage({ type: "bridge.status" });
    document.querySelector("#bridge-state").textContent = {
      connected: "Connected to this profile's native host.",
      connecting: "Connecting to the native host.",
      disconnected: "Native host disconnected. Ordinary Firefox handling remains available."
    }[result?.status] || "Could not read native bridge status.";
    const { receipts = [] } = await browser.storage.local.get("receipts");
    document.querySelector("#receipts").textContent = JSON.stringify(receipts, null, 2);
  } catch { document.querySelector("#bridge-state").textContent = "Could not read native bridge status."; }
}
document.querySelector("#reconnect").addEventListener("click", async () => {
  try { await browser.runtime.sendMessage({ type: "bridge.reconnect" }); }
  catch { document.querySelector("#bridge-state").textContent = "Could not reconnect the native host."; }
  await refresh();
});
document.querySelector("#title-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const titleLabel = document.querySelector("#title-label").value;
  const status = document.querySelector("#title-status");
  if (!NFR.validTitleLabel(titleLabel)) { status.textContent = "Use at most 64 characters, without control characters or [nfr:."; return; }
  try { await browser.storage.local.set({ titleLabel }); status.textContent = "Label saved for this profile."; }
  catch { status.textContent = "Could not save the label."; }
});
browser.storage.onChanged.addListener((changes, area) => {
  if (area === "session" && changes.bridgeState) void refresh();
});
document.querySelector("#refresh").addEventListener("click", refresh);
void (async () => {
  try {
    const { titleLabel = "" } = await browser.storage.local.get("titleLabel");
    document.querySelector("#title-label").value = titleLabel;
  } catch { document.querySelector("#title-status").textContent = "Could not read the label."; }
  await refresh();
})();
