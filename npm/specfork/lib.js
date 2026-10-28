'use strict';
// Picks the platform package that holds the prebuilt binary (CLI-12).

const supported = ['linux', 'darwin', 'win32'].flatMap((os) => ['x64', 'arm64'].map((cpu) => `${os}-${cpu}`));

function platformPackage(platform = process.platform, arch = process.arch) {
  const key = `${platform}-${arch}`;
  return supported.includes(key) ? `@specfork/cli-${key}` : null;
}

// binaryPath returns the absolute path of the binary, or throws an Error whose message says what to do.
function binaryPath(platform = process.platform, arch = process.arch) {
  const pkg = platformPackage(platform, arch);
  if (!pkg) {
    throw new Error(`no prebuilt binary for ${platform}-${arch}; install with: go install github.com/specfork/specfork/cmd/specfork@latest`);
  }
  const file = `${pkg}/bin/specfork${platform === 'win32' ? '.exe' : ''}`;
  try {
    return require.resolve(file, { paths: [__dirname] });
  } catch (e) {
    throw new Error(`${pkg} is not installed; reinstall without --omit=optional (npm i -g specfork)`);
  }
}

module.exports = { platformPackage, binaryPath };
