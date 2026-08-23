import type {
  ExecutionResponse,
  OpenAPISpec,
  OperationLookupResponse,
  OperationRequestValues,
  OperationsApiClient,
} from "@flanksource/clicky-ui";

// substitutePath replaces `{name}` segments in an OpenAPI path with the
// corresponding values from `params`, returning the resolved path and the
// remaining params (those not consumed by a path segment).
function substitutePath(
  path: string,
  params: OperationRequestValues,
): { resolved: string; remaining: OperationRequestValues } {
  const remaining = { ...params };
  let resolved = path;
  for (const [key, value] of Object.entries(params)) {
    if (!resolved.includes(`{${key}}`)) {
      continue;
    }
    if (Array.isArray(value)) {
      throw new Error(`Path parameter ${key} must be a scalar value`);
    }
    resolved = resolved.replace(`{${key}}`, encodeURIComponent(value));
    delete remaining[key];
    delete remaining.args;
  }
  return { resolved, remaining };
}

// Multi-valued params are sent comma-joined, matching how the Go executor
// decodes repeated filter values (rpc/executor.go splits on ",").
function toSearchParams(params: OperationRequestValues): URLSearchParams {
  return new URLSearchParams(
    Object.entries(params).map(([key, value]) => [
      key,
      Array.isArray(value) ? value.join(",") : value,
    ]),
  );
}

async function request(
  path: string,
  method: string,
  body?: unknown,
  headers?: Record<string, string>,
  requestUrl?: string,
): Promise<ExecutionResponse> {
  const upper = method.toUpperCase();
  const init: RequestInit = {
    method: upper,
    headers: { Accept: "application/json", ...(headers ?? {}) },
  };
  if (upper !== "GET" && body !== undefined) {
    init.headers = { "Content-Type": "application/json", ...init.headers };
    init.body = JSON.stringify(body);
  }
  const response = await fetch(path, init);
  const text = await response.text();
  const contentType = response.headers.get("Content-Type") || undefined;
  if (!response.ok) {
    throw new Error(
      `${upper} ${path} failed with ${response.status}: ${text || response.statusText}`,
    );
  }
  return {
    success: true,
    exit_code: 0,
    stdout: text,
    contentType,
    requestUrl,
    parsed: maybeParseJson(text, contentType),
  };
}

function maybeParseJson(text: string, contentType?: string) {
  const trimmed = text.trim();
  if (!trimmed) {
    return undefined;
  }

  const shouldParse =
    contentType?.toLowerCase().includes("json") ||
    trimmed.startsWith("{") ||
    trimmed.startsWith("[");
  if (!shouldParse) {
    return undefined;
  }

  try {
    return JSON.parse(trimmed) as unknown;
  } catch {
    return undefined;
  }
}

export const apiClient: OperationsApiClient = {
  async getOpenAPISpec(): Promise<OpenAPISpec> {
    const response = await fetch("/api/openapi.json", {
      headers: { Accept: "application/json" },
    });
    if (!response.ok) {
      throw new Error(
        `GET /api/openapi.json failed with ${response.status}: ${response.statusText}`,
      );
    }
    return (await response.json()) as OpenAPISpec;
  },

  async executeCommand(path, method, params, headers) {
    const { resolved, remaining } = substitutePath(path, params);
    if (method.toUpperCase() === "GET") {
      const search = toSearchParams(remaining).toString();
      const url = search ? `${resolved}?${search}` : resolved;
      return request(url, method, undefined, headers, url);
    }
    return request(resolved, method, remaining, headers);
  },

  async lookupFilters(
    path: string,
    _method: string,
    params: OperationRequestValues,
    headers?: Record<string, string>,
  ): Promise<OperationLookupResponse> {
    const { resolved, remaining } = substitutePath(path, params);
    const searchParams = toSearchParams(remaining);
    searchParams.set("__lookup", "filters");
    const url = `${resolved}?${searchParams.toString()}`;
    const response = await fetch(url, {
      method: "GET",
      headers: { Accept: "application/json+clicky", ...(headers ?? {}) },
    });
    if (!response.ok) {
      throw new Error(
        `GET ${url} failed with ${response.status}: ${await response.text() || response.statusText}`,
      );
    }
    return (await response.json()) as OperationLookupResponse;
  },
};
