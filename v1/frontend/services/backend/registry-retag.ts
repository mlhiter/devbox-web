const MANIFEST_ACCEPT = [
  'application/vnd.oci.image.index.v1+json',
  'application/vnd.oci.image.manifest.v1+json',
  'application/vnd.docker.distribution.manifest.list.v2+json',
  'application/vnd.docker.distribution.manifest.v2+json'
].join(', ');

type ImageRef = {
  registry: string;
  repository: string;
  reference: string;
};

type Descriptor = {
  mediaType?: string;
  digest?: string;
  size?: number;
  platform?: unknown;
  annotations?: Record<string, string>;
};

type Manifest = {
  schemaVersion?: number;
  mediaType?: string;
  config?: Descriptor;
  layers?: Descriptor[];
  manifests?: Descriptor[];
};

type RegistryCredentials = {
  username: string;
  password: string;
};

type RegistryRequestOptions = {
  method?: string;
  body?: BodyInit | ReadableStream<Uint8Array>;
  headers?: Record<string, string>;
  accept?: string;
  scope?: string | string[];
};

type FetchInit = Omit<RequestInit, 'body'> & {
  body?: BodyInit | ReadableStream<Uint8Array>;
  duplex?: 'half';
};

class RegistryRetagError extends Error {
  status?: number;

  constructor(message: string, status?: number) {
    super(message);
    this.name = 'RegistryRetagError';
    this.status = status;
  }
}

const normalizeRegistry = (registry: string) =>
  registry.replace(/^https?:\/\//, '').replace(/\/+$/, '');

const isTruthyEnv = (value?: string) =>
  ['1', 'true', 'yes', 'on'].includes(value?.toLowerCase() ?? '');

const parseImageRef = (image: string): ImageRef => {
  const raw = image.trim();
  const schemeMatch = raw.match(/^(https?:\/\/)(.+)$/);
  const trimmed = schemeMatch ? schemeMatch[2] : raw;
  const firstSlash = trimmed.indexOf('/');

  if (firstSlash <= 0) {
    throw new RegistryRetagError(`Invalid image reference: ${image}`);
  }

  const registry = `${schemeMatch?.[1] || ''}${trimmed.slice(0, firstSlash)}`;
  const remainder = trimmed.slice(firstSlash + 1);
  const digestIndex = remainder.indexOf('@');

  if (digestIndex > 0) {
    return {
      registry,
      repository: remainder.slice(0, digestIndex),
      reference: remainder.slice(digestIndex + 1)
    };
  }

  const lastSlash = remainder.lastIndexOf('/');
  const tagIndex = remainder.lastIndexOf(':');

  if (tagIndex > lastSlash) {
    return {
      registry,
      repository: remainder.slice(0, tagIndex),
      reference: remainder.slice(tagIndex + 1)
    };
  }

  return {
    registry,
    repository: remainder,
    reference: 'latest'
  };
};

const getRegistryCredentials = (): RegistryCredentials => {
  const username = process.env.REGISTRY_USER || '';
  const password = process.env.REGISTRY_PASSWORD || '';

  if (!username || !password) {
    throw new RegistryRetagError('Registry credentials are not configured');
  }

  return { username, password };
};

const registryBaseUrl = (registry: string) => {
  const trimmed = registry.trim().replace(/\/+$/, '');

  if (/^https?:\/\//.test(trimmed)) {
    return trimmed;
  }

  const scheme = isTruthyEnv(process.env.REGISTRY_INSECURE) ? 'http' : 'https';
  return `${scheme}://${normalizeRegistry(trimmed)}`;
};

const authHeader = ({ username, password }: RegistryCredentials) =>
  `Basic ${Buffer.from(`${username}:${password}`).toString('base64')}`;

const parseAuthenticateHeader = (value: string) => {
  const separatorIndex = value.indexOf(' ');
  const scheme = separatorIndex >= 0 ? value.slice(0, separatorIndex) : value;
  const paramsRaw = separatorIndex >= 0 ? value.slice(separatorIndex + 1) : '';
  const params: Record<string, string> = {};
  const pattern = /(\w+)="([^"]*)"/g;
  let match = pattern.exec(paramsRaw);

  while (match) {
    params[match[1]] = match[2];
    match = pattern.exec(paramsRaw);
  }

  return {
    scheme: scheme?.toLowerCase(),
    params
  };
};

const bearerTokenCache = new Map<string, string>();

const scopeCachePart = (scope?: string | string[]) =>
  (Array.isArray(scope) ? [...scope].sort() : scope ? [scope] : []).join(',');

const bearerTokenCacheKey = (
  url: string | URL,
  credentials: RegistryCredentials,
  scope?: string | string[]
) => `${new URL(url.toString()).origin}|${credentials.username}|${scopeCachePart(scope)}`;

