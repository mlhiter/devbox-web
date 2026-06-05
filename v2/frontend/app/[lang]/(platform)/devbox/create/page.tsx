'use client';

import { toast } from 'sonner';
import { debounce } from 'lodash';
import { useTranslations } from 'next-intl';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'next/navigation';

import { FormProvider, useForm } from 'react-hook-form';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { useRouter } from '@/i18n';
import type { YamlItemType } from '@/types';
import { patchYamlList } from '@/utils/tools';
import { useConfirm } from '@/hooks/useConfirm';
import { generateYamlList } from '@/utils/json2Yaml';
import { createDevbox, updateDevbox } from '@/api/devbox';
import type { DevboxEditTypeV2, DevboxKindsType, DevboxPatchPropsType } from '@/types/devbox';
import {
  defaultDevboxEditValueV2,
  editModeMap,
  GPU_AMOUNT_MAX,
  gpuNodeSelectorKey,
  gpuTypeAnnotationKey,
  YamlKindEnum
} from '@/constants/devbox';

import { useEnvStore } from '@/stores/env';
import { useIDEStore } from '@/stores/ide';
import { usePriceStore } from '@/stores/price';
import { useGuideStore } from '@/stores/guide';
import { useDevboxStore } from '@/stores/devbox';
import { useQuotaGuarded } from '@labring/sealos-shared-sdk';
import { useDevboxOperation } from '@/hooks/useDevboxOperation';
import ErrorModal from '@/components/ErrorModal';

import Form from './components/Form';
import Yaml from './components/Yaml';
import Header from './components/Header';
import { Loading } from '@labring/sealos-ui/loading';
import { track } from '@labring/sealos-gtm-sdk';
import { listTemplate } from '@/api/template';
import { z } from 'zod';

const omitMergeBaseImageTopLayer = (formData: DevboxEditTypeV2): DevboxEditTypeV2 => {
  const editableFormData = { ...formData };
  delete editableFormData.mergeBaseImageTopLayer;

  return editableFormData;
};

const isPlainObject = (value: unknown): value is Record<string, any> =>
  !!value && typeof value === 'object' && !Array.isArray(value);

const mergeJsonMergePatch = (target: Record<string, any>, source: Record<string, any>) => {
  Object.entries(source).forEach(([key, value]) => {
    if (isPlainObject(value) && isPlainObject(target[key])) {
      mergeJsonMergePatch(target[key], value);
      return;
    }

    target[key] = value;
  });

  return target;
};

