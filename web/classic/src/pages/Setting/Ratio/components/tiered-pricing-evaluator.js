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
import jsep from 'jsep';

const MAX_EXPRESSION_BYTES = 20_000;
const MAX_EXPRESSION_DEPTH = 64;
const MAX_EXPRESSION_NODES = 1_000;
const CACHE_LENGTH_STATE_KEYS = new Set([
  'cacheReadTokens',
  'cacheCreateTokens',
  'cacheCreate1hTokens',
]);

function assertExpressionComplexity(expression) {
  if (expression.length > MAX_EXPRESSION_BYTES) {
    throw new Error('Billing expression is too long');
  }

  let depth = 0;
  let quote = '';
  let escaped = false;
  for (const character of expression) {
    if (quote) {
      if (escaped) {
        escaped = false;
      } else if (character === '\\') {
        escaped = true;
      } else if (character === quote) {
        quote = '';
      }
      continue;
    }
    if (character === '"' || character === "'") {
      quote = character;
      continue;
    }
    if (character === '(' || character === '[') {
      depth += 1;
      if (depth > MAX_EXPRESSION_DEPTH) {
        throw new Error('Billing expression is nested too deeply');
      }
    } else if (character === ')' || character === ']') {
      depth -= 1;
    }
  }
}

function requireNumber(value, operation) {
  if (typeof value !== 'number') {
    throw new Error(`${operation} requires numeric operands`);
  }
  return value;
}

function requireBoolean(value, operation) {
  if (typeof value !== 'boolean') {
    throw new Error(`${operation} requires boolean operands`);
  }
  return value;
}

function compareValues(left, right, operation) {
  if (typeof left === 'number' && typeof right === 'number') {
    if (operation === '<') return left < right;
    if (operation === '<=') return left <= right;
    if (operation === '>') return left > right;
    return left >= right;
  }
  if (typeof left === 'string' && typeof right === 'string') {
    if (operation === '<') return left < right;
    if (operation === '<=') return left <= right;
    if (operation === '>') return left > right;
    return left >= right;
  }
  throw new Error(`${operation} requires operands of the same comparable type`);
}

function evaluateBinaryExpression(node, context, depth) {
  if (node.operator === '&&') {
    const left = requireBoolean(
      evaluateExpressionNode(node.left, context, depth + 1),
      node.operator,
    );
    return left
      ? requireBoolean(
          evaluateExpressionNode(node.right, context, depth + 1),
          node.operator,
        )
      : false;
  }
  if (node.operator === '||') {
    const left = requireBoolean(
      evaluateExpressionNode(node.left, context, depth + 1),
      node.operator,
    );
    return left
      ? true
      : requireBoolean(
          evaluateExpressionNode(node.right, context, depth + 1),
          node.operator,
        );
  }

  const left = evaluateExpressionNode(node.left, context, depth + 1);
  const right = evaluateExpressionNode(node.right, context, depth + 1);
  switch (node.operator) {
    case '+':
      return (
        requireNumber(left, node.operator) + requireNumber(right, node.operator)
      );
    case '-':
      return (
        requireNumber(left, node.operator) - requireNumber(right, node.operator)
      );
    case '*':
      return (
        requireNumber(left, node.operator) * requireNumber(right, node.operator)
      );
    case '/':
      return (
        requireNumber(left, node.operator) / requireNumber(right, node.operator)
      );
    case '%':
      return (
        requireNumber(left, node.operator) % requireNumber(right, node.operator)
      );
    case '**':
      return (
        requireNumber(left, node.operator) **
        requireNumber(right, node.operator)
      );
    case '<':
    case '<=':
    case '>':
    case '>=':
      return compareValues(left, right, node.operator);
    case '==':
    case '===':
      return left === right;
    case '!=':
    case '!==':
      return left !== right;
    default:
      throw new Error(`Unsupported operator: ${node.operator}`);
  }
}

