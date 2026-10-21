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
import { describe, expect, test } from 'bun:test'

import {
  SNOW_HEADER_SCROLL_CYCLE_LENGTH,
  SNOW_HEADER_SCROLL_THRESHOLD,
  nextSnowHeaderScrollState,
} from './snow-header-scroll'

describe('snow header scroll state', () => {
  test('shrinks the public header after Snow receives enough downward scroll', () => {
    const state = nextSnowHeaderScrollState({ offset: 0 }, 30)

    expect(state.completedCycles).toBe(0)
    expect(state.offset).toBe(30)
    expect(state.scrolled).toBe(true)
  })

  test('keeps the public header expanded for small Snow scroll movement', () => {
    const state = nextSnowHeaderScrollState(
      { offset: 0 },
      SNOW_HEADER_SCROLL_THRESHOLD
    )

    expect(state.offset).toBe(SNOW_HEADER_SCROLL_THRESHOLD)
    expect(state.scrolled).toBe(false)
  })

  test('restores the expanded header when Snow scrolls back to the top', () => {
    const state = nextSnowHeaderScrollState({ offset: 120 }, -200)

    expect(state.offset).toBe(0)
    expect(state.scrolled).toBe(false)
  })

  test('restores the expanded header when Snow loops back to its opening scene', () => {
    const state = nextSnowHeaderScrollState(
      { offset: SNOW_HEADER_SCROLL_CYCLE_LENGTH - 10 },
      10
    )

    expect(state.offset).toBe(0)
    expect(state.scrolled).toBe(false)
  })

  test('keeps the expanded header around Snow opening after a wheel overshoots the loop boundary', () => {
    const state = nextSnowHeaderScrollState(
      { offset: SNOW_HEADER_SCROLL_CYCLE_LENGTH - 100 },
      900
    )

    expect(state.completedCycles).toBe(1)
    expect(state.offset).toBeCloseTo(800)
    expect(state.scrolled).toBe(false)
  })
})
