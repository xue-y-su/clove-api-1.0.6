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
import jsep from 'jsep'

import { BILLING_CACHE_VAR_MAP } from './billing-expr'

export const CACHE_MODE_TIMED = 'timed'
export const CACHE_MODE_GENERIC = 'generic'
export type CacheMode = typeof CACHE_MODE_TIMED | typeof CACHE_MODE_GENERIC

export type TierConditionInput = {
  var: 'p' | 'c' | 'len'
  op: '<' | '<=' | '>' | '>='
  value: number | string
}

export type VisualTier = {
  label: string
  conditions: TierConditionInput[]
  input_unit_cost: number
  output_unit_cost: number
  cache_mode: CacheMode
  cache_read_unit_cost?: number
  cache_create_unit_cost?: number
  cache_create_1h_unit_cost?: number
  image_unit_cost?: number
  image_output_unit_cost?: number
  audio_input_unit_cost?: number
  audio_output_unit_cost?: number
  [field: string]: unknown
}

export type VisualConfig = {
  tiers: VisualTier[]
}

export function getTierCacheMode(
  tier: Partial<VisualTier> | null | undefined
): CacheMode {
  if (tier?.cache_mode === CACHE_MODE_TIMED) return CACHE_MODE_TIMED
  if (tier?.cache_mode === CACHE_MODE_GENERIC) return CACHE_MODE_GENERIC
  return Number(tier?.cache_create_1h_unit_cost) > 0
    ? CACHE_MODE_TIMED
    : CACHE_MODE_GENERIC
}

export function normalizeVisualTier(
  tier: Partial<VisualTier> = {}
): VisualTier {
  return {
    label: tier.label ?? '',
    input_unit_cost: Number(tier.input_unit_cost) || 0,
    output_unit_cost: Number(tier.output_unit_cost) || 0,
    cache_mode: getTierCacheMode(tier),
    conditions: Array.isArray(tier.conditions) ? tier.conditions : [],
    ...tier,
    cache_read_unit_cost: Number(tier.cache_read_unit_cost) || 0,
    cache_create_unit_cost: Number(tier.cache_create_unit_cost) || 0,
    cache_create_1h_unit_cost: Number(tier.cache_create_1h_unit_cost) || 0,
    image_unit_cost: Number(tier.image_unit_cost) || 0,
    image_output_unit_cost: Number(tier.image_output_unit_cost) || 0,
    audio_input_unit_cost: Number(tier.audio_input_unit_cost) || 0,
    audio_output_unit_cost: Number(tier.audio_output_unit_cost) || 0,
  }
}

export function createDefaultVisualConfig(): VisualConfig {
  return {
    tiers: [
      normalizeVisualTier({
        conditions: [],
        input_unit_cost: 0,
        output_unit_cost: 0,
        label: 'base',
        cache_mode: CACHE_MODE_GENERIC,
      }),
    ],
  }
}

export function normalizeVisualConfig(
  config: VisualConfig | null | undefined
): VisualConfig {
  if (!config || !Array.isArray(config.tiers) || config.tiers.length === 0) {
    return createDefaultVisualConfig()
  }
  return {
    ...config,
    tiers: config.tiers.map((tier) => normalizeVisualTier(tier)),
  }
}

function buildConditionStr(conditions: TierConditionInput[]): string {
  if (!conditions || conditions.length === 0) return ''
  return conditions
    .filter((c) => c.var && c.op && c.value != null && c.value !== '')
    .map((c) => `${c.var} ${c.op} ${c.value}`)
    .join(' && ')
}

function buildTierBodyExpr(tier: VisualTier): string {
  const parts: string[] = []
  const ic = Number(tier.input_unit_cost) || 0
  const oc = Number(tier.output_unit_cost) || 0
  parts.push(`p * ${ic}`)
  parts.push(`c * ${oc}`)
  for (const cv of BILLING_CACHE_VAR_MAP) {
    const v = Number((tier as Record<string, unknown>)[cv.field]) || 0
    if (v !== 0) parts.push(`${cv.exprVar} * ${v}`)
  }
  return parts.join(' + ')
}

export function generateExprFromVisualConfig(
  config: VisualConfig | null | undefined
): string {
  if (!config || !config.tiers || config.tiers.length === 0) {
    return 'p * 0 + c * 0'
  }
  const tiers = config.tiers

  if (tiers.length === 1) {
    const tier = tiers[0]
    const label = tier.label || 'default'
    const body = `tier("${label}", ${buildTierBodyExpr(tier)})`
    const cond = buildConditionStr(tier.conditions)
    if (cond) {
      return `${cond} ? ${body} : p * 0 + c * 0`
    }
    return body
  }

  const parts: string[] = []
  for (let i = 0; i < tiers.length; i++) {
    const tier = tiers[i]
    const label = tier.label || `tier_${i + 1}`
    const body = `tier("${label}", ${buildTierBodyExpr(tier)})`
    const cond = buildConditionStr(tier.conditions)

    if (i < tiers.length - 1 && cond) {
      parts.push(`${cond} ? ${body}`)
    } else {
      parts.push(body)
    }
  }
  return parts.join(' : ')
}

