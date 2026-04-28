export function getProviderUrl(
  provider: string,
  repository: string,
  version?: string
): string | null {
  switch (provider) {
    case "github":
      return version
        ? `https://github.com/${repository}/releases/tag/${version}`
        : `https://github.com/${repository}`;
    case "dockerhub":
      return version
        ? `https://hub.docker.com/r/${repository}/tags?name=${encodeURIComponent(version)}`
        : `https://hub.docker.com/r/${repository}`;
    case "ecr-public":
      return `https://gallery.ecr.aws/${repository}`;
    case "gitlab":
      return version
        ? `https://gitlab.com/${repository}/-/releases/${version}`
        : `https://gitlab.com/${repository}`;
    case "pypi":
      return version
        ? `https://pypi.org/project/${repository}/${encodeURIComponent(version)}/`
        : `https://pypi.org/project/${repository}`;
    case "npm":
      return version
        ? `https://www.npmjs.com/package/${repository}/v/${encodeURIComponent(version)}`
        : `https://www.npmjs.com/package/${repository}`;
    default:
      return null;
  }
}