const preloadBearerToken = async (
  url: string | URL,
  credentials: RegistryCredentials,
  scope?: string | string[]
) => {
  if (!scope) return null;

  const registryOrigin = new URL(url.toString()).origin;
  const response = await fetch(`${registryOrigin}/v2/`, {
    cache: 'no-store',
    headers: {
      Authorization: authHeader(credentials)
    }
  });

  if (response.status !== 401) {
    return null;
  }

  return getBearerToken(response.headers.get('www-authenticate') || '', credentials, scope);
};

const getBearerToken = async (
  challenge: string,
  credentials: RegistryCredentials,
  requestedScope?: string | string[]
) => {
  const { scheme, params } = parseAuthenticateHeader(challenge);

  if (scheme !== 'bearer' || !params.realm) {
    return null;
  }

  const url = new URL(params.realm);

  if (params.service) {
    url.searchParams.set('service', params.service);
  }

  const scopes = Array.isArray(requestedScope)
    ? requestedScope
    : requestedScope
      ? [requestedScope]
      : params.scope
        ? [params.scope]
        : [];

  for (const scope of scopes) {
    url.searchParams.append('scope', scope);
  }

  const response = await fetch(url, {
    headers: {
      Authorization: authHeader(credentials)
    }
  });
  await assertOk(response, 'Get registry bearer token');

  const body = (await response.json()) as { token?: string; access_token?: string };
  return body.token || body.access_token || null;
};

const fetchWithAuthRetry = async (
  url: string | URL,
  credentials: RegistryCredentials,
  init: FetchInit,
  scope?: string | string[]
) => {
  const tokenCacheKey = bearerTokenCacheKey(url, credentials, scope);
  const isStreamingBody = init.body instanceof ReadableStream;
  let bearerToken = bearerTokenCache.get(tokenCacheKey);

  if (isStreamingBody && !bearerToken) {
    bearerToken = (await preloadBearerToken(url, credentials, scope)) || undefined;
    if (bearerToken) {
      bearerTokenCache.set(tokenCacheKey, bearerToken);
    }
  }

  const requestInit: FetchInit = {
    ...init,
    cache: 'no-store',
    duplex: isStreamingBody ? 'half' : init.duplex,
    headers: {
      Authorization: bearerToken ? `Bearer ${bearerToken}` : authHeader(credentials),
      ...(init.headers || {})
    }
  };

  const response = await fetch(url, requestInit as RequestInit);

  if (response.status !== 401) {
    return response;
  }

  if (isStreamingBody) {
    return response;
  }

  bearerToken =
    (await getBearerToken(response.headers.get('www-authenticate') || '', credentials, scope)) ||
    undefined;

  if (!bearerToken) {
    return response;
  }
  bearerTokenCache.set(tokenCacheKey, bearerToken);

  const retryInit: FetchInit = {
    ...requestInit,
    headers: {
      ...requestInit.headers,
      Authorization: `Bearer ${bearerToken}`
    }
  };
  return fetch(url, retryInit as RequestInit);
};

const readErrorBody = async (response: Response) => {
  try {
    const text = await response.text();
    return text ? `: ${text.slice(0, 500)}` : '';
  } catch {
    return '';
  }
};

const registryFetch = async (
  image: Pick<ImageRef, 'registry' | 'repository'>,
  path: string,
  credentials: RegistryCredentials,
  options: RegistryRequestOptions = {}
) => {
  const url = `${registryBaseUrl(image.registry)}${path}`;
  return fetchWithAuthRetry(
    url,
    credentials,
    {
      method: options.method || 'GET',
      body: options.body,
      duplex: options.body instanceof ReadableStream ? 'half' : undefined,
      headers: {
        ...(options.accept ? { Accept: options.accept } : {}),
        ...options.headers
      }
    },
    options.scope
  );
};

const assertOk = async (response: Response, action: string) => {
  if (!response.ok) {
    throw new RegistryRetagError(
      `${action} failed with HTTP ${response.status}${await readErrorBody(response)}`,
      response.status
    );
  }
};

const getManifest = async (
  image: ImageRef,
  credentials: RegistryCredentials,
  reference = image.reference
) => {
  const response = await registryFetch(
    image,
    `/v2/${image.repository}/manifests/${encodeURIComponent(reference)}`,
    credentials,
    {
      accept: MANIFEST_ACCEPT,
      scope: `repository:${image.repository}:pull`
    }
  );
  await assertOk(response, `Get manifest ${image.repository}:${reference}`);

  const contentType = response.headers.get('content-type')?.split(';')[0] || '';
  const body = await response.text();
  const manifest = JSON.parse(body) as Manifest;

  return {
    body,
    contentType:
      contentType || manifest.mediaType || 'application/vnd.docker.distribution.manifest.v2+json',
    manifest
  };
};

const layerBlobDigests = (manifest: Manifest): string[] => {
  const digests = new Set<string>();

  if (manifest.config?.digest) {
    digests.add(manifest.config.digest);
  }

  for (const layer of manifest.layers || []) {
    if (layer.digest) digests.add(layer.digest);
  }

  return [...digests];
};