export function tryParseVisualConfig(
  exprStr: string | null | undefined
): VisualConfig | null {
  if (!exprStr) return null
  try {
    let body = exprStr
    const versionMatch = body.match(/^v\d+:([\s\S]*)$/)
    if (versionMatch) body = versionMatch[1]
    const cacheVarNames = BILLING_CACHE_VAR_MAP.map((cv) => cv.exprVar)
    const optCacheStr = cacheVarNames
      .map((v) => `(?:\\s*\\+\\s*${v}\\s*\\*\\s*([\\d.eE+-]+))?`)
      .join('')

    const bodyPat = `p\\s*\\*\\s*([\\d.eE+-]+)\\s*\\+\\s*c\\s*\\*\\s*([\\d.eE+-]+)${optCacheStr}`

    const singleRe = new RegExp(`^tier\\("([^"]*)",\\s*${bodyPat}\\)$`)
    const simple = body.match(singleRe)
    if (simple) {
      const tier: Record<string, unknown> = {
        conditions: [],
        input_unit_cost: Number(simple[2]),
        output_unit_cost: Number(simple[3]),
        label: simple[1],
      }
      BILLING_CACHE_VAR_MAP.forEach((cv, i) => {
        const val = simple[4 + i]
        if (val != null) tier[cv.field] = Number(val)
      })
      return normalizeVisualConfig({
        tiers: [normalizeVisualTier(tier as Partial<VisualTier>)],
      })
    }

    const condGroup =
      `((?:(?:p|c|len)\\s*(?:<|<=|>|>=)\\s*[\\d.eE+]+)` +
      `(?:\\s*&&\\s*(?:p|c|len)\\s*(?:<|<=|>|>=)\\s*[\\d.eE+]+)*)`
    const tierRe = new RegExp(
      `(?:${condGroup}\\s*\\?\\s*)?tier\\("([^"]*)",\\s*${bodyPat}\\)`,
      'g'
    )
    const tiers: VisualTier[] = []
    let match: RegExpExecArray | null
    while ((match = tierRe.exec(body)) !== null) {
      const condStr = match[1] || ''
      const conditions: TierConditionInput[] = []
      if (condStr) {
        for (const cp of condStr.split(/\s*&&\s*/)) {
          const cm = cp.trim().match(/^(p|c|len)\s*(<|<=|>|>=)\s*([\d.eE+]+)$/)
          if (cm) {
            conditions.push({
              var: cm[1] as TierConditionInput['var'],
              op: cm[2] as TierConditionInput['op'],
              value: Number(cm[3]),
            })
          }
        }
      }
      const tier: Record<string, unknown> = {
        conditions,
        input_unit_cost: Number(match[3]),
        output_unit_cost: Number(match[4]),
        label: match[2],
      }
      const m = match
      BILLING_CACHE_VAR_MAP.forEach((cv, i) => {
        const val = m[5 + i]
        if (val != null) tier[cv.field] = Number(val)
      })
      tiers.push(normalizeVisualTier(tier as Partial<VisualTier>))
    }
    if (tiers.length === 0) return null

    const cfg = normalizeVisualConfig({ tiers })
    const regenerated = generateExprFromVisualConfig(cfg)
    if (regenerated.replaceAll(/\s+/g, '') !== body.replaceAll(/\s+/g, '')) {
      return null
    }
    return cfg
  } catch {
    return null
  }
}

// ---------------------------------------------------------------------------
// Local cost evaluator (for the estimator preview)
// ---------------------------------------------------------------------------

const ESTIMATOR_VARS = [
  { var: 'cr', stateKey: 'cacheReadTokens' },
  { var: 'cc', stateKey: 'cacheCreateTokens' },
  { var: 'cc1h', stateKey: 'cacheCreate1hTokens' },
  { var: 'img', stateKey: 'imageTokens' },
  { var: 'img_o', stateKey: 'imageOutputTokens' },
  { var: 'ai', stateKey: 'audioInputTokens' },
  { var: 'ao', stateKey: 'audioOutputTokens' },
] as const

export type ExtraTokenValues = Record<
  (typeof ESTIMATOR_VARS)[number]['stateKey'],
  number
>

export type EvalResult = {
  cost: number
  matchedTier: string
  error: string | null
}

type BillingExpressionValue = number | string | boolean | null

