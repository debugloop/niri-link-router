"use strict";
(() => {
  const fail = (code) => { throw new Error(code); };
  function validURL(raw) {
    if (typeof raw !== "string" || new TextEncoder().encode(raw).length > 8192 ||
        /[\s\p{Cc}\\]/u.test(raw) || /%(?![\da-f]{2})/i.test(raw) ||
        !/^https?:\/\/[^/?#]+/i.test(raw)) return false;
    // Reject lone surrogates before URL's replacement-character conversion.
    if (new TextDecoder("utf-8", { fatal: true }).decode(new TextEncoder().encode(raw)) !== raw) return false;
    try {
      const authority = raw.match(/^https?:\/\/([^/?#]+)/i)[1];
      const u = new URL(raw);
      return ["http:", "https:"].includes(u.protocol) && !!u.hostname &&
        !authority.includes("@") && !u.username && !u.password;
    } catch { return false; }
  }

  function validTitleLabel(label) {
    return typeof label === "string" && label.length <= 64 && !/[\p{Cc}\p{Zl}\p{Zp}]/u.test(label) && !label.includes("[nfr:");
  }
  function validRequest(request, now) {
    if (!request || typeof request !== "object" || Array.isArray(request)) return false;
    const keys = ["version", "type", "id", "window_id", "url", "active", "expires"];
    if (Object.keys(request).length !== keys.length || keys.some((key) => !Object.hasOwn(request, key))) return false;
    return request.version === 1 && request.type === "open" && /^[a-f0-9]{32}$/.test(request.id) &&
      Number.isInteger(request.window_id) && request.window_id > 0 && request.window_id <= 2147483647 &&
      validURL(request.url) && typeof request.active === "boolean" && Number.isSafeInteger(request.expires) &&
      request.expires > now && request.expires <= now + 30000;
  }
  function createService(api, now = Date.now) {
    let queue = Promise.resolve();
    async function reserve(request) {
      const { requests = [] } = await api.storage.session.get("requests");
      if (!Array.isArray(requests) || requests.some((entry) => !Array.isArray(entry) || entry.length !== 2 ||
          !/^[a-f0-9]{32}$/.test(entry[0]) || !Number.isSafeInteger(entry[1]))) fail("request-storage");
      const live = requests.filter((entry) => entry[1] > now());
      if (live.some((entry) => entry[0] === request.id)) fail("duplicate");
      if (live.length >= 2048) fail("request-capacity");
      await api.storage.session.set({ requests: [...live, [request.id, request.expires]] });
    }
    async function receipt(status) {
      try {
        const { receipts = [] } = await api.storage.local.get("receipts");
        const safe = Array.isArray(receipts) ? receipts.filter((entry) => entry && Number.isSafeInteger(entry.at) &&
          ["opened", "rejected", "uncertain"].includes(entry.status)).map(({ at, status }) => ({ at, status })) : [];
        await api.storage.local.set({ receipts: [...safe.slice(-31), { at: now(), status }] });
      } catch { /* A receipt error must not repeat a delivery. */ }
    }
    async function deliver(request) {
      let status = "rejected";
      try {
        if (!validRequest(request, now())) fail("request");
        await reserve(request);
        const win = await api.windows.get(request.window_id);
        if (win.type !== "normal" || win.incognito || request.expires <= now()) fail("destination");
        status = "uncertain";
        await api.tabs.create({ windowId: request.window_id, url: request.url, active: request.active });
        status = "opened";
      } catch (error) {
        if (error?.message === "duplicate") status = "uncertain";
      }
      await receipt(status);
      return { version: 1, type: "result", id: request?.id ?? "", status };
    }
    function open(request) {
      const result = queue.then(() => deliver(request));
      queue = result.catch(() => {});
      return result;
    }
    return { open };
  }
  globalThis.NFR = { validURL, validTitleLabel, validRequest, createService };
})();
