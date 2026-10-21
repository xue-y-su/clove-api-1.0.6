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

const unavailableCapability = Object.freeze({
  available: false,
  native: false,
  tavily: false,
  reason: 'not_configured',
});

const isCapability = (value) =>
  value !== null &&
  typeof value === 'object' &&
  typeof value.available === 'boolean' &&
  typeof value.native === 'boolean' &&
  typeof value.tavily === 'boolean' &&
  typeof value.reason === 'string';

export const getPlaygroundWebSearchCapability = (status) => {
  const nested = status?.data?.playground_web_search;
  if (isCapability(nested)) return nested;

  const direct = status?.playground_web_search;
  if (isCapability(direct)) return direct;

  return unavailableCapability;
};