const blobExists = async (
  image: Pick<ImageRef, 'registry' | 'repository'>,
  digest: string,
  credentials: RegistryCredentials
) => {
  const response = await registryFetch(
    image,
    `/v2/${image.repository}/blobs/${digest}`,
    credentials,
    {
      method: 'HEAD',
      scope: `repository:${image.repository}:pull`
    }
  );

  if (response.status === 200) return true;
  if (response.status === 404) return false;
  await assertOk(response, `Check blob ${digest}`);
  return true;
};

const mountBlob = async (
  source: ImageRef,
  target: ImageRef,
  digest: string,
  credentials: RegistryCredentials
) => {
  const query = new URLSearchParams({
    mount: digest,
    from: source.repository
  });
  const response = await registryFetch(
    target,
    `/v2/${target.repository}/blobs/uploads/?${query.toString()}`,
    credentials,
    {
      method: 'POST',
      scope: [`repository:${target.repository}:pull,push`, `repository:${source.repository}:pull`],
      headers: {
        'Content-Length': '0'
      }
    }
  );

  if (response.status === 201) return true;
  if (response.status === 202 || response.status === 404 || response.status === 405) return false;
  await assertOk(response, `Mount blob ${digest}`);
  return false;
};

const uploadBlob = async (
  source: ImageRef,
  target: ImageRef,
  digest: string,
  credentials: RegistryCredentials
) => {
  const blobResponse = await registryFetch(
    source,
    `/v2/${source.repository}/blobs/${digest}`,
    credentials,
    {
      accept: 'application/octet-stream',
      scope: `repository:${source.repository}:pull`
    }
  );
  await assertOk(blobResponse, `Download blob ${digest}`);

  if (!blobResponse.body) {
    throw new RegistryRetagError(`Download blob ${digest} did not return a response body`);
  }

  const startResponse = await registryFetch(
    target,
    `/v2/${target.repository}/blobs/uploads/`,
    credentials,
    {
      method: 'POST',
      scope: `repository:${target.repository}:pull,push`,
      headers: {
        'Content-Length': '0'
      }
    }
  );
  await assertOk(startResponse, `Start blob upload ${digest}`);

  const uploadLocation = startResponse.headers.get('location');
  if (!uploadLocation) {
    throw new RegistryRetagError(`Start blob upload ${digest} did not return a location`);
  }

  const uploadUrl = new URL(uploadLocation, registryBaseUrl(target.registry));
  uploadUrl.searchParams.set('digest', digest);
  const uploadResponse = await fetchWithAuthRetry(
    uploadUrl,
    credentials,
    {
      method: 'PUT',
      body: blobResponse.body,
      headers: {
        'Content-Type': 'application/octet-stream'
      }
    },
    `repository:${target.repository}:pull,push`
  );

  if (uploadResponse.status !== 201) {
    await assertOk(uploadResponse, `Upload blob ${digest}`);
    throw new RegistryRetagError(`Upload blob ${digest} returned HTTP ${uploadResponse.status}`);
  }
};

const ensureBlob = async (
  source: ImageRef,
  target: ImageRef,
  digest: string,
  credentials: RegistryCredentials
) => {
  if (await blobExists(target, digest, credentials)) {
    return;
  }

  if (
    normalizeRegistry(source.registry) === normalizeRegistry(target.registry) &&
    (await mountBlob(source, target, digest, credentials))
  ) {
    return;
  }

  await uploadBlob(source, target, digest, credentials);
};

const putManifest = async (
  target: ImageRef,
  manifestBody: string,
  contentType: string,
  credentials: RegistryCredentials,
  reference = target.reference
) => {
  const response = await registryFetch(
    target,
    `/v2/${target.repository}/manifests/${encodeURIComponent(reference)}`,
    credentials,
    {
      method: 'PUT',
      body: manifestBody,
      scope: `repository:${target.repository}:pull,push`,
      headers: {
        'Content-Type': contentType,
        'Content-Length': String(Buffer.byteLength(manifestBody))
      }
    }
  );

  await assertOk(response, `Put manifest ${target.repository}:${reference}`);
};

const copyManifest = async (
  source: ImageRef,
  target: ImageRef,
  credentials: RegistryCredentials,
  sourceReference = source.reference
) => {
  const { body, contentType, manifest } = await getManifest(source, credentials, sourceReference);

  for (const childManifest of manifest.manifests || []) {
    if (childManifest.digest) {
      await copyManifest(source, target, credentials, childManifest.digest);
    }
  }

  for (const digest of layerBlobDigests(manifest)) {
    await ensureBlob(source, target, digest, credentials);
  }

  if (sourceReference !== source.reference) {
    await putManifest(target, body, contentType, credentials, sourceReference);
  }

  return {
    body,
    contentType
  };
};

export const retagImage = async (original: string, target: string) => {
  const sourceRef = parseImageRef(original);
  const targetRef = parseImageRef(target);
  const credentials = getRegistryCredentials();
  const { body, contentType } = await copyManifest(sourceRef, targetRef, credentials);
  await putManifest(targetRef, body, contentType, credentials);
};
