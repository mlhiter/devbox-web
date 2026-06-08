import { NextRequest } from 'next/server';
import { jsonRes } from '@/services/backend/response';
import { GET as getDevboxList } from '../../../getDevboxList/route';

export const dynamic = 'force-dynamic';

export async function GET(req: NextRequest) {
  const response = await getDevboxList(req);
  const payload = await response.json();

  if (response.status >= 400) {
    return jsonRes({
      code: payload?.code || response.status,
      message: payload?.message,
      data: payload?.data
    });
  }

  const data = Array.isArray(payload?.data) ? payload.data : [];
  return jsonRes({
    data: data.map((item: any) => ({
      id: item.id || item.uid || '',
      name: item.name || '',
      createTime: item.createTime || ''
    }))
  });
}
