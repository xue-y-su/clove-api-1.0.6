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
export function buildCatalogVisibilityDraft(state) {
  return {
    enabled: state.config?.enabled ?? false,
    defaultMode: state.config?.default_mode ?? 'all',
    rules: cloneRules(state.config?.rules ?? {}),
    groups: sortAdminGroups(state.groups ?? []),
    userGroups: sortAdminGroups(state.user_groups ?? []),
    etag: state.etag,
    summary: { ...(state.summary ?? {}) },
    configError: state.config_error,
  };
}

export function setCatalogVisibilityEnabled(draft, enabled) {
  return { ...draft, enabled };
}

export function filterCatalogGroups(groups, query) {
  const normalized = query.trim().toLocaleLowerCase();
  if (!normalized) return groups;
  return groups.filter((group) =>
    `${group.name} ${group.description}`
      .toLocaleLowerCase()
      .includes(normalized),
  );
}

export function getCatalogRule(draft, catalogGroup) {
  const rule = draft.rules[catalogGroup];
  if (rule) {
    return { mode: rule.mode, userGroups: [...rule.userGroups] };
  }
  if (draft.defaultMode === 'all') {
    return { mode: 'all', userGroups: [] };
  }
  return { mode: 'selected', userGroups: [] };
}

export function getRestrictedCatalogGroupCount(draft) {
  if (!draft.enabled) return 0;
  return draft.groups.reduce(
    (count, group) =>
      count + (getCatalogRule(draft, group.name).mode === 'selected' ? 1 : 0),
    0,
  );
}

export function isCatalogRuleExplicit(draft, catalogGroup) {
  return Object.hasOwn(draft.rules, catalogGroup);
}

export function setCatalogRuleMode(draft, catalogGroup, mode) {
  const current = getCatalogRule(draft, catalogGroup);
  let userGroups = [];
  if (mode === 'selected') {
    userGroups = current.userGroups;
    if (
      userGroups.length === 0 &&
      draft.userGroups.some((group) => group.name === catalogGroup)
    ) {
      userGroups = [catalogGroup];
    }
  }
  return setCatalogRule(draft, catalogGroup, { mode, userGroups });
}

export function setCatalogRuleUserGroup(
  draft,
  catalogGroup,
  userGroup,
  checked,
) {
  const current = getCatalogRule(draft, catalogGroup);
  const userGroups = new Set(current.userGroups);
  if (checked) userGroups.add(userGroup);
  else userGroups.delete(userGroup);
  return setCatalogRule(draft, catalogGroup, {
    mode: 'selected',
    userGroups: [...userGroups].sort(),
  });
}

export function clearCatalogRule(draft, catalogGroup) {
  const rules = { ...draft.rules };
  delete rules[catalogGroup];
  return { ...draft, rules };
}

export function getMissingUserGroups(draft, catalogGroup) {
  const known = new Set(draft.userGroups.map((group) => group.name));
  return getCatalogRule(draft, catalogGroup).userGroups.filter(
    (group) => !known.has(group),
  );
}

export function toVisibilityUpdate(draft) {
  const rules = {};
  for (const [catalogGroup, rule] of Object.entries(draft.rules)) {
    const userGroups = normalizeUserGroups(rule.userGroups);
    if (rule.mode === 'all') {
      if (draft.defaultMode !== 'all') {
        rules[catalogGroup] = { mode: 'all', user_groups: [] };
      }
      continue;
    }
    if (draft.defaultMode === 'hidden' && userGroups.length === 0) {
      continue;
    }
    rules[catalogGroup] = { mode: 'selected', user_groups: userGroups };
  }
  return {
    config: {
      version: 3,
      enabled: draft.enabled,
      default_mode: draft.defaultMode,
      rules,
    },
    expected_etag: draft.etag,
  };
}

function setCatalogRule(draft, catalogGroup, rule) {
  return {
    ...draft,
    rules: {
      ...draft.rules,
      [catalogGroup]: {
        mode: rule.mode,
        userGroups: normalizeUserGroups(rule.userGroups),
      },
    },
  };
}

function cloneRules(rules) {
  const cloned = {};
  for (const [catalogGroup, rule] of Object.entries(rules)) {
    cloned[catalogGroup] = {
      mode: rule.mode,
      userGroups: normalizeUserGroups(rule.user_groups ?? []),
    };
  }
  return cloned;
}

function normalizeUserGroups(groups) {
  return [...new Set(groups)].sort();
}

function sortAdminGroups(groups) {
  return groups
    .map((group) => ({ ...group }))
    .sort((left, right) => {
      if (left.exists !== right.exists) return left.exists ? -1 : 1;
      return (left.description || left.name).localeCompare(
        right.description || right.name,
      );
    });
}
