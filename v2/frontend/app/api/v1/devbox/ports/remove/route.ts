import { NextRequest } from 'next/server';
import { z } from 'zod';
import { jsonRes } from '@/services/backend/response';
import { GET as getDevbox } from '../../[name]/route';
import { PUT as updatePorts } from '../../[name]/ports/route';
import { jsonRequest, normalizeLegacyPort } from '../../_legacy';

export const dynamic = 'force-dynamic';

const RequestSchema = z.object({
  devboxName: z.string().min(1),
  port: z.number().min(1).max(65535)
});

const normalizeResponse = async (response: Response) => {
  const payload = await response.json();
  const ports = payload?.data?.ports;
  if (!Array.isArray(ports)) {
    return jsonRes(payload);
  }

  return jsonRes({
    code: payload.code,
    message: payload.message,
    data: ports.map((item: any) => ({
      ...item,
      port: normalizeLegacyPort(item.number)
    }))
  });
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

    const { devboxName, port } = validationResult.data;
    const detailResponse = await getDevbox(req, { params: { name: devboxName } });
    const detailPayload = await detailResponse.json();

    if (detailResponse.status >= 400) {
      return jsonRes({
        code: detailPayload?.code || detailResponse.status,
        message: detailPayload?.message || 'Failed to get existing ports',
        data: detailPayload?.data
      });
    }

    const existingPorts = Array.isArray(detailPayload?.data?.ports) ? detailPayload.data.ports : [];
    const targetPort = existingPorts.find((item: any) => item.number === port);

    if (!targetPort) {
      return jsonRes({
        code: 404,
        message: 'Port not found in service'
      });
    }

    const response = await updatePorts(
      jsonRequest(
        req,
        {
          ports: existingPorts
            .filter((item: any) => item.number !== port)
            .filter((item: any) => item?.portName)
            .map((item: any) => ({
              portName: item.portName
            }))
        },
        'PUT'
      ),
      { params: { name: devboxName } }
    );

    return normalizeResponse(response);
  } catch (err: any) {
    return jsonRes({
      code: 500,
      message: err?.message || 'Internal server error',
      error: err
    });
  }
}
