const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const assert = require('node:assert/strict');

const mainSource = fs.readFileSync(path.join(__dirname, 'main.js'), 'utf8');
const packageConfig = JSON.parse(
  fs.readFileSync(path.join(__dirname, 'package.json'), 'utf8'),
);
const macEntitlements = fs.readFileSync(
  path.join(__dirname, 'entitlements.mac.plist'),
  'utf8',
);

test('Electron renderer is sandboxed and keeps browser security enabled', () => {
  assert.match(mainSource, /sandbox:\s*true/);
  assert.match(mainSource, /webSecurity:\s*true/);
  assert.match(mainSource, /allowRunningInsecureContent:\s*false/);
  assert.match(mainSource, /webviewTag:\s*false/);
});

test('Electron blocks renderer navigation outside the local app origin', () => {
  assert.match(mainSource, /setWindowOpenHandler/);
  assert.match(mainSource, /will-navigate/);
  assert.match(mainSource, /will-redirect/);
  assert.match(mainSource, /isTrustedAppUrl/);
});

test('Electron backend binds only to IPv4 loopback', () => {
  assert.match(mainSource, /BIND_ADDRESS:\s*'127\.0\.0\.1'/);
});

test('macOS release builds require the hardened runtime and signing discovery', () => {
  assert.equal(packageConfig.build.mac.hardenedRuntime, true);
  assert.equal(packageConfig.build.mac.identity, undefined);
});

test('macOS entitlements do not disable library or environment validation', () => {
  assert.doesNotMatch(
    macEntitlements,
    /com\.apple\.security\.cs\.disable-library-validation/,
  );
  assert.doesNotMatch(
    macEntitlements,
    /com\.apple\.security\.cs\.allow-dyld-environment-variables/,
  );
});
