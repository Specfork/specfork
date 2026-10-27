#!/usr/bin/env node
'use strict';
// Runs the prebuilt specfork binary; arguments, stdio and exit code pass through (CLI-12).

const { spawnSync } = require('child_process');
const { binaryPath } = require('../lib.js');

let bin;
try {
  bin = binaryPath();
} catch (e) {
  process.stderr.write(`specfork: ${e.message}\n`);
  process.exit(1);
}

const r = spawnSync(bin, process.argv.slice(2), { stdio: 'inherit' });
if (r.error) {
  process.stderr.write(`specfork: ${r.error.message}\n`);
  process.exit(1);
}
if (r.signal) {
  process.kill(process.pid, r.signal);
} else {
  process.exit(r.status === null ? 1 : r.status);
}
