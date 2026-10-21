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
import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Banner,
  Button,
  Checkbox,
  Input,
  Radio,
  RadioGroup,
  Spin,
  Switch,
  Tag,
} from '@douyinfe/semi-ui';
import { RefreshCw, RotateCcw, Save, Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { API, showSuccess } from '../../../helpers';
import {
  buildCatalogVisibilityDraft,
  clearCatalogRule,
  filterCatalogGroups,
  getCatalogRule,
  getMissingUserGroups,
  getRestrictedCatalogGroupCount,
  isCatalogRuleExplicit,
  setCatalogRuleMode,
  setCatalogRuleUserGroup,
  setCatalogVisibilityEnabled,
  toVisibilityUpdate,
} from './model-catalog-visibility-state';

const endpoint = '/api/option/model-catalog-visibility';

export default function ModelCatalogGroupVisibility() {
  const { t } = useTranslation();
  const [draft, setDraft] = useState(null);
  const [query, setQuery] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      for (let attempt = 0; attempt < 3; attempt += 1) {
        try {
          const response = await API.get(endpoint);
          if (!response.data.success) throw new Error(response.data.message);
          setDraft(buildCatalogVisibilityDraft(response.data.data));
          return;
        } catch {
          if (attempt < 2) {
            await new Promise((resolve) =>
              setTimeout(resolve, 100 * (attempt + 1)),
            );
          }
        }
      }
      setError(t('Failed to load catalog visibility'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const groups = useMemo(
    () => (draft ? filterCatalogGroups(draft.groups, query) : []),
    [draft, query],
  );

  const setEnabled = (enabled) => {
    setDraft((current) =>
      current ? setCatalogVisibilityEnabled(current, enabled) : current,
    );
  };

  const setGroupMode = (catalogGroup, mode) => {
    setDraft((current) =>
      current ? setCatalogRuleMode(current, catalogGroup, mode) : current,
    );
  };

  const setUserGroup = (catalogGroup, userGroup, checked) => {
    setDraft((current) =>
      current
        ? setCatalogRuleUserGroup(current, catalogGroup, userGroup, checked)
        : current,
    );
  };

  const useDefaultRule = (catalogGroup) => {
    setDraft((current) =>
      current ? clearCatalogRule(current, catalogGroup) : current,
    );
  };

  const save = async () => {
    if (!draft) return;
    setSaving(true);
    setError('');
    try {
      const response = await API.put(endpoint, toVisibilityUpdate(draft));
      if (!response.data.success) throw new Error(response.data.message);
      await load();
      showSuccess(t('Save visibility'));
    } catch (requestError) {
      if (requestError.response?.status === 409) {
        setError(t('Catalog visibility changed; refresh before saving'));
      } else {
        setError(t('Failed to save catalog visibility'));
      }
    } finally {
      setSaving(false);
    }
  };

  if (loading && !draft) {
    return (
      <div className='flex justify-center p-8'>
        <Spin spinning />
      </div>
    );
  }

  return (
    <div className='space-y-4 pb-16 md:pb-0'>
      {error ? (
        <Banner
          type='danger'
          description={error}
          closeIcon={null}
          action={
            <Button icon={<RefreshCw size={16} />} onClick={load}>
              {t('Refresh')}
            </Button>
          }
        />
      ) : null}
      {draft?.configError ? (
        <Banner
          type='warning'
          description={t('Invalid stored catalog visibility')}
          closeIcon={null}
        />
      ) : null}
      {draft ? (
        <>
          <div className='flex flex-col gap-3 rounded border p-4 sm:flex-row sm:items-center sm:justify-between'>
            <div className='min-w-0'>
              <div className='font-medium'>
                {t('Enable custom model catalog visibility rules')}
              </div>
              <div className='mt-1 text-sm text-gray-500'>
                {t(
                  'Control which user groups can see each model group in the model catalog.',
                )}
              </div>
            </div>
            <Switch checked={draft.enabled} onChange={setEnabled} />
          </div>

          <div className='grid grid-cols-1 gap-3 sm:grid-cols-3'>
            <div>
              {t('Catalog groups')}:{' '}
              {draft.summary.group_count ?? draft.groups.length}
            </div>
            <div>
              {t('Visible models')}: {draft.summary.model_count ?? 0}
            </div>
            <div>
              {t('Restricted groups')}: {getRestrictedCatalogGroupCount(draft)}
            </div>
          </div>

          {draft.enabled ? (
            <div className='space-y-3'>
              <Input
                prefix={<Search size={16} />}
                value={query}
                onChange={setQuery}
                placeholder={t('Search catalog groups')}
                showClear
              />
              <div className='space-y-3'>
                {groups.map((group) => {
                  const rule = getCatalogRule(draft, group.name);
                  const explicit = isCatalogRuleExplicit(draft, group.name);
                  const missingUserGroups = getMissingUserGroups(
                    draft,
                    group.name,
                  );
                  return (
                    <div
                      key={group.name}
                      className='space-y-3 rounded border p-3'
                    >
                      <div className='flex min-w-0 flex-wrap items-start justify-between gap-2'>
                        <div className='min-w-0'>
                          <strong className='block break-words'>
                            {group.description || group.name}
                          </strong>
                          <span className='block text-xs break-all text-gray-500'>
                            {group.name}
                            {!group.exists ? ` · ${t('No longer exists')}` : ''}
                          </span>
                        </div>
                        <div className='flex items-center gap-2'>
                          <Tag color={explicit ? 'blue' : 'grey'}>
                            {t(explicit ? 'Custom rule' : 'Default rule')}
                          </Tag>
                          {explicit ? (
                            <Button
                              theme='borderless'
                              icon={<RotateCcw size={15} />}
                              onClick={() => useDefaultRule(group.name)}
                            >
                              {t('Use default rule')}
                            </Button>
                          ) : null}
                        </div>
                      </div>

                      <RadioGroup
                        type='button'
                        value={rule.mode}
                        onChange={(event) =>
                          setGroupMode(group.name, event.target.value)
                        }
                      >
                        <Radio value='all'>{t('All user groups')}</Radio>
                        <Radio value='selected'>
                          {t('Specified user groups')}
                        </Radio>
                      </RadioGroup>

                      {rule.mode === 'selected' ? (
                        <div className='space-y-2'>
                          {rule.userGroups.length === 0 ? (
                            <Banner
                              type='warning'
                              closeIcon={null}
                              description={t(
                                'This catalog group is hidden from every user group.',
                              )}
                            />
                          ) : null}
                          {missingUserGroups.length > 0 ? (
                            <Banner
                              type='warning'
                              closeIcon={null}
                              description={`${t('User group no longer exists')}: ${missingUserGroups.join(', ')}`}
                            />
                          ) : null}
                          {draft.userGroups.length > 0 ? (
                            <div className='grid max-h-72 grid-cols-1 gap-2 overflow-y-auto rounded border p-2 sm:grid-cols-2 lg:grid-cols-3'>
                              {draft.userGroups.map((userGroup) => (
                                <label
                                  key={userGroup.name}
                                  className='flex min-w-0 cursor-pointer items-start gap-2 rounded p-2'
                                >
                                  <Checkbox
                                    checked={rule.userGroups.includes(
                                      userGroup.name,
                                    )}
                                    onChange={(event) =>
                                      setUserGroup(
                                        group.name,
                                        userGroup.name,
                                        event.target.checked,
                                      )
                                    }
                                  />
                                  <span className='min-w-0 break-words'>
                                    {userGroup.description || userGroup.name}
                                    <span className='block text-xs break-all text-gray-500'>
                                      {userGroup.name}
                                    </span>
                                  </span>
                                </label>
                              ))}
                            </div>
                          ) : (
                            <Banner
                              type='warning'
                              closeIcon={null}
                              description={t('No user groups available')}
                            />
                          )}
                        </div>
                      ) : null}
                    </div>
                  );
                })}
                {groups.length === 0 ? (
                  <div className='rounded border p-6 text-center text-gray-500'>
                    {t('No catalog groups match')}
                  </div>
                ) : null}
              </div>
            </div>
          ) : (
            <Banner
              type='info'
              closeIcon={null}
              title={t('Using original model catalog rules')}
              description={t(
                'Custom visibility rules are disabled. The model catalog uses its original display rules. Saved custom rules are retained and will apply again when enabled.',
              )}
            />
          )}

          <div className='sticky bottom-0 border-t bg-white py-3 dark:bg-black'>
            <Button
              theme='solid'
              icon={<Save size={16} />}
              loading={saving}
              onClick={save}
            >
              {t('Save visibility')}
            </Button>
          </div>
        </>
      ) : null}
    </div>
  );
}
