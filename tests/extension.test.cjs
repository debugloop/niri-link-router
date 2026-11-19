"use strict";
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");
vm.runInThisContext(fs.readFileSync("extension/core.js", "utf8"));
const now = 100000;
function request(changes = {}) {
  return { version: 1, type: "open", id: "a".repeat(32), window_id: 1,
    url: "https://example.org/a+b?q=%25#fragment", active: true, expires: now + 5000, ...changes };
}
function fixture() {
  const session = {}, local = {}, tabs = [], focused = [];
  const api = {
    storage: {
      session: { get: async () => ({ ...session }), set: async (value) => Object.assign(session, value) },
      local: { get: async () => ({ ...local }), set: async (value) => Object.assign(local, value) }
    },
    windows: { get: async () => ({ type: "normal", incognito: false }), update: async (...args) => focused.push(args) },
    tabs: { create: async (value) => { tabs.push(value); return { id: 100 }; } }
  };
  return { api, session, local, tabs, focused, service: NFR.createService(api, () => now) };
}
test("native request opens exactly one tab in its profile-local window", async () => {
  const f = fixture(), r = request();
  const reply = await f.service.open(r);
  assert.deepEqual(reply, { version: 1, type: "result", id: r.id, status: "opened" });
  assert.deepEqual(f.tabs, [{ windowId: 1, url: r.url, active: true }]);
  assert.deepEqual(f.focused, []);
  assert.deepEqual(f.local.receipts, [{ at: now, status: "opened" }]);
});
test("invalid input never creates a tab", async () => {
  for (const changes of [{ version: 2 }, { type: "pair" }, { id: "bad" }, { window_id: 0 }, { active: 1 },
    { expires: now }, { expires: now + 30001 }, { url: "file:///etc/passwd" }, { extra: true }]) {
    const f = fixture();
    assert.equal((await f.service.open(request(changes))).status, "rejected");
    assert.equal(f.tabs.length, 0);
  }
});
test("duplicates are uncertain, never an invitation to repeat ordinary delivery", async () => {
  const f = fixture();
  const replies = await Promise.all([f.service.open(request()), f.service.open(request())]);
  assert.deepEqual(replies.map((r) => r.status), ["opened", "uncertain"]);
  assert.equal(f.tabs.length, 1);
});
test("two profiles can both use window ID one", async () => {
  const first = fixture(), second = fixture();
  assert.equal((await first.service.open(request())).status, "opened");
  assert.equal((await second.service.open(request())).status, "opened");
  assert.equal(first.tabs[0].windowId, 1);
  assert.equal(second.tabs[0].windowId, 1);
});
test("inactive tab preserves destination window focus", async () => {
  const f = fixture();
  await f.service.open(request({ active: false }));
  assert.equal(f.tabs[0].active, false);
  assert.equal(f.focused.length, 0);
});
test("invalid destination and storage failures reject before tab creation", async () => {
  for (const cause of ["private", "popup", "missing", "storage", "corrupt", "capacity"]) {
    const f = fixture();
    if (cause === "private") f.api.windows.get = async () => ({ type: "normal", incognito: true });
    if (cause === "popup") f.api.windows.get = async () => ({ type: "popup", incognito: false });
    if (cause === "missing") f.api.windows.get = async () => { throw new Error("missing"); };
    if (cause === "storage") f.api.storage.session.set = async () => { throw new Error("write"); };
    if (cause === "corrupt") f.session.requests = ["invalid"];
    if (cause === "capacity") f.session.requests = Array.from({ length: 2048 }, (_, i) => [i.toString(16).padStart(32, "0"), now + 5000]);
    assert.equal((await f.service.open(request())).status, "rejected");
    assert.equal(f.tabs.length, 0);
  }
});
test("tab API errors are uncertain and never retried", async () => {
  const f = fixture(); let calls = 0;
  f.api.tabs.create = async () => { calls++; throw new Error("ambiguous failure"); };
  assert.equal((await f.service.open(request())).status, "uncertain");
  assert.equal(calls, 1);
});
test("tab creation preserves window focus even if recording the receipt fails", async () => {
  const f = fixture();
  let focusCalls = 0;
  f.api.windows.update = async () => { focusCalls++; };
  f.api.storage.local.set = async () => { throw new Error("receipt"); };
  assert.equal((await f.service.open(request())).status, "opened");
  assert.equal(f.tabs.length, 1);
  assert.equal(focusCalls, 0);
});
test("receipts discard unrelated fields and stay bounded", async () => {
  const f = fixture();
  f.local.receipts = Array.from({ length: 40 }, () => ({ at: now, status: "opened", url: "SECRET", key: "SECRET" }));
  await f.service.open(request());
  assert.equal(f.local.receipts.length, 32);
  assert.equal(JSON.stringify(f.local.receipts).includes("SECRET"), false);
});
test("URL and title validation", () => {
  for (const url of ["javascript:alert(1)", "https://user:secret@example.org/", "https://example.org/\n", "https://example.org/%xx", "https://example.org/\\bad", "https://example.org/\ud800"]) assert.equal(NFR.validURL(url), false);
  assert.equal(NFR.validURL("https://example.org/a+b?q=%25#fragment"), true);
  assert.equal(NFR.validTitleLabel("private"), true);
  for (const value of ["[nfr:1]", "a\nb", "x".repeat(65)]) assert.equal(NFR.validTitleLabel(value), false);
});
test("public branding avoids protected names without changing native identity", () => {
  const manifest = JSON.parse(fs.readFileSync("extension/manifest.json", "utf8"));
  assert.equal(manifest.name, "niri Link Router");
  assert.doesNotMatch(manifest.name, /mozilla|firefox/i);
  assert.equal(manifest.browser_specific_settings.gecko.id, "niri-firefox-router@danieln.de");
  const settings = fs.readFileSync("extension/options.html", "utf8");
  assert.ok(settings.includes(`<h1>${manifest.name}</h1>`));
  assert.ok(settings.includes(`<title>${manifest.name} settings</title>`));
});
test("manifest replaces URL handlers with a persistent native connection", () => {
  const manifest = JSON.parse(fs.readFileSync("extension/manifest.json", "utf8"));
  assert.equal(manifest.protocol_handlers, undefined);
  assert.equal(manifest.background.persistent, true);
  assert.deepEqual(manifest.permissions, ["storage", "nativeMessaging"]);
  for (const file of ["extension/handler.html", "extension/handler.js", "extension/pairing.js"]) assert.equal(fs.existsSync(file), false);
});
test("background owns native connection and reconnects without browser pages", async () => {
  function event() {
    const listeners = [];
    return { addListener(fn) { listeners.push(fn); }, emit(...args) { return listeners.map((fn) => fn(...args)); } };
  }
  const channels = [], timers = [], writes = [];
  const runtimeMessages = event();
  const browser = {
    runtime: {
      id: "router@test", getURL: (path) => "moz-extension://router/" + path, lastError: undefined,
      onMessage: runtimeMessages,
      connectNative(name) {
        assert.equal(name, "niri_firefox_router");
        const channel = { onMessage: event(), onDisconnect: event(), sent: [],
          postMessage(value) { this.sent.push(JSON.parse(JSON.stringify(value))); },
          disconnect() { this.onDisconnect.emit(); } };
        channels.push(channel); return channel;
      }
    },
    storage: { onChanged: event(), local: { remove: async (key) => writes.push(key), get: async () => ({}) },
      session: { remove: async (key) => writes.push(key), set: async (value) => writes.push(value) } },
    windows: { onCreated: event(), getAll: async () => [], update: async () => { throw new Error("startup must not focus windows"); } }
  };
  vm.runInNewContext(fs.readFileSync("extension/background.js", "utf8"), {
    browser, NFR, setTimeout: (fn, delay) => { const timer = { fn, delay }; timers.push(timer); return timer; },
    clearTimeout: () => {}
  });
  await new Promise(setImmediate);
  assert.equal(channels.length, 1);
  assert.deepEqual(channels[0].sent, [{ version: 1, type: "hello" }]);
  channels[0].onMessage.emit({ version: 1, type: "ready", firefox_pid: 123 });
  const sender = { id: "router@test", url: "moz-extension://router/options.html" };
  const state = await runtimeMessages.emit({ type: "bridge.status" }, sender)[0];
  assert.equal(state.status, "connected");
  assert.equal(runtimeMessages.emit({ type: "bridge.reconnect" }, { id: "website", url: "https://example.org/" })[0], undefined);
  channels[0].onDisconnect.emit();
  assert.equal(timers.length, 1);
  assert.equal(timers[0].delay, 250);
  timers[0].fn();
  assert.equal(channels.length, 2);
  assert.ok(writes.includes("key"));
  assert.ok(writes.includes("nonces"));
});
