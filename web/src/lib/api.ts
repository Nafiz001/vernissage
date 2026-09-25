// A thin, typed client for the Go API. In the browser requests go to the
// same origin (Next rewrites them); on the server they go straight to Go.

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public fields?: Record<string, string>,
    public problems?: { artworkId: number; message: string }[],
  ) {
    super(message);
  }
}

const serverBase = process.env.VERNISSAGE_API ?? "http://127.0.0.1:8790";

function base() {
  return typeof window === "undefined" ? serverBase : "";
}

export async function api<T>(path: string, init?: RequestInit & { json?: unknown }): Promise<T> {
  const headers = new Headers(init?.headers);
  let body = init?.body;
  if (init?.json !== undefined) {
    headers.set("Content-Type", "application/json");
    body = JSON.stringify(init.json);
  }
  let res: Response;
  try {
    res = await fetch(base() + path, { ...init, headers, body, credentials: "same-origin" });
  } catch {
    throw new ApiError(0, "Couldn't reach the server. Check your connection and try again.");
  }
  const data = await res.json().catch(() => null);
  if (!res.ok) {
    const e = data?.error ?? {};
    throw new ApiError(res.status, e.message ?? `The server answered ${res.status}.`, e.fields, e.problems);
  }
  return data as T;
}

/** Server-side fetch that forwards the visitor's cookies. */
export async function serverApi<T>(path: string, cookie?: string, revalidate = 0): Promise<T> {
  return api<T>(path, {
    headers: cookie ? { cookie } : undefined,
    ...(revalidate > 0 ? { next: { revalidate } } : { cache: "no-store" }),
  } as RequestInit);
}

export const IMAGE_WIDTHS = [200, 400, 800, 1200, 1600, 2400] as const;

export function img(id: number, width: (typeof IMAGE_WIDTHS)[number]) {
  return `/img/${id}/${width}.jpg`;
}

export function srcSet(id: number, max: number = 1200) {
  return IMAGE_WIDTHS.filter((w) => w <= max)
    .map((w) => `${img(id, w)} ${w}w`)
    .join(", ");
}

export function wsURL(path: string) {
  const configured = process.env.NEXT_PUBLIC_WS_ORIGIN;
  if (configured) return configured + path;
  const { protocol, hostname, port } = window.location;
  const scheme = protocol === "https:" ? "wss:" : "ws:";
  // In development Next serves on 3790 and Go on 8790.
  const host = port === "3790" ? `${hostname}:8790` : window.location.host;
  return `${scheme}//${host}${path}`;
}
