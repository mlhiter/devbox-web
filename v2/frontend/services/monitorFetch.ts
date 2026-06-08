import { MetricsClient } from '@labring/sealos-metrics-sdk';
import type { LaunchpadQueryParams } from '@labring/sealos-metrics-sdk';
import { KubeConfig } from '@kubernetes/client-node';

type KubeConfigWithLegacyHttpsOptions = KubeConfig & {
  applytoHTTPSOptions?: KubeConfig['applyToHTTPSOptions'];
};

const ensureMetricsSdkKubeConfigCompatibility = () => {
  const proto = KubeConfig.prototype as KubeConfigWithLegacyHttpsOptions;

  if (!proto.applytoHTTPSOptions) {
    proto.applytoHTTPSOptions = proto.applyToHTTPSOptions;
  }
};

export const monitorFetch = async (params: LaunchpadQueryParams, kubeconfig: string) => {
  ensureMetricsSdkKubeConfigCompatibility();

  const metricsURL = process.env.METRICS_URL;
  const client = new MetricsClient({
    kubeconfig,
    metricsURL
  });

  return client.launchpad.query(params);
};
