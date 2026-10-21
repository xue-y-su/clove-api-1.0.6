/*
Copyright (C) 2023-2026 QuantumNous

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
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const srcRoot = resolve(__dirname, '..')

function read(relativePath) {
  return readFileSync(resolve(srcRoot, relativePath), 'utf8')
}

const oauthCallback = read('routes/oauth/$provider.tsx')
const accountBindings = read(
  'features/profile/components/tabs/account-bindings-tab.tsx'
)
const profileTypes = read('features/profile/types.ts')

assert.match(
  oauthCallback,
  /data\s*\?\.\s*action|callbackAction|action\s*===\s*['"]bind['"]/,
  'default OAuth 回调必须根据响应 data.action 判断绑定结果'
)
assert.doesNotMatch(
  oauthCallback,
  /message\s*===\s*['"]bind['"]/,
  'default OAuth 回调不能依赖可本地化的 message 判断绑定结果'
)
assert.match(
  accountBindings,
  /buildCustomOAuthUrl|authorization_endpoint/,
  'default 自定义 OAuth 绑定入口必须使用 provider 授权端点构造跳转地址'
)
assert.doesNotMatch(
  accountBindings,
  /\/api\/oauth\/\$\{slug\}\?redirect=/,
  'default 自定义 OAuth 绑定入口不能直接跳后端 callback 路由绕过 state 初始化'
)
assert.match(
  profileTypes,
  /authorization_endpoint\??:\s*string/,
  '个人资料自定义 OAuth provider 类型必须包含授权端点'
)
assert.match(
  profileTypes,
  /client_id\??:\s*string/,
  '个人资料自定义 OAuth provider 类型必须包含 client_id'
)

console.log('default oauth binding checks passed')
