"use strict";
const fs = require("node:fs");
const vm = require("node:vm");
vm.runInThisContext(fs.readFileSync("extension/core.js", "utf8"));
const request = JSON.parse(fs.readFileSync(0, "utf8"));
if (!NFR.validRequest(request, Date.now())) process.exit(1);
process.stdout.write(JSON.stringify({ url: request.url, window_id: request.window_id }));
