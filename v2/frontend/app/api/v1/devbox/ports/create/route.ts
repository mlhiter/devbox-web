import { NextRequest } from 'next/server';
import { z } from 'zod';
import { jsonRes } from '@/services/backend/response';
import { GET as getDevbox } from '../../[name]/route';
import { PUT as updatePorts } from '../../[name]/ports/route';
import { jsonRequest, normalizeLegacyPort } from '../../_legacy';

export const dynamic = 'force-dynamic';

const RequestSchema = z.object({
  devboxName: z.string().min(1),
  port: z.number().min(1).max(65535),
  protocol: z.enum(['HTTP', 'GRPC', 'WS']).optional().default('HTTP')
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

const getExistingPortConfigs = async (req: NextRequest, devboxName: string) => {
  const response = await getDevbox(req, { params: { name: devboxName } });
  const payload = await response.json();

  if (response.status >= 400) {
    return {
      error: jsonRes({
        code: payload?.code || response.status,
        message: payload?.message || 'Failed to get existing ports',
        data: payload?.data
      }),
      ports: []
    };
  }

  const ports = Array.isArray(payload?.data?.ports) ? payload.data.ports : [];
  return {
    ports: ports
      .filter((item: any) => item?.portName)
      .map((item: any) => ({
        portName: item.portName
      }))
  };
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

    const { devboxName, port, protocol } = validationResult.data;
    const existingPortConfigs = await getExistingPortConfigs(req, devboxName);
    if (existingPortConfigs.error) {
      return existingPortConfigs.error;
    }

    const response = await updatePorts(
      jsonRequest(
        req,
        {
          ports: [
            ...existingPortConfigs.ports,
            {
              number: port,
              protocol,
              exposesPublicDomain: true
            }
          ]
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
