/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const checkFile = fileURLToPath(import.meta.url);
const repoRoot = path.resolve(path.dirname(checkFile), '../../../..');

const requiredChineseKeys = [
  'Compliance Statement',
  'Invitation Registration',
  'Source Attribution',
  'Invitation source',
  'Invitation code',
  'Invitation code (optional)',
  'Invitation code is required',
  'Enter invitation code',
  'Optional invitation code',
  'Registration requires a valid invitation code.',
  'Source attribution confirmations',
  'Source Attribution Confirmations',
  'Attribution option',
  'Source description',
  'Enter your source',
  'No attribution options are configured. Please contact an administrator.',
];

const failures = [];

function readText(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), 'utf8');
}

function assert(condition, message) {
  if (!condition) failures.push(message);
}

function indexOfRequired(text, needle, file) {
  const index = text.indexOf(needle);
  assert(index !== -1, `${file} is missing ${needle}`);
  return index;
}

function sliceBetween(text, startNeedle, endNeedle, file) {
  const start = indexOfRequired(text, startNeedle, file);
  const end = indexOfRequired(text.slice(start), endNeedle, file);
  return text.slice(start, start + end);
}

function assertLocaleKeys(relativePath) {
  const locale = JSON.parse(readText(relativePath));
  const translations = locale.translation || locale;
  for (const key of requiredChineseKeys) {
    const value = translations[key];
    assert(
      typeof value === 'string' && value.trim() && value !== key,
      `${relativePath} must translate "${key}" in Chinese locales`,
    );
  }
}

const classicApp = readText('web/classic/src/App.jsx');
assert(
  /path='\/sign-up'[\s\S]*?<RegisterForm \/>/.test(classicApp),
  'classic App.jsx must route /sign-up to RegisterForm',
);

const classicRegister = readText(
  'web/classic/src/components/auth/RegisterForm.jsx',
);
assert(
  classicRegister.includes('function getAffFromCurrentUrl()') &&
    classicRegister.includes('new URL(window.location.href)') &&
    classicRegister.includes(".searchParams.get('aff')"),
  'classic RegisterForm must read aff from the current URL, including BrowserRouter basename URLs',
);
assert(
  classicRegister.includes("localStorage.setItem('aff', urlAffCode)") ||
    classicRegister.includes("localStorage.setItem('aff', nextAffCode)") ||
    classicRegister.includes("localStorage.setItem('aff', affCode)"),
  'classic RegisterForm must persist URL aff to localStorage',
);
assert(
  classicRegister.includes('syncAffFormField(nextAffCode)') &&
    classicRegister.includes("setValue('aff_code'"),
  'classic RegisterForm must sync URL aff into the Semi Form field state',
);
assert(
  classicRegister.includes("localStorage.removeItem('aff')"),
  'classic RegisterForm must clear stale localStorage aff when the invitation field is cleared',
);
assert(
  classicRegister.includes('function getRegistrationAffCode()') &&
    classicRegister.includes('function validateAffForRegistration()') &&
    classicRegister.includes('buildRegistrationOAuthOptions'),
  'classic RegisterForm must share the same aff resolver for normal and OAuth registration',
);

const classicTelegramRegister = sliceBetween(
  classicRegister,
  'const onTelegramLoginClicked = async (response) => {',
  'const renderAffInput = () =>',
  'classic RegisterForm Telegram registration handler',
);
assert(
  classicTelegramRegister.includes('validateAffForRegistration()') &&
    classicTelegramRegister.includes('params.aff = affCode'),
  'classic Telegram registration must validate and pass the shared invitation code',
);

const classicAuth = readText('web/classic/src/helpers/auth.jsx');
const classicComplianceIndex = indexOfRequired(
  classicAuth,
  '<ComplianceGate>',
  'classic auth gate',
);
const classicSourceIndex = indexOfRequired(
  classicAuth,
  '<SourceAttributionGate>',
  'classic auth gate',
);
assert(
  classicComplianceIndex < classicSourceIndex,
  'classic login gate order must be compliance before source attribution',
);

const classicSourceGate = readText(
  'web/classic/src/components/source-attribution/SourceAttributionGate.jsx',
);
const classicComplianceGate = readText(
  'web/classic/src/components/compliance/ComplianceGate.jsx',
);
assert(
  classicSourceGate.includes('complianceReady') &&
    classicSourceGate.includes('!statement?.enabled || statement?.confirmed') &&
    classicSourceGate.includes(
      'status?.enabled && !status.confirmed && complianceReady',
    ),
  'classic source attribution gate must wait for compliance confirmation',
);
assert(
  classicComplianceGate.includes(
    "window.dispatchEvent(new Event('compliance-confirmed'))",
  ) &&
    classicSourceGate.includes("'compliance-confirmed'") &&
    classicSourceGate.includes('window.addEventListener') &&
    classicSourceGate.includes('window.removeEventListener'),
  'classic source attribution gate must refresh immediately after compliance confirmation',
);

const defaultLayout = readText(
  'web/default/src/components/layout/components/authenticated-layout.tsx',
);
const defaultComplianceIndex = indexOfRequired(
  defaultLayout,
  '<ComplianceGate />',
  'default authenticated layout',
);
const defaultSourceIndex = indexOfRequired(
  defaultLayout,
  '<SourceAttributionGate />',
  'default authenticated layout',
);
const defaultHeaderIndex = indexOfRequired(
  defaultLayout,
  '<AppHeader />',
  'default authenticated layout',
);
assert(
  defaultComplianceIndex < defaultSourceIndex &&
    defaultSourceIndex < defaultHeaderIndex,
  'default login surface order must be compliance, source attribution, then announcements header',
);

const defaultSourceGate = readText(
  'web/default/src/features/source-attribution/source-attribution-gate.tsx',
);
assert(
  defaultSourceGate.includes('complianceReady') &&
    defaultSourceGate.includes(
      '!complianceStatement.enabled || complianceStatement.confirmed',
    ) &&
    defaultSourceGate.includes(
      'status?.enabled && !status.confirmed && complianceReady',
    ),
  'default source attribution gate must wait for compliance confirmation',
);

for (const localePath of [
  'web/classic/src/i18n/locales/zh.json',
  'web/classic/src/i18n/locales/zh-CN.json',
  'web/classic/src/i18n/locales/zh-TW.json',
  'web/default/src/i18n/locales/zh.json',
]) {
  assertLocaleKeys(localePath);
}

if (failures.length > 0) {
  console.error(failures.map((failure) => `- ${failure}`).join('\n'));
  process.exit(1);
}

console.log('registration/i18n/order checks passed');
