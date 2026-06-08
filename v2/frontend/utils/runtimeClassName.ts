import { infoLog } from '@labring/sealos-desktop-sdk';

import type { KBDevboxTypeV2 } from '@/types/k8s';

export const preserveExistingRuntimeClassName = async ({
  jsonPatch,
  k8sCustomObjects,
  namespace
}: {
  jsonPatch: Record<string, any>;
  k8sCustomObjects: any;
  namespace: string;
}) => {
  const devboxName = jsonPatch?.metadata?.name;
  if (
    !devboxName ||
    !jsonPatch?.spec ||
    !Object.prototype.hasOwnProperty.call(jsonPatch.spec, 'runtimeClassName')
  ) {
    return jsonPatch;
  }

  try {
    const { body: devboxBody } = (await k8sCustomObjects.getNamespacedCustomObject(
      'devbox.sealos.io',
      'v1alpha2',
      namespace,
      'devboxes',
      devboxName
    )) as { body: KBDevboxTypeV2 };

    const existingRuntimeClassName = devboxBody?.spec?.runtimeClassName;

    if (existingRuntimeClassName === undefined) {
      const { runtimeClassName: _runtimeClassName, ...restSpec } = jsonPatch.spec;
      return {
        ...jsonPatch,
        spec: restSpec
      };
    }

    return {
      ...jsonPatch,
      spec: {
        ...jsonPatch.spec,
        runtimeClassName: existingRuntimeClassName
      }
    };
  } catch (error: any) {
    infoLog('preserve runtimeClassName failed, fallback to patch payload', {
      name: devboxName,
      error: error?.message
    });
    return jsonPatch;
  }
};