type BillingExpressionContext = {
  variables: Record<string, number>
  matchedTier: string
  visitedNodes: number
}

const MAX_BILLING_EXPR_BYTES = 20_000
const MAX_BILLING_EXPR_DEPTH = 64
const MAX_BILLING_EXPR_NODES = 1_000

function assertExpressionComplexity(exprStr: string): void {
  if (exprStr.length > MAX_BILLING_EXPR_BYTES) {
    throw new Error('Billing expression is too long')
  }

  let depth = 0
  let quote = ''
  let escaped = false
  for (const char of exprStr) {
    if (quote) {
      if (escaped) {
        escaped = false
      } else if (char === '\\') {
        escaped = true
      } else if (char === quote) {
        quote = ''
      }
      continue
    }
    if (char === '"' || char === "'") {
      quote = char
      continue
    }
    if (char === '(' || char === '[') {
      depth += 1
      if (depth > MAX_BILLING_EXPR_DEPTH) {
        throw new Error('Billing expression is nested too deeply')
      }
    } else if (char === ')' || char === ']') {
      depth -= 1
    }
  }
}

function requireNumber(
  value: BillingExpressionValue,
  operation: string
): number {
  if (typeof value !== 'number') {
    throw new Error(`${operation} requires numeric operands`)
  }
  return value
}

function requireBoolean(
  value: BillingExpressionValue,
  operation: string
): boolean {
  if (typeof value !== 'boolean') {
    throw new Error(`${operation} requires boolean operands`)
  }
  return value
}

function compareValues(
  left: BillingExpressionValue,
  right: BillingExpressionValue,
  operation: '<' | '<=' | '>' | '>='
): boolean {
  if (typeof left === 'number' && typeof right === 'number') {
    if (operation === '<') return left < right
    if (operation === '<=') return left <= right
    if (operation === '>') return left > right
    return left >= right
  }
  if (typeof left === 'string' && typeof right === 'string') {
    if (operation === '<') return left < right
    if (operation === '<=') return left <= right
    if (operation === '>') return left > right
    return left >= right
  }
  throw new Error(`${operation} requires operands of the same comparable type`)
}

function evaluateBinaryExpression(
  node: jsep.BinaryExpression,
  context: BillingExpressionContext,
  depth: number
): BillingExpressionValue {
  if (node.operator === '&&') {
    const left = requireBoolean(
      evaluateExpressionNode(node.left, context, depth + 1),
      node.operator
    )
    return left
      ? requireBoolean(
          evaluateExpressionNode(node.right, context, depth + 1),
          node.operator
        )
      : false
  }
  if (node.operator === '||') {
    const left = requireBoolean(
      evaluateExpressionNode(node.left, context, depth + 1),
      node.operator
    )
    return left
      ? true
      : requireBoolean(
          evaluateExpressionNode(node.right, context, depth + 1),
          node.operator
        )
  }

  const left = evaluateExpressionNode(node.left, context, depth + 1)
  const right = evaluateExpressionNode(node.right, context, depth + 1)
  switch (node.operator) {
    case '+':
      return (
        requireNumber(left, node.operator) + requireNumber(right, node.operator)
      )
    case '-':
      return (
        requireNumber(left, node.operator) - requireNumber(right, node.operator)
      )
    case '*':
      return (
        requireNumber(left, node.operator) * requireNumber(right, node.operator)
      )
    case '/':
      return (
        requireNumber(left, node.operator) / requireNumber(right, node.operator)
      )
    case '%':
      return (
        requireNumber(left, node.operator) % requireNumber(right, node.operator)
      )
    case '**':
      return (
        requireNumber(left, node.operator) **
        requireNumber(right, node.operator)
      )
    case '<':
    case '<=':
    case '>':
    case '>=':
      return compareValues(left, right, node.operator)
    case '==':
    case '===':
      return left === right
    case '!=':
    case '!==':
      return left !== right
    default:
      throw new Error(`Unsupported operator: ${node.operator}`)
  }
}

