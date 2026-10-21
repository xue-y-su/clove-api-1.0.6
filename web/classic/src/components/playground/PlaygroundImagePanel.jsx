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
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import {
  Button,
  Empty,
  InputNumber,
  Select,
  Spin,
  TextArea,
  Toast,
  Typography,
} from '@douyinfe/semi-ui';
import {
  Copy,
  Download,
  Image as ImageIcon,
  RefreshCw,
  Square,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { API_ENDPOINTS } from '../../constants/playground.constants';
import {
  API,
  buildPlaygroundImageRequest,
  copy,
  createPlaygroundImagePreviews,
  getPlaygroundImageErrorMessage,
  getPlaygroundImageCapabilities,
  processModelsData,
  resolvePlaygroundImageGroup,
  revokePlaygroundImagePreviews,
  selectFilter,
} from '../../helpers';

const IMAGE_SIZES = [
  '1024x1024',
  '1024x1536',
  '1536x1024',
  '1024x1792',
  '1792x1024',
];

const clampImageCount = (value) => {
  if (!Number.isFinite(value)) return 1;
  return Math.min(4, Math.max(1, Math.trunc(value)));
};

const PlaygroundImagePanel = ({ groups, initialGroup }) => {
  const { t } = useTranslation();
  const [group, setGroup] = useState(initialGroup || '');
  const [model, setModel] = useState('');
  const [models, setModels] = useState([]);
  const [modelsLoading, setModelsLoading] = useState(false);
  const [modelsError, setModelsError] = useState('');
  const [prompt, setPrompt] = useState('');
  const [size, setSize] = useState('1024x1024');
  const [count, setCount] = useState(1);
  const [quality, setQuality] = useState('');
  const [style, setStyle] = useState('');
  const [background, setBackground] = useState('auto');
  const [images, setImages] = useState([]);
  const [generationError, setGenerationError] = useState('');
  const [isGenerating, setIsGenerating] = useState(false);
  const generationControllerRef = useRef(null);
  const modelControllerRef = useRef(null);
  const modelRequestRef = useRef(0);
  const imagesRef = useRef([]);
  const capabilities = useMemo(
    () => getPlaygroundImageCapabilities(model),
    [model],
  );

  const replaceImages = useCallback((nextImages) => {
    revokePlaygroundImagePreviews(imagesRef.current);
    imagesRef.current = nextImages;
    setImages(nextImages);
  }, []);

  const loadModels = useCallback(async () => {
    modelControllerRef.current?.abort();
    const requestId = modelRequestRef.current + 1;
    modelRequestRef.current = requestId;
    setModels([]);
    setModel('');
    if (!group) {
      setModelsLoading(false);
      return;
    }

    const controller = new AbortController();
    modelControllerRef.current = controller;
    setModelsLoading(true);
    setModelsError('');
    try {
      const response = await API.get(API_ENDPOINTS.USER_MODELS, {
        params: { group },
        signal: controller.signal,
        skipErrorHandler: true,
      });
      if (controller.signal.aborted || modelRequestRef.current !== requestId) {
        return;
      }
      if (!response.data?.success) {
        throw new Error(response.data?.message || t('加载模型失败'));
      }
      const { modelOptions, selectedModel } = processModelsData(
        response.data.data || [],
        '',
      );
      setModels(modelOptions);
      setModel(selectedModel || '');
    } catch (error) {
      if (controller.signal.aborted || modelRequestRef.current !== requestId) {
        return;
      }
      setModels([]);
      setModel('');
      setModelsError(
        error?.response?.data?.message || error?.message || t('加载模型失败'),
      );
    } finally {
      if (
        modelControllerRef.current === controller &&
        modelRequestRef.current === requestId
      ) {
        modelControllerRef.current = null;
        setModelsLoading(false);
      }
    }
  }, [group, t]);

  const handleGroupChange = useCallback(
    (value) => {
      if (!value || value === group) return;
      modelControllerRef.current?.abort();
      modelRequestRef.current += 1;
      setModels([]);
      setModel('');
      setModelsError('');
      setGroup(value);
    },
    [group],
  );

  useEffect(() => {
    const nextGroup = resolvePlaygroundImageGroup(
      groups,
      group || initialGroup,
    );
    if (nextGroup !== group) {
      setGroup(nextGroup);
    }
  }, [group, groups, initialGroup]);

  useEffect(() => {
    void loadModels();
  }, [loadModels]);

  useEffect(() => {
    setQuality(capabilities.quality[0] || '');
    setStyle(capabilities.style[0] || '');
    if (!capabilities.background) setBackground('auto');
  }, [capabilities]);

  useEffect(
    () => () => {
      modelRequestRef.current += 1;
      modelControllerRef.current?.abort();
      generationControllerRef.current?.abort();
      revokePlaygroundImagePreviews(imagesRef.current);
      imagesRef.current = [];
    },
    [],
  );

  const handleGenerate = async () => {
    const trimmedPrompt = prompt.trim();
    if (!trimmedPrompt || !model || isGenerating) return;

    const controller = new AbortController();
    generationControllerRef.current = controller;
    setGenerationError('');
    setIsGenerating(true);
    try {
      const response = await API.post(
        API_ENDPOINTS.IMAGE_GENERATIONS,
        buildPlaygroundImageRequest({
          background,
          count,
          group,
          model,
          prompt: trimmedPrompt,
          quality,
          size,
          style,
        }),
        { signal: controller.signal, skipErrorHandler: true },
      );
      if (controller.signal.aborted) return;
      replaceImages(createPlaygroundImagePreviews(response.data?.data || []));
    } catch (error) {
      if (controller.signal.aborted) return;
      const errorMessage = t(
        getPlaygroundImageErrorMessage(error, t('未知错误')),
      );
      setGenerationError(errorMessage);
      Toast.error({
        content: `${t('图片生成失败')}: ${errorMessage}`,
        duration: 4,
      });
    } finally {
      if (generationControllerRef.current === controller) {
        generationControllerRef.current = null;
        setIsGenerating(false);
      }
    }
  };

  const handleStop = () => {
    generationControllerRef.current?.abort();
    generationControllerRef.current = null;
    setIsGenerating(false);
  };

  const copyImageURL = async (url) => {
    const success = await copy(url);
    if (success) {
      Toast.success(t('已复制到剪贴板'));
    } else {
      Toast.error(t('复制失败'));
    }
  };

  const downloadImage = (image, index) => {
    const link = document.createElement('a');
    link.href = image.src;
    link.download = `playground-image-${index + 1}.png`;
    link.rel = 'noreferrer';
    link.click();
  };

  return (
    <div className='h-full overflow-y-auto bg-semi-color-bg-0'>
      <div className='mx-auto w-full max-w-5xl p-4 sm:p-6'>
        <div className='mb-5 flex items-center gap-3'>
          <div className='flex h-9 w-9 items-center justify-center rounded-md bg-semi-color-primary-light-default text-semi-color-primary'>
            <ImageIcon size={18} aria-hidden='true' />
          </div>
          <div>
            <Typography.Title heading={5} className='!mb-0'>
              {t('图片生成')}
            </Typography.Title>
            <Typography.Text type='tertiary' size='small'>
              {model || t('选择模型')}
            </Typography.Text>
          </div>
        </div>

        <div className='rounded-md border border-semi-color-border bg-semi-color-bg-1 p-4'>
          <label
            className='mb-2 block text-sm font-medium'
            htmlFor='classic-image-prompt'
          >
            {t('提示词')}
          </label>
          <TextArea
            id='classic-image-prompt'
            value={prompt}
            onChange={setPrompt}
            placeholder={t('描述要生成的图片')}
            autosize={{ minRows: 4, maxRows: 10 }}
            disabled={isGenerating}
          />

          <div className='mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
            <label className='grid gap-2 text-sm font-medium'>
              {t('分组')}
              <Select
                aria-label={t('分组')}
                value={group}
                onChange={handleGroupChange}
                optionList={groups}
                style={{ width: '100%' }}
                disabled={isGenerating}
              />
            </label>

            <label className='grid gap-2 text-sm font-medium sm:col-span-1 lg:col-span-2'>
              {t('模型')}
              <Select
                aria-label={t('模型')}
                value={model}
                onChange={setModel}
                optionList={models}
                filter={selectFilter}
                searchPosition='dropdown'
                searchPlaceholder={t('搜索模型...')}
                autoClearSearchValue={false}
                placeholder={t('搜索模型...')}
                style={{ width: '100%' }}
                loading={modelsLoading}
                disabled={isGenerating || modelsLoading || models.length === 0}
              />
            </label>

            <label className='grid gap-2 text-sm font-medium'>
              {t('数量')}
              <InputNumber
                value={count}
                onChange={(value) => setCount(clampImageCount(Number(value)))}
                min={1}
                max={4}
                style={{ width: '100%' }}
                disabled={isGenerating}
              />
            </label>

            <label className='grid gap-2 text-sm font-medium'>
              {t('尺寸')}
              <Select
                aria-label={t('尺寸')}
                value={size}
                onChange={setSize}
                optionList={IMAGE_SIZES.map((value) => ({
                  label: value,
                  value,
                }))}
                style={{ width: '100%' }}
                disabled={isGenerating}
              />
            </label>

            {capabilities.quality.length > 0 && (
              <label className='grid gap-2 text-sm font-medium'>
                {t('质量')}
                <Select
                  aria-label={t('质量')}
                  value={quality}
                  onChange={setQuality}
                  optionList={capabilities.quality.map((value) => ({
                    label: value,
                    value,
                  }))}
                  style={{ width: '100%' }}
                  disabled={isGenerating}
                />
              </label>
            )}

            {capabilities.style.length > 0 && (
              <label className='grid gap-2 text-sm font-medium'>
                {t('风格')}
                <Select
                  aria-label={t('风格')}
                  value={style}
                  onChange={setStyle}
                  optionList={capabilities.style.map((value) => ({
                    label: value,
                    value,
                  }))}
                  style={{ width: '100%' }}
                  disabled={isGenerating}
                />
              </label>
            )}

            {capabilities.background && (
              <label className='grid gap-2 text-sm font-medium'>
                {t('背景')}
                <Select
                  aria-label={t('背景')}
                  value={background}
                  onChange={setBackground}
                  optionList={['auto', 'opaque', 'transparent'].map(
                    (value) => ({
                      label: value,
                      value,
                    }),
                  )}
                  style={{ width: '100%' }}
                  disabled={isGenerating}
                />
              </label>
            )}
          </div>

          {modelsLoading && (
            <div className='mt-4 flex items-center gap-2 text-sm text-semi-color-text-2'>
              <Spin size='small' />
              {t('正在加载模型')}
            </div>
          )}
          {!modelsLoading && modelsError && (
            <div className='mt-4 flex items-center justify-between gap-3'>
              <Typography.Text type='danger'>{modelsError}</Typography.Text>
              <Button
                icon={<RefreshCw size={14} />}
                onClick={() => void loadModels()}
                size='small'
              >
                {t('重试')}
              </Button>
            </div>
          )}
          {!modelsLoading && !modelsError && models.length === 0 && (
            <Typography.Text type='tertiary' className='mt-4 block'>
              {t('当前分组没有可用模型')}
            </Typography.Text>
          )}

          {generationError && (
            <div className='mt-4 rounded-md border border-semi-color-danger p-3'>
              <div className='flex flex-wrap items-start justify-between gap-3'>
                <div className='min-w-0'>
                  <Typography.Text type='danger' strong>
                    {t('图片生成失败')}
                  </Typography.Text>
                  <Typography.Text className='mt-1 block break-words'>
                    {generationError}
                  </Typography.Text>
                </div>
                <Button
                  icon={<RefreshCw size={14} />}
                  onClick={() => void handleGenerate()}
                  size='small'
                  disabled={isGenerating}
                >
                  {t('重试')}
                </Button>
              </div>
            </div>
          )}

          <div className='mt-4 flex justify-end gap-2'>
            {isGenerating && (
              <Button icon={<Square size={14} />} onClick={handleStop}>
                {t('停止')}
              </Button>
            )}
            <Button
              type='primary'
              theme='solid'
              icon={<ImageIcon size={15} />}
              loading={isGenerating}
              disabled={!prompt.trim() || !model || modelsLoading}
              onClick={() => void handleGenerate()}
            >
              {t('生成')}
            </Button>
          </div>
        </div>

        {images.length > 0 ? (
          <div className='mt-5 grid gap-4 sm:grid-cols-2'>
            {images.map((image, index) => (
              <div
                key={image.id}
                className='overflow-hidden rounded-md border border-semi-color-border bg-semi-color-bg-1'
              >
                {image.src ? (
                  <img
                    src={image.src}
                    alt={image.revised_prompt || prompt}
                    className='aspect-square w-full bg-semi-color-fill-0 object-contain'
                  />
                ) : (
                  <Empty
                    title={t('没有可预览的图片')}
                    className='aspect-square'
                  />
                )}
                <div className='flex items-center justify-between gap-2 p-2'>
                  <Typography.Text
                    ellipsis
                    type='tertiary'
                    size='small'
                    className='min-w-0'
                  >
                    {image.revised_prompt || prompt}
                  </Typography.Text>
                  <div className='flex shrink-0 items-center gap-1'>
                    {image.url && (
                      <Button
                        aria-label={t('复制 URL')}
                        icon={<Copy size={14} />}
                        onClick={() => void copyImageURL(image.url)}
                        size='small'
                        theme='borderless'
                      />
                    )}
                    {image.src && (
                      <Button
                        aria-label={t('下载')}
                        icon={<Download size={14} />}
                        onClick={() => downloadImage(image, index)}
                        size='small'
                        theme='borderless'
                      />
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        ) : null}
      </div>
    </div>
  );
};

export default PlaygroundImagePanel;
