type WorkspaceQuotaResponse = {
  quota: Array<{
    type: string;
    used: number;
    limit: number;
  }>;
};

type WorkspaceQuotaSealosApp = {
  getWorkspaceQuota: () => Promise<WorkspaceQuotaResponse>;
};

let workspaceQuotaBridgeSupported: boolean | undefined;

export const isWorkspaceQuotaBridgeUnsupported = (error: unknown) => {
  const message =
    typeof error === 'string'
      ? error
      : error && typeof error === 'object' && 'message' in error
        ? String((error as { message?: unknown }).message || '')
        : '';

  return message === 'function is not declare';
};

export const createQuotaCompatibleSealosApp = <T extends WorkspaceQuotaSealosApp>(
  sealosApp: T
): T => {
  return new Proxy(sealosApp, {
    get(target, property, receiver) {
      if (property === 'getWorkspaceQuota') {
        return async () => {
          if (workspaceQuotaBridgeSupported === false) {
            return { quota: [] };
          }

          try {
            const response = await target.getWorkspaceQuota();
            workspaceQuotaBridgeSupported = true;
            return response;
          } catch (error) {
            if (!isWorkspaceQuotaBridgeUnsupported(error)) {
              throw error;
            }

            workspaceQuotaBridgeSupported = false;
            console.info('devbox: workspace quota bridge is unavailable, skipping quota guard');
            return { quota: [] };
          }
        };
      }

      const value = Reflect.get(target, property, receiver);
      if (typeof value === 'function') {
        return value.bind(target);
      }
      return value;
    }
  });
};
