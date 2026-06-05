import { NextRequest } from 'next/server';
import { jsonRes } from '@/services/backend/response';
import { GET as getDevbox } from '../[name]/route';
import { getRequiredSearchParam } from '../_legacy';

export const dynamic = 'force-dynamic';

const normalizeNetwork = (port: any) => ({
  portName: port.portName,
  port: port.number,
  protocol: port.protocol,
  networkName: port.networkName,
  openPublicDomain: !!port.publicHost || !!port.publicAddress,
  publicDomain: port.publicHost,
  customDomain: port.customDomain
});

export async function GET(req: NextRequest) {
  const { value: devboxName, error } = getRequiredSearchParam(req, 'devboxName');
  if (error) return error;

  const response = await getDevbox(req, { params: { name: devboxName } });
  const payload = await response.json();

  if (response.status >= 400) {
    return jsonRes({
      code: payload?.code || response.status,
      message: payload?.message,
      data: payload?.data
    });
  }

  const detail = payload?.data || {};
  return jsonRes({
    data: {
      id: detail.uid || '',
      name: detail.name || devboxName,
      status: detail.status || 'Pending',
      createTime: detail.createTime || '',
      imageName: detail.image || '',
      cpu: Number(detail.resources?.cpu || 0) * 1000,
      memory: Number(detail.resources?.memory || 0) * 1024,
      networks: Array.isArray(detail.ports) ? detail.ports.map(normalizeNetwork) : [],
      sshPort: detail.ssh?.port || 0,
      base64PrivateKey: detail.ssh?.privateKey || '',
      userName: detail.ssh?.user || 'devbox',
      workingDir: detail.ssh?.workingDir || '/home/devbox/project',
      domain: detail.ssh?.host || process.env.SEALOS_DOMAIN
    }
  });
}
