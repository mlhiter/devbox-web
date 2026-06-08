import { NextRequest } from 'next/server';
import { DELETE as deleteDevbox } from '../[name]/delete/route';
import { getRequiredSearchParam } from '../_legacy';

export const dynamic = 'force-dynamic';

export async function DELETE(req: NextRequest) {
  const { value: devboxName, error } = getRequiredSearchParam(req, 'devboxName');
  if (error) return error;

  return deleteDevbox(req, { params: { name: devboxName } });
}
