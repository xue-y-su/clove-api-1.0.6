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
export const SNOW_HEADER_SCROLL_THRESHOLD = 20
// Snow's bundled WebGL controller uses a virtual scroll multiplier of 0.00075
// and three looping scenes with heights 2.35, 3, and 5.5. The iframe does
// not expose normal scrollY, so the host header mirrors that virtual cycle.
export const SNOW_HEADER_SCROLL_CYCLE_LENGTH = (2.35 + 3 + 5.5) / 0.00075
export const SNOW_HEADER_SCROLL_LOOP_RESET_WINDOW = 1000

export type SnowHeaderScrollState = {
  offset: number
  scrolled?: boolean
  completedCycles?: number
}

export function nextSnowHeaderScrollState(
  previous: SnowHeaderScrollState,
  deltaY: number,
  threshold = SNOW_HEADER_SCROLL_THRESHOLD,
  cycleLength = SNOW_HEADER_SCROLL_CYCLE_LENGTH,
  loopResetWindow = SNOW_HEADER_SCROLL_LOOP_RESET_WINDOW
): Required<SnowHeaderScrollState> {
  const safeDelta = Number.isFinite(deltaY) ? deltaY : 0
  const currentCycleLength =
    Number.isFinite(cycleLength) && cycleLength > 0 ? cycleLength : 0
  const rawOffset = Math.max(
    0,
    (previous.completedCycles ?? 0) * currentCycleLength +
      previous.offset +
      safeDelta
  )
  const completedCycles =
    currentCycleLength > 0 ? Math.floor(rawOffset / currentCycleLength) : 0
  const offset =
    currentCycleLength > 0 ? rawOffset % currentCycleLength : rawOffset
  const resetWindow =
    completedCycles > 0 && Number.isFinite(loopResetWindow)
      ? Math.max(threshold, loopResetWindow)
      : threshold

  return {
    completedCycles,
    offset,
    scrolled: offset > resetWindow,
  }
}
