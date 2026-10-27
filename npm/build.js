#!/usr/bin/env node
'use strict';
// Builds the npm packages for one release (CLI-12, CLI-13):
//   node npm/build.js <version> <bin-dir> <out-dir>
// <bin-dir>/<goos>_<goarch>/specfork[.exe] -> <out-dir>/cli-<os>-<cpu>/ plus <out-dir>/specfork/.
// No postinstall script; every package carries the release version.

const fs = require('fs');
const path = require('path');

const [version, binDir, outDir] = process.argv.slice(2);
if (!version || !binDir || !outDir || !/^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$/.test(version)) {
  console.error('usage: node npm/build.js <version, e.g. 1.2.3> <bin-dir> <out-dir>');
  process.exit(2);
}

const targets = JSON.parse(fs.readFileSync(path.join(__dirname, 'targets.json'), 'utf8'));
const main = JSON.parse(fs.readFileSync(path.join(__dirname, 'specfork', 'package.json'), 'utf8'));
const common = {
  homepage: main.homepage,
  repository: main.repository,
};

main.version = version;
main.optionalDependencies = {};

for (const t of targets) {
  const name = `@specfork/cli-${t.os}-${t.cpu}`;
  const exe = t.os === 'win32' ? 'specfork.exe' : 'specfork';
  const dir = path.join(outDir, `cli-${t.os}-${t.cpu}`);
  fs.mkdirSync(path.join(dir, 'bin'), { recursive: true });
  fs.copyFileSync(path.join(binDir, `${t.goos}_${t.goarch}`, exe), path.join(dir, 'bin', exe));
  fs.chmodSync(path.join(dir, 'bin', exe), 0o755);
  fs.writeFileSync(
    path.join(dir, 'package.json'),
    JSON.stringify({
      name,
      version,
      description: `Specfork CLI binary for ${t.os} ${t.cpu}`,
      ...common,
      os: [t.os],
      cpu: [t.cpu],
      files: ['bin/'],
    }, null, 2) + '\n',
  );
  main.optionalDependencies[name] = version;
}

const dir = path.join(outDir, 'specfork');
fs.mkdirSync(path.join(dir, 'bin'), { recursive: true });
fs.copyFileSync(path.join(__dirname, 'specfork', 'lib.js'), path.join(dir, 'lib.js'));
fs.copyFileSync(path.join(__dirname, 'specfork', 'bin', 'specfork.js'), path.join(dir, 'bin', 'specfork.js'));
fs.chmodSync(path.join(dir, 'bin', 'specfork.js'), 0o755);
fs.copyFileSync(path.join(__dirname, 'specfork', 'README.md'), path.join(dir, 'README.md'));
fs.writeFileSync(path.join(dir, 'package.json'), JSON.stringify(main, null, 2) + '\n');