function evaluateCallExpression(
  node: jsep.CallExpression,
  context: BillingExpressionContext,
  depth: number
): BillingExpressionValue {
  if (node.callee.type !== 'Identifier') {
    throw new Error('Only named billing functions may be called')
  }

  const functionName = (node.callee as jsep.Identifier).name
  const args = node.arguments.map((argument) =>
    evaluateExpressionNode(argument, context, depth + 1)
  )
  if (functionName === 'tier') {
    if (args.length !== 2 || typeof args[0] !== 'string') {
      throw new Error('tier requires a name and numeric value')
    }
    const value = requireNumber(args[1], 'tier')
    context.matchedTier = args[0]
    return value
  }

  const numericArgs = args.map((argument) =>
    requireNumber(argument, functionName)
  )
  switch (functionName) {
    case 'max':
      if (numericArgs.length !== 2) {
        throw new Error('max requires two arguments')
      }
      return Math.max(numericArgs[0], numericArgs[1])
    case 'min':
      if (numericArgs.length !== 2) {
        throw new Error('min requires two arguments')
      }
      return Math.min(numericArgs[0], numericArgs[1])
    case 'abs':
      if (numericArgs.length !== 1) throw new Error('abs requires one argument')
      return Math.abs(numericArgs[0])
    case 'ceil':
      if (numericArgs.length !== 1) {
        throw new Error('ceil requires one argument')
      }
      return Math.ceil(numericArgs[0])
    case 'floor':
      if (numericArgs.length !== 1) {
        throw new Error('floor requires one argument')
      }
      return Math.floor(numericArgs[0])
    default:
      throw new Error(`Unsupported billing function: ${functionName}`)
  }
}

function evaluateExpressionNode(
  node: jsep.Expression,
  context: BillingExpressionContext,
  depth = 0
): BillingExpressionValue {
  context.visitedNodes += 1
  if (
    depth > MAX_BILLING_EXPR_DEPTH ||
    context.visitedNodes > MAX_BILLING_EXPR_NODES
  ) {
    throw new Error('Billing expression is too complex')
  }

  switch (node.type) {
    case 'Literal':
      return (node as jsep.Literal).value as BillingExpressionValue
    case 'Identifier': {
      const name = (node as jsep.Identifier).name
      if (!Object.hasOwn(context.variables, name)) {
        throw new Error(`Unknown billing variable: ${name}`)
      }
      return context.variables[name]
    }
    case 'UnaryExpression': {
      const unary = node as jsep.UnaryExpression
      const value = evaluateExpressionNode(unary.argument, context, depth + 1)
      if (unary.operator === '!') return !requireBoolean(value, unary.operator)
      if (unary.operator === '+') return requireNumber(value, unary.operator)
      if (unary.operator === '-') return -requireNumber(value, unary.operator)
      throw new Error(`Unsupported unary operator: ${unary.operator}`)
    }
    case 'BinaryExpression':
      return evaluateBinaryExpression(
        node as jsep.BinaryExpression,
        context,
        depth
      )
    case 'ConditionalExpression': {
      const conditional = node as jsep.ConditionalExpression
      const condition = requireBoolean(
        evaluateExpressionNode(conditional.test, context, depth + 1),
        'conditional expression'
      )
      return evaluateExpressionNode(
        condition ? conditional.consequent : conditional.alternate,
        context,
        depth + 1
      )
    }
    case 'CallExpression':
      return evaluateCallExpression(node as jsep.CallExpression, context, depth)
    default:
      throw new Error(`Unsupported billing expression node: ${node.type}`)
  }
}

export function evalExprLocally(
  exprStr: string,
  promptTokens: number,
  completionTokens: number,
  extraTokenValues: ExtraTokenValues
): EvalResult {
  try {
    if (!exprStr || !exprStr.trim()) {
      return { cost: 0, matchedTier: '', error: null }
    }
    const expression = exprStr.startsWith('v1:') ? exprStr.slice(3) : exprStr
    assertExpressionComplexity(expression)
    const cacheReadTokens = extraTokenValues.cacheReadTokens || 0
    const cacheCreateTokens = extraTokenValues.cacheCreateTokens || 0
    const cacheCreate1hTokens = extraTokenValues.cacheCreate1hTokens || 0
    const len =
      promptTokens + cacheReadTokens + cacheCreateTokens + cacheCreate1hTokens
    const context: BillingExpressionContext = {
      variables: {
        p: promptTokens,
        c: completionTokens,
        len,
      },
      matchedTier: '',
      visitedNodes: 0,
    }
    for (const field of ESTIMATOR_VARS) {
      context.variables[field.var] = extraTokenValues[field.stateKey] || 0
    }
    const value = evaluateExpressionNode(jsep(expression), context)
    const cost = requireNumber(value, 'Billing expression result')
    if (!Number.isFinite(cost)) {
      throw new Error('Billing expression result must be finite')
    }
    return { cost, matchedTier: context.matchedTier, error: null }
  } catch (e) {
    const message = e instanceof Error ? e.message : String(e)
    return { cost: 0, matchedTier: '', error: message }
  }
}

export function exprUsesExtraVars(exprStr: string): boolean {
  if (!exprStr) return false
  const varNames = ESTIMATOR_VARS.map((f) => f.var).join('|')
  return new RegExp(`\\b(${varNames})\\b`).test(exprStr)
}

export const ESTIMATOR_EXTRA_FIELDS = ESTIMATOR_VARS
