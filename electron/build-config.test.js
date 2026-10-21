const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const assert = require('node:assert/strict');

const repositoryRoot = path.join(__dirname, '..');
const packageConfig = JSON.parse(
  fs.readFileSync(path.join(__dirname, 'package.json'), 'utf8'),
);
const workflow = fs.readFileSync(
  path.join(repositoryRoot, '.github', 'workflows', 'electron-build.yml'),
  'utf8',
);
const buildScript = fs.readFileSync(path.join(__dirname, 'build.sh'), 'utf8');

test('Electron tooling declares and uses a supported Node runtime', () => {
  assert.equal(packageConfig.engines.node, '>=22.12.0');
  assert.match(workflow, /node-version:\s*['"]22['"]/);
});

test('macOS packaging relies on the frontend embedded in the Go binary', () => {
  const standaloneWebResources = packageConfig.build.mac.extraResources.filter(
    ({ from }) => from.startsWith('../web/'),
  );

  assert.deepEqual(standaloneWebResources, []);
});

test('portable build installs dependencies from lockfiles', () => {
  assert.match(buildScript, /bun install --frozen-lockfile/);
  assert.match(buildScript, /npm ci/);
  assert.doesNotMatch(buildScript, /npm install(?:\s|$)/);
});

test('portable build compiles both embedded frontend themes', () => {
  assert.match(buildScript, /web\/default/);
  assert.match(buildScript, /web\/classic/);
});

test('portable build injects the normalized release version', () => {
  assert.match(buildScript, /tr -d ['"]\\r\\n['"]/);
  assert.match(buildScript, /common\.Version=\$VERSION/);
});

test('portable build gives the desktop package the release version', () => {
  assert.match(buildScript, /ELECTRON_VERSION="\$\{VERSION#v\}"/);
  assert.match(
    buildScript,
    /--config\.extraMetadata\.version="\$ELECTRON_VERSION"/,
  );
});
