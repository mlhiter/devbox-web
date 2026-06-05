import { NextRequest } from 'next/server';
import { z } from 'zod';
import { jsonRes } from '@/services/backend/response';
import { POST as createRelease } from '../[name]/release/route';
import { jsonRequest } from '../_legacy';

export const dynamic = 'force-dynamic';

const RequestSchema = z.object({
  devboxName: z.string().min(1),
  tag: z.string().min(1),
  releaseDes: z.string().optional().default('')
});

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

    const { devboxName, tag, releaseDes } = validationResult.data;
    return createRelease(jsonRequest(req, { tag, releaseDes }), {
      params: { name: devboxName }
    });
  } catch (err: any) {
    return jsonRes({
      code: 500,
      message: err?.message || 'Internal server error',
      error: err
    });
  }
}
