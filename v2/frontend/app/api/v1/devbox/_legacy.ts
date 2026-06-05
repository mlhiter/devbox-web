import { NextRequest } from 'next/server';
import { jsonRes } from '@/services/backend/response';

const jsonHeaders = (headers: Headers) => {
  const nextHeaders = new Headers(headers);
  nextHeaders.delete('content-length');
  nextHeaders.set('content-type', 'application/json');
  return nextHeaders;
};

export const emptyJsonRequest = (req: NextRequest) =>
  new NextRequest(req.url, {
    method: 'POST',
    headers: jsonHeaders(req.headers),
    body: '{}'
  });

export const jsonRequest = (req: NextRequest, body: unknown, method = 'POST') =>
  new NextRequest(req.url, {
    method,
    headers: jsonHeaders(req.headers),
    body: JSON.stringify(body)
  });

export const getRequiredSearchParam = (req: NextRequest, key: string) => {
  const value = req.nextUrl.searchParams.get(key);
  if (!value) {
    return {
      error: jsonRes({
        code: 400,
        message: `${key} is required`
      }),
      value: ''
    };
  }

  return { value };
};

export const normalizeLegacyPort = (port: any) => {
  const parsedPort = Number(port);
  return Number.isFinite(parsedPort) ? parsedPort : port;
};
