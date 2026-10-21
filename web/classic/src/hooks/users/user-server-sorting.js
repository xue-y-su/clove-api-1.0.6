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

const USER_SORTABLE_COLUMNS = new Set([
  'id',
  'username',
  'quota',
  'group',
  'created_at',
  'last_login_at',
]);

export const DEFAULT_USER_SORT = Object.freeze({
  sortBy: 'id',
  sortOrder: 'desc',
});

export const resolveUserTableSort = (changeInfo) => {
  const dataIndex = changeInfo?.sorter?.dataIndex;
  const semiSortOrder = changeInfo?.sorter?.sortOrder;
  if (
    !USER_SORTABLE_COLUMNS.has(dataIndex) ||
    (semiSortOrder !== 'ascend' && semiSortOrder !== 'descend')
  ) {
    return DEFAULT_USER_SORT;
  }
  return {
    sortBy: dataIndex,
    sortOrder: semiSortOrder === 'ascend' ? 'asc' : 'desc',
  };
};

export const getSemiUserSortOrder = (sorting, dataIndex) => {
  if (sorting?.sortBy !== dataIndex) {
    return false;
  }
  return sorting.sortOrder === 'asc' ? 'ascend' : 'descend';
};
