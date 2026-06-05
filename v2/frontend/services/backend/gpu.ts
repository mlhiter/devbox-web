import { CoreV1Api } from '@kubernetes/client-node';

import { K8sApiDefault } from '@/services/backend/kubernetes';
import type { GpuAliasMap } from '@/types/gpu';

const GPU_CONFIGMAP_NAME = 'node-gpu-info';
const GPU_CONFIGMAP_NAMESPACE = 'node-system';

export async function getGpuAliasMap(): Promise<GpuAliasMap> {
  try {
    const kc = K8sApiDefault();
    const api = kc.makeApiClient(CoreV1Api);

    const { body } = await api.listNamespacedConfigMap(
      GPU_CONFIGMAP_NAMESPACE,
      undefined,
      undefined,
      undefined,
      `metadata.name=${GPU_CONFIGMAP_NAME}`
    );

    if (!body.items || body.items.length === 0) return {};
    const configMap = body.items[0];
    const aliasRaw = configMap.data?.alias;
    if (!aliasRaw) return {};

    const parsed = JSON.parse(aliasRaw) as GpuAliasMap;
    if (!parsed || typeof parsed !== 'object') return {};

    try {
      const gpuRaw = configMap.data?.gpu;
      if (gpuRaw) {
        const aliasBackupRaw = configMap.data?.['alias-backup'];
        const aliasBackup = aliasBackupRaw
          ? (JSON.parse(aliasBackupRaw) as Record<string, string>)
          : undefined;
        const gpuMap = JSON.parse(gpuRaw) as Record<
          string,
          {
            'gpu.product'?: string;
            'gpu.ref'?: string;
          }
        >;

        Object.values(gpuMap).forEach((item) => {
          const product = item['gpu.product'];
          if (!product) return;

          if (parsed[product]) {
            parsed[product] = {
              ...parsed[product],
              product
            };
            return;
          }

          const fallbackRef =
            aliasBackup?.[product] || aliasBackup?.[product.replace(/\s+/g, '-')];
          const ref = item['gpu.ref'] || fallbackRef;
          const aliasItem = (ref && parsed[ref]) || parsed[product];
          if (aliasItem) {
            if (ref && parsed[ref]) {
              parsed[ref] = {
                ...parsed[ref],
                product
              };
            }

            parsed[product] = {
              ...aliasItem,
              product
            };
          }
        });
      }
    } catch (error) {
      console.log('augmentGpuAliasMap error', error);
    }

    return parsed;
  } catch (error) {
    console.log('getGpuAliasMap error', error);
    return {};
  }
}