const DevboxCreatePage = () => {
  const router = useRouter();
  const t = useTranslations();
  const searchParams = useSearchParams();
  const { executeOperation, errorModalState, closeErrorModal } = useDevboxOperation();

  const { env } = useEnvStore();
  const { addDevboxIDE } = useIDEStore();
  const { setDevboxDetail, setStartedTemplate, startedTemplate } = useDevboxStore();
  const { sourcePrice, setSourcePrice } = usePriceStore();

  const crOldYamls = useRef<DevboxKindsType[]>([]);
  const formOldYamls = useRef<YamlItemType[]>([]);
  const oldDevboxEditData = useRef<DevboxEditTypeV2>();

  const [isLoading, setIsLoading] = useState(false);
  const [yamlList, setYamlList] = useState<YamlItemType[]>([]);

  const tabType = searchParams.get('type') || 'form';
  const devboxName = searchParams.get('name') || '';

  // NOTE: need to explain why this is needed
  // fix a bug: searchParams will disappear when go into this page
  const [captureFrom, setCaptureFrom] = useState('');
  const [captureScrollTo, setCaptureScrollTo] = useState('');
  const [captureDevboxName, setCaptureDevboxName] = useState('');
  const formHook = useForm<DevboxEditTypeV2>({
    defaultValues: defaultDevboxEditValueV2
  });

  useEffect(() => {
    const name = searchParams.get('name');
    const from = searchParams.get('from');
    const scrollTo = searchParams.get('scrollTo');
    if (name && name !== captureDevboxName) {
      setCaptureDevboxName(name);
      if (from) {
        setCaptureFrom(from);
      }
      if (scrollTo) {
        setCaptureScrollTo(scrollTo);
      }
    } else if (from === 'template') {
      const savedFormData = localStorage.getItem('devbox_create_form_data');
      if (savedFormData) {
        try {
          const formData = JSON.parse(savedFormData);
          formHook.reset({
            ...defaultDevboxEditValueV2,
            ...formData
          });
          localStorage.removeItem('devbox_create_form_data');
        } catch (error) {
          console.error('Failed to parse saved form data:', error);
        }
      }
    }
  }, [searchParams, formHook]);

  // eslint-disable-next-line react-hooks/exhaustive-deps
  const isEdit = useMemo(() => !!devboxName, []);
  const maxGpuAmount = GPU_AMOUNT_MAX;

  const { title, applyBtnText, applyMessage, applySuccess, applyError } = editModeMap(isEdit);

  const { openConfirm, ConfirmChild } = useConfirm({
    content: applyMessage
  });

  const templateRepositoryUid = formHook.watch('templateRepositoryUid');
  const isValidTemplateRepositoryUid = z.string().uuid().safeParse(templateRepositoryUid).success;

  const templateListQuery = useQuery(
    ['templateList', templateRepositoryUid],
    () => listTemplate(templateRepositoryUid),
    {
      enabled: isValidTemplateRepositoryUid
    }
  );
  const templateList = useMemo(
    () => templateListQuery.data?.templateList || [],
    [templateListQuery.data?.templateList]
  );

  const generateDefaultYamlList = () => generateYamlList(defaultDevboxEditValueV2, env);

  // update yamlList every time yamlList change
  const debouncedUpdateYaml = useMemo(
    () =>
      debounce((data: DevboxEditTypeV2, env) => {
        try {
          const yamlFormData = isEdit ? omitMergeBaseImageTopLayer(data) : data;
          const newYamlList = generateYamlList(yamlFormData, env);
          setYamlList(newYamlList);
        } catch (error) {
          console.error('Failed to generate yaml:', error);
        }
      }, 300),
    [isEdit]
  );

  const countGpuInventory = useCallback(
    (type?: string, product?: string) => {
      if (!type) return 0;
      const gpuItems =
        sourcePrice?.gpu?.filter((item) =>
          env.gpuSchedulerMode === 'native'
            ? item.annotationType === type && item.product === product
            : item.annotationType === type
        ) || [];
      const available = gpuItems.reduce((sum, item) => sum + (item.available || 0), 0);
      const total = gpuItems.reduce((sum, item) => sum + (item.count || 0), 0);

      if (!isEdit) {
        return available;
      }

      const originalGpu = oldDevboxEditData.current?.gpu;
      if (
        !originalGpu ||
        originalGpu.type !== type ||
        (env.gpuSchedulerMode === 'native' && originalGpu.product !== product)
      ) {
        return available;
      }

      return Math.min(available + (originalGpu.amount || 0), total);
    },
    [env.gpuSchedulerMode, isEdit, sourcePrice?.gpu]
  );

  useEffect(() => {
    const subscription = formHook.watch((value) => {
      if (value) {
        debouncedUpdateYaml(value as DevboxEditTypeV2, env);
      }
    });
    return () => {
      subscription.unsubscribe();
      debouncedUpdateYaml.cancel();
    };
  }, [debouncedUpdateYaml, env, formHook]);

  const { refetch: refetchPrice } = useQuery(['init-price'], setSourcePrice, {
    enabled: !!sourcePrice?.gpu,
    refetchInterval: 6000
  });

  useQuery(
    ['initDevboxCreateData'],
    () => {
      if (!devboxName) {
        setYamlList(generateDefaultYamlList());
        return null;
      }
      setIsLoading(true);
      return setDevboxDetail(devboxName, env.sealosDomain);
    },
    {
      onSuccess(res) {
        if (!res) {
          return;
        }
        oldDevboxEditData.current = res;
        const editableDevboxData = omitMergeBaseImageTopLayer(res);
        formOldYamls.current = generateYamlList(editableDevboxData, env);
        crOldYamls.current = generateYamlList(editableDevboxData, env) as DevboxKindsType[];
        formHook.reset(res);
      },
      onError(err) {
        toast.error(String(err));
      },
      onSettled() {
        setIsLoading(false);
      }
    }
  );
  const { guideConfigDevbox } = useGuideStore();

  const buildGpuSchedulerCleanupPatchValue = useCallback(
    (formData: DevboxEditTypeV2): Record<string, any> | undefined => {
      const hasCurrentGpu = !!formData.gpu?.type;
      const hadGpu = !!oldDevboxEditData.current?.gpu;

      if (!hasCurrentGpu && !hadGpu) {
        return undefined;
      }

      const spec: Record<string, any> = {};
      if (hasCurrentGpu && env.gpuSchedulerMode === 'native') {
        spec.nodeSelector = {
          [gpuNodeSelectorKey]: formData.gpu?.product
        };
      } else if (!hasCurrentGpu || env.gpuSchedulerMode === 'hami') {
        spec.nodeSelector = {
          [gpuNodeSelectorKey]: null
        };
      }

      if (hasCurrentGpu && env.gpuSchedulerMode === 'hami') {
        spec.config = {
          annotations: {
            [gpuTypeAnnotationKey]: formData.gpu?.type
          }
        };
      } else if (!hasCurrentGpu || env.gpuSchedulerMode === 'native') {
        spec.config = {
          annotations: {
            [gpuTypeAnnotationKey]: null
          }
        };
      }

      return Object.keys(spec).length > 0
        ? {
            metadata: {
              name: formData.name
            },
            spec
          }
        : undefined;
    },
    [env.gpuSchedulerMode]
  );

  const submitSuccess = async (formData: DevboxEditTypeV2) => {
    if (!guideConfigDevbox) {
      return router.push('/devbox/detail/devbox-mock');
    }

    // gpu inventory check
    if (formData.gpu?.type) {
      if (env.gpuSchedulerMode === 'native' && !formData.gpu.product) {
        return toast.warning(t('submit_form_error'));
      }

      if (formData.gpu.amount > maxGpuAmount) {
        return toast.warning(t('Gpu amount over max Tip', { max: maxGpuAmount }));
      }

      const inventory = countGpuInventory(formData.gpu.type, formData.gpu.product);
      if (formData.gpu?.amount > inventory) {
        return toast.warning(
          t('Gpu under inventory Tip', {
            gputype: formData.gpu.type
          })
        );
      }
    }

    // update
    if (isEdit) {
      const yamlList = generateYamlList(omitMergeBaseImageTopLayer(formData), env);
      setYamlList(yamlList);
      const parsedNewYamlList = yamlList.map((item) => item.value);
      const parsedOldYamlList = formOldYamls.current.map((item) => item.value);
      const areYamlListsEqual =
        new Set(parsedNewYamlList).size === new Set(parsedOldYamlList).size &&
        [...new Set(parsedNewYamlList)].every((item) => new Set(parsedOldYamlList).has(item));
      if (!parsedNewYamlList) {
        return toast.warning(t('submit_form_error'));
      }
      const cleanupPatchValue = buildGpuSchedulerCleanupPatchValue(formData);
      if (areYamlListsEqual && !cleanupPatchValue) {
        return toast.info(t('No changes detected'));
      }
      const patch = patchYamlList({
        parsedOldYamlList: parsedOldYamlList,
        parsedNewYamlList: parsedNewYamlList,
        originalYamlList: crOldYamls.current
      });
      if (cleanupPatchValue) {
        const devboxPatch = patch.find(
          (item): item is Extract<DevboxPatchPropsType[number], { type: 'patch' }> =>
            item.type === 'patch' && item.kind === YamlKindEnum.Devbox
        );

        if (devboxPatch) {
          mergeJsonMergePatch(devboxPatch.value, cleanupPatchValue);
        } else {
          patch.push({
            type: 'patch',
            kind: YamlKindEnum.Devbox,
            value: cleanupPatchValue
          });
        }
      }
      await executeOperation(
        () =>
          updateDevbox({
            patch,
            devboxName: formData.name
          }),
        {
          onSuccess: () => {
            track({
              event: 'deployment_update',
              module: 'devbox',
              context: 'app'
            });
            addDevboxIDE('vscode', formData.name);
            if (sourcePrice?.gpu) {
              refetchPrice();
            }
            setStartedTemplate(undefined);
            router.push(`/devbox/detail/${formData.name}`);
          },
          successMessage: t(applySuccess)
        }
      );
    } else {
      await executeOperation(() => createDevbox(formData), {
        onSuccess: () => {
          track({
            event: 'deployment_create',
            module: 'devbox',
            context: 'app',
            config: {
              template_name: startedTemplate?.name || '',
              template_version: templateList.find((t) => t.uid === formData.templateUid)?.name || ''
            },
            resources: {
              cpu_cores: formData.cpu,
              ram_mb: formData.memory
            }
          });
          addDevboxIDE('vscode', formData.name);
          if (sourcePrice?.gpu) {
            refetchPrice();
          }
          setStartedTemplate(undefined);
          router.push(`/devbox/detail/${formData.name}`);
        },
        successMessage: t(applySuccess)
      });
    }
  };

  const submitError = useCallback(() => {
    // deep search message
    const deepSearch = (obj: any): string => {
      if (!obj || typeof obj !== 'object') {
        return t('submit_form_error');
      }
      if (!!obj.message) {
        return obj.message;
      }
      return deepSearch(Object.values(obj)[0]);
    };
    toast.error(deepSearch(formHook.formState.errors));
  }, [formHook.formState.errors, t]);

  const formData = formHook.watch();
  const quotaRequirements = useMemo(
    () => ({
      cpu: isEdit ? formData.cpu - (oldDevboxEditData.current?.cpu ?? 0) : formData.cpu,
      memory: isEdit ? formData.memory - (oldDevboxEditData.current?.memory ?? 0) : formData.memory,
      gpu: 0,
      nodeport: 0,
      traffic: true
    }),
    [formData, isEdit]
  );

  const handleApply = useQuotaGuarded(
    {
      requirements: quotaRequirements,
      immediate: false,
      allowContinue: false
    },
    () => {
      formHook.handleSubmit((data) => openConfirm(() => submitSuccess(data))(), submitError)();
    }
  );

  if (isLoading) return <Loading />;

  return (
    <>
      <FormProvider {...formHook}>
        <div className="flex h-[calc(100vh-28px)] min-w-[1024px] flex-col items-center">
          <Header
            yamlList={yamlList}
            title={title}
            applyBtnText={applyBtnText}
            name={captureDevboxName}
            from={captureFrom as 'list' | 'detail'}
            applyCb={handleApply}
          />
          <div className="w-full px-5 pt-10 pb-30 md:px-10 lg:px-20">
            {tabType === 'form' ? (
              <Form
                isEdit={isEdit}
                oldDevboxData={oldDevboxEditData.current ?? null}
                countGpuInventory={countGpuInventory}
              />
            ) : (
              <Yaml yamlList={yamlList} />
            )}
          </div>
        </div>
      </FormProvider>
      <ConfirmChild />
      <ErrorModal
        isOpen={errorModalState.isOpen}
        onClose={closeErrorModal}
        errorCode={errorModalState.errorCode}
        errorMessage={errorModalState.errorMessage}
      />
    </>
  );
};

export default DevboxCreatePage;
