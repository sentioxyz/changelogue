/**
 * Extract path segments from window.location.pathname.
 *
 * Next.js static export bakes route params into the RSC payload at build time
 * (always "0" from generateStaticParams), so useParams() returns stale values
 * when serving a different URL. This helper reads directly from the browser URL.
 */
export function getPathSegment(index: number): string {
  if (typeof window === "undefined") return "";
  const segments = window.location.pathname.split("/").filter(Boolean);
  return segments[index] ?? "";
}
