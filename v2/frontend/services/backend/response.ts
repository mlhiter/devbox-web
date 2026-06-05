import { NextResponse } from 'next/server';

import { V1Status } from '@kubernetes/client-node';
import { ERROR_ENUM, ERROR_RESPONSE, ERROR_TEXT } from '../error';

const getQuotaExceededMessageKey = (message = '') => {
  const normalizedMessage = message.toLowerCase();

  if (!normalizedMessage.includes('exceeded quota')) {
    return '';
  }

  if (
    normalizedMessage.includes('requests.storage') ||
    normalizedMessage.includes('persistentvolumeclaims') ||
    normalizedMessage.includes('requests.ephemeral-storage') ||
    normalizedMessage.includes('limits.ephemeral-storage')
  ) {
    return 'storage_exceeds_quota';
  }

  if (normalizedMessage.includes('limits.cpu') || normalizedMessage.includes('requests.cpu')) {
    return 'cpu_exceeds_quota';
  }

  if (normalizedMessage.includes('limits.memory') || normalizedMessage.includes('requests.memory')) {
    return 'memory_exceeds_quota';
  }

  if (
    normalizedMessage.includes('services.nodeports') ||
    normalizedMessage.includes('count/devboxes')
  ) {
    return 'nodeports_exceeds_quota';
  }

  return '';
};

const isKubernetesStatusBody = (body: any) =>
  body instanceof V1Status ||
  (!!body &&
    typeof body === 'object' &&
    (typeof body.code === 'number' ||
      typeof body.code === 'string' ||
      typeof body.reason === 'string' ||
      typeof body.message === 'string'));

const normalizeStatusCode = (status?: number, fallback = 200) => {
  if (typeof status === 'number' && Number.isInteger(status) && status >= 100 && status <= 599) {
    return status;
  }
  return fallback;
};

const toSerializableError = (error: any) => {
  const detail = error?.body ?? error;
  if (detail === undefined) return null;
  try {
    JSON.stringify(detail);
    return detail;
  } catch {
    return {
      name: error?.name || 'Error',
      message: error?.message || 'Unknown error'
    };
  }
};

export const jsonRes = <T = any>(props: {
  code?: number;
  message?: string;
  data?: T;
  error?: any;
}) => {
  const { code = 200, message = '', data = null, error } = props || {};

  if (typeof error === 'string' && ERROR_RESPONSE[error]) {
    const mappedError = ERROR_RESPONSE[error];
    const status = normalizeStatusCode(mappedError.code, normalizeStatusCode(code, 500));
    return NextResponse.json(mappedError, { status });
  }
  const body = error?.body;
  if (isKubernetesStatusBody(body)) {
    const bodyMessage = typeof body.message === 'string' ? body.message : '';
    const quotaExceededMessageKey = getQuotaExceededMessageKey(bodyMessage);

    if (bodyMessage.includes('40001:')) {
      const mappedError = ERROR_RESPONSE[ERROR_ENUM.outstandingPayment];
      return NextResponse.json(mappedError, {
        status: normalizeStatusCode(mappedError.code, 402)
      });
    } else if (quotaExceededMessageKey) {
      const status = normalizeStatusCode(Number(body.code) || code, 403);
      return NextResponse.json(
        {
          code: status,
          statusText: bodyMessage || '',
          message: quotaExceededMessageKey,
          data: body
        },
        { status }
      );
    } else if (body.code === 403 || body.reason === 'Forbidden') {
      const mappedError = ERROR_RESPONSE[ERROR_ENUM.insufficientPermissions];
      return NextResponse.json(mappedError, {
        status: normalizeStatusCode(mappedError.code, 403)
      });
    } else {
      const status = normalizeStatusCode(body.code || code, 500);
      return NextResponse.json(
        {
          code: status,
          statusText: body.reason || '',
          message: body.message || 'Kubernetes error',
          data: body
        },
        { status }
      );
    }
  }

  if (error?.statusCode && error.statusCode >= 400) {
    const quotaExceededMessageKey = getQuotaExceededMessageKey(error.message || '');
    if (quotaExceededMessageKey) {
      const status = normalizeStatusCode(error.statusCode, 500);
      return NextResponse.json(
        {
          code: status,
          statusText: error.message || '',
          message: quotaExceededMessageKey,
          data: toSerializableError(error)
        },
        { status }
      );
    }

    if (error.statusCode === 403) {
      const mappedError = ERROR_RESPONSE[ERROR_ENUM.insufficientPermissions];
      return NextResponse.json(mappedError, {
        status: normalizeStatusCode(mappedError.code, 403)
      });
    }
    const status = normalizeStatusCode(error.statusCode, 500);
    return NextResponse.json(
      {
        code: status,
        statusText: error.message || '',
        message: error.message || `HTTP error ${status}`,
        data: toSerializableError(error)
      },
      { status }
    );
  }

  let msg = message;
  if ((code < 200 || code >= 400) && !message) {
    msg = error?.body?.message || error?.message || 'request error';
    if (typeof error === 'string') {
      msg = error;
    } else if (error?.code && error.code in ERROR_TEXT) {
      msg = ERROR_TEXT[error.code];
    }
    console.log('===jsonRes===\n', error);
  }

  const status = normalizeStatusCode(code, 200);
  const responseBody: {
    code: number;
    statusText: string;
    message: string;
    data?: any;
  } = {
    code: status,
    statusText: '',
    message: msg
  };

  if (status >= 400) {
    responseBody.data = data ?? toSerializableError(error);
  } else {
    responseBody.data = data || error || null;
  }

  return NextResponse.json(responseBody, { status });
};
