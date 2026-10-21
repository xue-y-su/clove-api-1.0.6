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

const signIn = readFileSync(
  resolve(srcRoot, 'features/auth/sign-in/index.tsx'),
  'utf8'
)
const signUpForm = readFileSync(
  resolve(srcRoot, 'features/auth/sign-up/components/sign-up-form.tsx'),
  'utf8'
)
const authAvailability = readFileSync(
  resolve(srcRoot, 'features/auth/lib/registration-availability.ts'),
  'utf8'
)

assert.match(
  authAvailability,
  /hasPasswordRegistration/,
  '注册能力判断必须区分总注册开关和密码注册开关'
)
assert.match(
  authAvailability,
  /hasAnyRegistrationMethod/,
  '登录页注册链接必须只在至少一种注册方式可用时展示'
)
assert.match(
  signIn,
  /hasAnyRegistrationMethod/,
  '登录页不能只看 register_enabled，否则密码注册关闭时仍会误导用户进入注册页'
)
assert.match(
  signUpForm,
  /passwordRegistrationEnabled/,
  '注册页必须根据 password_register_enabled 隐藏或禁用密码注册表单'
)
assert.match(
  signUpForm,
  /Password registration is currently disabled/,
  '密码注册关闭时必须显示明确提示'
)

process.stdout.write('auth registration availability checks passed\n')
