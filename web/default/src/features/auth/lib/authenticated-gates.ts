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
export type AuthenticatedGateResolution =
  | 'loading'
  | 'required'
  | 'satisfied'
  | 'error'
export type AuthenticatedGateFailure = 'compliance' | 'sourceAttribution'

type AuthenticatedGateState = {
  signedIn: boolean
  compliance: AuthenticatedGateResolution
  sourceAttribution: AuthenticatedGateResolution
}

export function getAuthenticatedGateAccess(state: AuthenticatedGateState) {
  const compliance = state.signedIn
  const failed: AuthenticatedGateFailure | null =
    state.signedIn && state.compliance === 'error'
      ? 'compliance'
      : state.signedIn &&
          state.compliance === 'satisfied' &&
          state.sourceAttribution === 'error'
        ? 'sourceAttribution'
        : null
  const sourceAttribution =
    compliance &&
    state.compliance === 'satisfied' &&
    state.sourceAttribution !== 'error'
  const announcement =
    sourceAttribution && state.sourceAttribution === 'satisfied'
  const content = announcement

  return { compliance, sourceAttribution, announcement, content, failed }
}

export function gateQueryKey(gate: AuthenticatedGateFailure) {
  return gate === 'compliance'
    ? (['compliance-statement'] as const)
    : (['source-attribution-status'] as const)
}
