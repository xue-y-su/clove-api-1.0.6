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
import { describe, expect, test } from 'bun:test';

import {
  CHANNEL_OPTIONS,
  MODEL_FETCHABLE_CHANNEL_TYPES,
} from '../../constants';
import {
  createUpstreamDiffSelection,
  deriveRetainedModels,
} from './upstreamUpdateUtils';

describe('Sub2API publication preview', () => {
  test('registers channel type 59 as fetchable', () => {
    expect(CHANNEL_OPTIONS.some((option) => option.value === 59)).toBe(true);
    expect(MODEL_FETCHABLE_CHANNEL_TYPES.has(59)).toBe(true);
  });

  test('selects additions but keeps missing removals off by default', () => {
    expect(
      createUpstreamDiffSelection(['new-a', 'new-b'], ['missing-a']),
    ).toEqual({
      selectedAdd: ['new-a', 'new-b'],
      selectedRemove: [],
    });
  });

  test('derives retained models from current models and missing entries', () => {
    expect(deriveRetainedModels('kept,missing, kept ', ['missing'])).toEqual([
      'kept',
    ]);
  });

  test('does not offer unsupported automatic publication for Sub2API', async () => {
    const source = await Bun.file(
      new URL(
        '../../components/table/channels/modals/EditChannelModal.jsx',
        import.meta.url,
      ),
    ).text();
    expect(source).toContain('inputs.type !== 59');
  });

  test('submits the default additions without an extra partial-selection prompt', async () => {
    const source = await Bun.file(
      new URL(
        '../../components/table/channels/modals/ChannelUpstreamUpdateModal.jsx',
        import.meta.url,
      ),
    ).text();

    expect(source).not.toContain('partialSubmitConfirmed');
    expect(source).not.toContain("t('仍有未处理项')");
  });
});
