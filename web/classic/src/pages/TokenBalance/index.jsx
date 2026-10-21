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

import {
  IconEyeClosed,
  IconEyeOpened,
  IconKey,
  IconSearch,
} from '@douyinfe/semi-icons';
import { Button, Card, Input, Typography } from '@douyinfe/semi-ui';
import React, { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { queryTokenBalance } from './api';
import BalanceQueryError from './components/BalanceQueryError';
import BalanceResult from './components/BalanceResult';

const TokenBalance = () => {
  const { t } = useTranslation();
  const inputRef = useRef(null);
  const [key, setKey] = useState('');
  const [showKey, setShowKey] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [result, setResult] = useState(null);

  const handleSubmit = async (event) => {
    event.preventDefault();
    const normalizedKey = key.trim();
    if (!normalizedKey) {
      setResult(null);
      setError({ code: 'invalid_request' });
      inputRef.current?.focus();
      return;
    }

    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const response = await queryTokenBalance(normalizedKey);
      if (!response.success || !response.data) {
        setError({
          code: response.code || 'query_failed',
          retryAfterSeconds: response.retry_after_seconds,
        });
        setKey('');
        setShowKey(false);
        return;
      }
      setResult(response.data);
      setKey('');
      setShowKey(false);
    } catch {
      setError({ code: 'query_failed' });
      setKey('');
      setShowKey(false);
    } finally {
      setLoading(false);
    }
  };

  return (
    <main className='classic-page-fill mx-auto w-full max-w-4xl px-3 py-5 sm:px-5 sm:py-8'>
      <div className='mb-6 flex items-center gap-3'>
        <div className='flex size-9 shrink-0 items-center justify-center rounded-lg border border-semi-color-border bg-semi-color-fill-0'>
          <IconKey size='large' />
        </div>
        <Typography.Title heading={3} className='!mb-0 !text-2xl'>
          {t('API Key 余额查询')}
        </Typography.Title>
      </div>

      <div className='space-y-4'>
        <Card className='!rounded-lg' bodyStyle={{ padding: 20 }}>
          <form onSubmit={handleSubmit} className='space-y-2.5'>
            <label
              htmlFor='token-balance-key'
              className='block text-sm font-medium text-semi-color-text-0'
            >
              {t('API Key')}
            </label>
            <div className='flex flex-col gap-2 sm:flex-row'>
              <Input
                id='token-balance-key'
                forwardRef={inputRef}
                type={showKey ? 'text' : 'password'}
                value={key}
                onChange={(value) => {
                  setKey(value);
                  if (error?.code === 'invalid_request') setError(null);
                }}
                placeholder={t('请输入 API Key')}
                autoComplete='off'
                autoCapitalize='none'
                spellCheck={false}
                validateStatus={error ? 'error' : 'default'}
                aria-invalid={error !== null}
                className='min-w-0 flex-1 font-mono'
                suffix={
                  <Button
                    type='tertiary'
                    theme='borderless'
                    size='small'
                    icon={showKey ? <IconEyeClosed /> : <IconEyeOpened />}
                    onClick={() => setShowKey((visible) => !visible)}
                    aria-label={showKey ? t('隐藏 API Key') : t('显示 API Key')}
                    title={showKey ? t('隐藏 API Key') : t('显示 API Key')}
                  />
                }
              />
              <Button
                htmlType='submit'
                type='primary'
                theme='solid'
                icon={<IconSearch />}
                loading={loading}
                disabled={loading}
                className='sm:min-w-28'
              >
                {loading ? t('查询中...') : t('查询余额')}
              </Button>
            </div>
          </form>
        </Card>

        {error ? (
          <BalanceQueryError
            code={error.code}
            retryAfterSeconds={error.retryAfterSeconds}
            t={t}
          />
        ) : null}
        {result ? <BalanceResult data={result} t={t} /> : null}
      </div>
    </main>
  );
};

export default TokenBalance;
