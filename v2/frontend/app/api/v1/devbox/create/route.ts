import { NextRequest } from 'next/server';
import { z } from 'zod';
import { jsonRes } from '@/services/backend/response';
import { POST as createDevbox } from '../route';
import { jsonRequest } from '../_legacy';

export const dynamic = 'force-dynamic';

const RequestSchema = z.object({
  name: z.string().min(1),
  runtimeName: z.string().min(1),
  cpu: z.number().min(0).default(2000),
  memory: z.number().min(0).default(4096),
  storage: z.number().min(1).optional()
});

const legacyRuntimeNameMap: Record<string, string> = {
  debian: 'debian-ssh',
  'c++': 'cpp',
  rust: 'rust',
  java: 'java',
  go: 'go',
  python: 'python',
  'node.js': 'node.js',
  '.net': 'net',
  c: 'c',
  php: 'php'
};

const normalizeLegacyRuntime = (runtimeName: string) =>
  legacyRuntimeNameMap[runtimeName.trim().toLowerCase()] || runtimeName;

const toStorageLimit = (storage?: number) => {
  if (!storage) return undefined;
  const normalized = Math.min(50, Math.max(10, Math.ceil(storage / 10) * 10));
  return `${normalized}Gi`;
};

export async function POST(req: NextRequest) {
  try {
    const body = await req.json();
    const validationResult = RequestSchema.safeParse(body);

    if (!validationResult.success) {
      return jsonRes({
        code: 400,
        message: 'Invalid request body',
        error: validationResult.error.errors
      });
    }

    const { runtimeName, cpu, memory, storage, ...rest } = validationResult.data;
    return createDevbox(
      jsonRequest(req, {
        ...rest,
        runtime: normalizeLegacyRuntime(runtimeName),
        isRuntimeName: false,
        resource: {
          cpu: cpu / 1000,
          memory: memory / 1024
        },
        storageLimit: toStorageLimit(storage),
        ports: []
      })
    );
  } catch (err: any) {
    return jsonRes({
      code: 500,
      message: err?.message || 'Internal server error',
      error: err
    });
  }
}
