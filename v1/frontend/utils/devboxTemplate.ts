import { z } from 'zod';

import { TemplateRepositoryKind } from '@/prisma/generated/client';

const uuidSchema = z.string().uuid();

export const CUSTOM_RUNTIME_ICON_ID = 'custom';
export const EXTERNAL_DEVBOX_UNMANAGED = 'EXTERNAL_DEVBOX_UNMANAGED';

export const DEFAULT_EXTERNAL_DEVBOX_USER = 'devbox';
export const DEFAULT_EXTERNAL_DEVBOX_WORKING_DIR = '/home/devbox/project';
export const DEFAULT_EXTERNAL_DEVBOX_RELEASE_COMMAND = ['/bin/bash', '-c'];
export const DEFAULT_EXTERNAL_DEVBOX_RELEASE_ARGS = ['/home/devbox/project/entrypoint.sh'];

export const isValidTemplateID = (value: unknown): value is string => {
  return typeof value === 'string' && uuidSchema.safeParse(value).success;
};

export const collectValidTemplateIDs = (values: unknown[]): string[] => {
  const seen = new Set<string>();

  return values.filter((value): value is string => {
    if (!isValidTemplateID(value) || seen.has(value)) {
      return false;
    }
    seen.add(value);
    return true;
  });
};

export const buildFallbackTemplateSummary = (templateID: unknown) => ({
  uid: typeof templateID === 'string' && templateID.length > 0 ? templateID : 'external',
  name: CUSTOM_RUNTIME_ICON_ID,
  templateRepository: {
    iconId: CUSTOM_RUNTIME_ICON_ID
  }
});

export const buildFallbackTemplateDetail = (
  templateID: unknown,
  image = '',
  config: unknown = {}
) => ({
  templateRepository: {
    uid: 'external',
    iconId: CUSTOM_RUNTIME_ICON_ID,
    name: CUSTOM_RUNTIME_ICON_ID,
    kind: TemplateRepositoryKind.CUSTOM,
    description: 'External Devbox'
  },
  uid: typeof templateID === 'string' && templateID.length > 0 ? templateID : 'external',
  image,
  name: CUSTOM_RUNTIME_ICON_ID,
  config: JSON.stringify(config || {})
});

export const buildExternalTemplateConfig = () => ({
  user: DEFAULT_EXTERNAL_DEVBOX_USER,
  workingDir: DEFAULT_EXTERNAL_DEVBOX_WORKING_DIR,
  releaseCommand: DEFAULT_EXTERNAL_DEVBOX_RELEASE_COMMAND,
  releaseArgs: DEFAULT_EXTERNAL_DEVBOX_RELEASE_ARGS
});

export const buildExternalDevboxUnmanagedError = (devboxName: string, templateID: unknown) => ({
  code: EXTERNAL_DEVBOX_UNMANAGED,
  reason: 'templateID_missing_or_invalid',
  devboxName,
  templateID: typeof templateID === 'string' ? templateID : ''
});

export const buildExternalDevboxUnmanagedResponse = (devboxName: string, templateID: unknown) => ({
  code: 422,
  message:
    'This Devbox was created externally and does not provide a valid templateID. Template-dependent operations are unavailable.',
  error: buildExternalDevboxUnmanagedError(devboxName, templateID)
});
