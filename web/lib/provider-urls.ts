export const PROVIDERS = ["github", "dockerhub", "ecr-public", "gitlab", "pypi", "npm"] as const;
export type Provider = (typeof PROVIDERS)[number];

const PROVIDER_URLS: Record<Provider, (repository: string, version?: string) => string> = {
  github: (r, v) => v ? `https://github.com/${r}/releases/tag/${v}` : `https://github.com/${r}`,
  dockerhub: (r, v) => v ? `https://hub.docker.com/r/${r}/tags?name=${encodeURIComponent(v)}` : `https://hub.docker.com/r/${r}`,
  "ecr-public": (r) => `https://gallery.ecr.aws/${r}`,
  gitlab: (r, v) => v ? `https://gitlab.com/${r}/-/releases/${v}` : `https://gitlab.com/${r}`,
  pypi: (r, v) => v ? `https://pypi.org/project/${r}/${encodeURIComponent(v)}/` : `https://pypi.org/project/${r}`,
  npm: (r, v) => v ? `https://www.npmjs.com/package/${r}/v/${encodeURIComponent(v)}` : `https://www.npmjs.com/package/${r}`,
};

export function getProviderUrl(
  provider: string,
  repository: string,
  version?: string
): string | null {
  const fn = PROVIDER_URLS[provider as Provider];
  return fn ? fn(repository, version) : null;
}
