import { NextRequest } from 'next/server';
import { z } from 'zod';
import { jsonRes } from '@/services/backend/response';
import { POST as pauseDevbox } from '../[name]/pause/route';
import { POST as restartDevbox } from '../[name]/restart/route';
import { POST as shutdownDevbox } from '../[name]/shutdown/route';
import { POST as startDevbox } from '../[name]/start/route';
import { emptyJsonRequest } from '../_legacy';

export const dynamic = 'force-dynamic';

const RequestSchema = z.object({
  devboxName: z.string().min(1),
  action: z.enum(['start', 'stop', 'restart', 'shutdown'])
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

    const { devboxName, action } = validationResult.data;
    const nextReq = emptyJsonRequest(req);

    if (action === 'start') {
      return startDevbox(nextReq, { params: { name: devboxName } });
    }

    if (action === 'restart') {
      return restartDevbox(nextReq, { params: { name: devboxName } });
    }

    if (action === 'shutdown') {
      return shutdownDevbox(nextReq, { params: { name: devboxName } });
    }

    return pauseDevbox(nextReq, { params: { name: devboxName } });
  } catch (err: any) {
    return jsonRes({
      code: 500,
      message: err?.message || 'Internal server error',
      error: err
    });
  }
}
