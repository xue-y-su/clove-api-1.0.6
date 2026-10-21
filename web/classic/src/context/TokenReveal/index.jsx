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
import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react';
import { Button, Input, Modal, Space, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API } from '../../helpers/api';
import { UserContext } from '../User';
import { createTokenRevealMemory } from '../../helpers/token-reveal-memory';

const TokenRevealContext = createContext(null);

const errorCode = (error) => error?.response?.data?.code || '';
const errorMessage = (error, fallback) =>
  error?.response?.data?.message || error?.message || fallback;

export const TokenRevealProvider = ({ children }) => {
  const { t } = useTranslation();
  const [userState] = useContext(UserContext);
  const memoryRef = useRef(createTokenRevealMemory());
  const waiterRef = useRef(null);
  const pendingRef = useRef({});
  const generationRef = useRef(0);
  const userIdRef = useRef(userState?.user?.id);
  const timerRef = useRef();
  const [resolvedKeys, setResolvedKeys] = useState({});
  const [loadingKeys, setLoadingKeys] = useState({});
  const [visible, setVisible] = useState(false);
  const [setupMode, setSetupMode] = useState(false);
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [sending, setSending] = useState(false);

  const clear = useCallback(() => {
    generationRef.current += 1;
    clearTimeout(timerRef.current);
    memoryRef.current.clear();
    pendingRef.current = {};
    setResolvedKeys({});
    setLoadingKeys({});
  }, []);

  const updateWindow = useCallback(
    (until) => {
      memoryRef.current.setVerifiedUntil(until);
      clearTimeout(timerRef.current);
      const delay = memoryRef.current.getVerifiedUntilMs() - Date.now();
      if (delay <= 0) return clear();
      timerRef.current = setTimeout(clear, delay);
    },
    [clear],
  );

  const settle = useCallback((reason) => {
    const waiter = waiterRef.current;
    waiterRef.current = null;
    setVisible(false);
    setPassword('');
    setConfirmPassword('');
    setCode('');
    setError('');
    if (!waiter) return;
    if (reason) waiter.reject(reason);
    else waiter.resolve();
  }, []);

  const requestVerification = useCallback(() => {
    if (waiterRef.current) return waiterRef.current.promise;
    let resolve;
    let reject;
    const promise = new Promise((ok, fail) => {
      resolve = ok;
      reject = fail;
    });
    waiterRef.current = { promise, resolve, reject };
    setSetupMode(false);
    setError('');
    setVisible(true);
    return promise;
  }, []);

  const withVerification = useCallback(
    async (operation) => {
      try {
        return await operation();
      } catch (requestError) {
        if (errorCode(requestError) !== 'TOKEN_REVEAL_VERIFICATION_REQUIRED') {
          throw requestError;
        }
        clear();
        await requestVerification();
        return operation();
      }
    },
    [clear, requestVerification],
  );

  const resolveKey = useCallback(
    async (id) => {
      const cached = memoryRef.current.getKey(id);
      if (cached) return cached;
      if (pendingRef.current[id]) return pendingRef.current[id];
      const request = withVerification(async () => {
        const generation = generationRef.current;
        setLoadingKeys((current) => ({ ...current, [id]: true }));
        try {
          const response = await API.post(`/api/token/${id}/key`, undefined, {
            skipErrorHandler: true,
          });
          if (generation !== generationRef.current) {
            throw new Error('token reveal cancelled');
          }
          if (!response.data?.success || !response.data?.data?.key) {
            throw new Error(response.data?.message || t('获取令牌密钥失败'));
          }
          updateWindow(response.headers['x-token-reveal-verified-until']);
          memoryRef.current.setKey(id, response.data.data.key);
          const key = memoryRef.current.getKey(id);
          if (!key) throw new Error(t('密钥查看窗口已过期'));
          setResolvedKeys(memoryRef.current.snapshot());
          return key;
        } finally {
          setLoadingKeys((current) => {
            const next = { ...current };
            delete next[id];
            return next;
          });
        }
      }).finally(() => delete pendingRef.current[id]);
      pendingRef.current[id] = request;
      return request;
    },
    [t, updateWindow, withVerification],
  );

  const resolveKeys = useCallback(
    async (ids) => {
      const unique = [...new Set(ids.filter((id) => Number(id) > 0))];
      const cached = {};
      const missing = [];
      unique.forEach((id) => {
        const key = memoryRef.current.getKey(id);
        if (key) cached[id] = key;
        else missing.push(id);
      });
      if (!missing.length) return cached;
      const revealed = await withVerification(async () => {
        const generation = generationRef.current;
        setLoadingKeys((current) => {
          const next = { ...current };
          missing.forEach((id) => (next[id] = true));
          return next;
        });
        try {
          const response = await API.post(
            '/api/token/batch/keys',
            { ids: missing },
            { skipErrorHandler: true },
          );
          if (generation !== generationRef.current) {
            throw new Error('token reveal cancelled');
          }
          if (!response.data?.success || !response.data?.data?.keys) {
            throw new Error(response.data?.message || t('获取令牌密钥失败'));
          }
          updateWindow(response.headers['x-token-reveal-verified-until']);
          memoryRef.current.setKeys(response.data.data.keys);
          const snapshot = memoryRef.current.snapshot();
          setResolvedKeys(snapshot);
          return snapshot;
        } finally {
          setLoadingKeys((current) => {
            const next = { ...current };
            missing.forEach((id) => delete next[id]);
            return next;
          });
        }
      });
      return { ...cached, ...revealed };
    },
    [t, updateWindow, withVerification],
  );

  useEffect(() => {
    if (userIdRef.current === userState?.user?.id) return;
    userIdRef.current = userState?.user?.id;
    clear();
    if (!userState?.user?.id && waiterRef.current) {
      settle(new Error('token reveal cancelled'));
    }
  }, [clear, settle, userState?.user?.id]);
  useEffect(
    () => () => {
      clear();
      waiterRef.current?.reject(new Error('token reveal provider unmounted'));
    },
    [clear],
  );

  const verify = async () => {
    if (!password) return setError(t('请输入密码'));
    setSubmitting(true);
    setError('');
    try {
      const response = await API.post(
        '/api/token/reveal/verify',
        { password },
        { skipErrorHandler: true },
      );
      updateWindow(response.data?.data?.verified_until);
      settle();
    } catch (requestError) {
      if (errorCode(requestError) === 'PASSWORD_SETUP_REQUIRED') {
        setSetupMode(true);
        setError('');
      } else {
        setError(errorMessage(requestError, t('密码验证失败')));
      }
    } finally {
      setSubmitting(false);
    }
  };

  const sendCode = async () => {
    setSending(true);
    setError('');
    try {
      await API.post('/api/user/self/password/setup/code', undefined, {
        skipErrorHandler: true,
      });
    } catch (requestError) {
      setError(errorMessage(requestError, t('验证码发送失败')));
    } finally {
      setSending(false);
    }
  };

  const setup = async () => {
    if (password.length < 8 || password.length > 20)
      return setError(t('密码长度必须为 8 到 20 个字符'));
    if (password !== confirmPassword)
      return setError(t('两次输入的密码不一致'));
    if (!code.trim()) return setError(t('请输入邮箱验证码'));
    setSubmitting(true);
    setError('');
    try {
      await API.post(
        '/api/user/self/password/setup',
        { password, verification_code: code.trim() },
        { skipErrorHandler: true },
      );
      const response = await API.post(
        '/api/token/reveal/verify',
        { password },
        { skipErrorHandler: true },
      );
      updateWindow(response.data?.data?.verified_until);
      settle();
    } catch (requestError) {
      setError(errorMessage(requestError, t('设置密码失败')));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <TokenRevealContext.Provider
      value={{ resolveKey, resolveKeys, resolvedKeys, loadingKeys, clear }}
    >
      {children}
      <Modal
        visible={visible}
        title={setupMode ? t('设置账户密码') : t('验证账户密码')}
        onCancel={() =>
          !submitting && settle(new Error('token reveal cancelled'))
        }
        closeOnEsc={!submitting}
        maskClosable={false}
        footer={
          <Space>
            <Button
              type='tertiary'
              disabled={submitting}
              onClick={() => settle(new Error('token reveal cancelled'))}
            >
              {t('取消')}
            </Button>
            <Button loading={submitting} onClick={setupMode ? setup : verify}>
              {setupMode ? t('设置密码') : t('验证')}
            </Button>
          </Space>
        }
      >
        <Typography.Paragraph type='tertiary'>
          {setupMode
            ? t('验证绑定邮箱并设置密码后才能查看完整密钥。')
            : t('验证后 30 分钟内可以查看自己的全部密钥。')}
        </Typography.Paragraph>
        <Space vertical align='start' style={{ width: '100%' }}>
          {setupMode && <Input value={userState?.user?.email || ''} readOnly />}
          <Input
            type='password'
            value={password}
            placeholder={t('密码')}
            onChange={setPassword}
          />
          {setupMode && (
            <>
              <Input
                type='password'
                value={confirmPassword}
                placeholder={t('确认密码')}
                onChange={setConfirmPassword}
              />
              <Space>
                <Input
                  value={code}
                  placeholder={t('邮箱验证码')}
                  onChange={setCode}
                />
                <Button loading={sending} onClick={sendCode}>
                  {t('发送验证码')}
                </Button>
              </Space>
            </>
          )}
          {error && <Typography.Text type='danger'>{error}</Typography.Text>}
        </Space>
      </Modal>
    </TokenRevealContext.Provider>
  );
};

export const useTokenReveal = () => {
  const context = useContext(TokenRevealContext);
  if (!context) throw new Error('useTokenReveal must be used within provider');
  return context;
};
