import { NextRequest } from 'next/server';
import { GET as listReleases } from '../[name]/release/route';
import { getRequiredSearchParam } from '../_legacy';

export const dynamic = 'force-dynamic';

export async function GET(req: NextRequest) {
  const { value: devboxName, error } = getRequiredSearchParam(req, 'devboxName');
  if (error) return error;

  return listReleases(req, { params: { name: devboxName } });
}