function evaluateCallExpression(node, context, depth) {
  if (node.callee.type !== 'Identifier') {
    throw new Error(`Unsupported billing expression node: ${node.callee.type}`);
  }

  const functionName = node.callee.name;
  const args = node.arguments.map((argument) =>
    evaluateExpressionNode(argument, context, depth + 1),
  );
  if (functionName === 'tier') {
    if (args.length !== 2 || typeof args[0] !== 'string') {
      throw new Error('tier requires a name and numeric value');
    }
    const value = requireNumber(args[1], 'tier');
    context.matchedTier = args[0];
    return value;
  }

  const numericArgs = args.map((argument) =>
    requireNumber(argument, functionName),
  );
  switch (functionName) {
    case 'max':
      if (numericArgs.length !== 2)
        throw new Error('max requires two arguments');
      return Math.max(numericArgs[0], numericArgs[1]);
    case 'min':
      if (numericArgs.length !== 2)
        throw new Error('min requires two arguments');
      return Math.min(numericArgs[0], numericArgs[1]);
    case 'abs':
      if (numericArgs.length !== 1)
        throw new Error('abs requires one argument');
      return Math.abs(numericArgs[0]);
    case 'ceil':
      if (numericArgs.length !== 1)
        throw new Error('ceil requires one argument');
      return Math.ceil(numericArgs[0]);
    case 'floor':
      if (numericArgs.length !== 1)
        throw new Error('floor requires one argument');
      return Math.floor(numericArgs[0]);
    default:
      throw new Error(`Unsupported billing function: ${functionName}`);
  }
}

function evaluateExpressionNode(node, context, depth = 0) {
  context.visitedNodes += 1;
  if (
    depth > MAX_EXPRESSION_DEPTH ||
    context.visitedNodes > MAX_EXPRESSION_NODES
  ) {
    throw new Error('Billing expression is too complex');
  }

  switch (node.type) {
    case 'Literal':
      return node.value;
    case 'Identifier': {
      if (!Object.hasOwn(context.variables, node.name)) {
        throw new Error(`Unknown billing variable: ${node.name}`);
      }
      return context.variables[node.name];
    }
    case 'UnaryExpression': {
      const value = evaluateExpressionNode(node.argument, context, depth + 1);
      if (node.operator === '!') return !requireBoolean(value, node.operator);
      if (node.operator === '+') return requireNumber(value, node.operator);
      if (node.operator === '-') return -requireNumber(value, node.operator);
      throw new Error(`Unsupported unary operator: ${node.operator}`);
    }
    case 'BinaryExpression':
      return evaluateBinaryExpression(node, context, depth);
    case 'ConditionalExpression': {
      const condition = requireBoolean(
        evaluateExpressionNode(node.test, context, depth + 1),
        'conditional expression',
      );
      return evaluateExpressionNode(
        condition ? node.consequent : node.alternate,
        context,
        depth + 1,
      );
    }
    case 'CallExpression':
      return evaluateCallExpression(node, context, depth);
    default:
      throw new Error(`Unsupported billing expression node: ${node.type}`);
  }
}

export function evaluateTieredPricingExpression(
  exprStr,
  promptTokens,
  completionTokens,
  extraTokenValues = {},
  extraFields = [],
) {
  try {
    if (!exprStr || !exprStr.trim()) {
      return { cost: 0, matchedTier: '', error: null };
    }
    const expression = exprStr.startsWith('v1:') ? exprStr.slice(3) : exprStr;
    assertExpressionComplexity(expression);

    const variables = {
      p: promptTokens,
      c: completionTokens,
      len: promptTokens,
    };
    for (const field of extraFields) {
      const value = extraTokenValues[field.stateKey] || 0;
      variables[field.var] = value;
      if (CACHE_LENGTH_STATE_KEYS.has(field.stateKey)) {
        variables.len += value;
      }
    }

    const context = { variables, matchedTier: '', visitedNodes: 0 };
    const cost = requireNumber(
      evaluateExpressionNode(jsep(expression), context),
      'Billing expression result',
    );
    if (!Number.isFinite(cost)) {
      throw new Error('Billing expression result must be finite');
    }
    return { cost, matchedTier: context.matchedTier, error: null };
  } catch (error) {
    return {
      cost: 0,
      matchedTier: '',
      error: error instanceof Error ? error.message : String(error),
    };
  }
}
