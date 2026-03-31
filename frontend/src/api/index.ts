const BASE_URL: string =
  (typeof import.meta !== "undefined" && import.meta.env?.VITE_API_BASE_URL) || "/api";

type JsonObject = Record<string, unknown>;

interface RequestOptions extends Omit<RequestInit, "headers" | "body"> {
  headers?: HeadersInit;
  body?: BodyInit | JsonObject | null;
}

export interface ApiRequestErrorInfo {
  key: string;
  status?: number;
  backendMessage?: string;
}

export class ApiRequestError extends Error {
  readonly key: string;
  readonly status?: number;
  readonly backendMessage?: string;

  constructor(info: ApiRequestErrorInfo) {
    super(info.key);
    this.name = "ApiRequestError";
    this.key = info.key;
    this.status = info.status;
    this.backendMessage = info.backendMessage;
  }
}

function isPlainObject(value: unknown): value is JsonObject {
  return Object.prototype.toString.call(value) === "[object Object]";
}

function getResponseBackendError(data: unknown): string | undefined {
  // 后端契约约定优先返回 error 字段，前端统一在这里提取后再映射到 i18n key。
  if (data && typeof data === "object" && "error" in data) {
    const error = (data as { error?: unknown }).error;
    if (typeof error === "string" && error.trim().length > 0) {
      return error.trim();
    }
  }
  return undefined;
}

async function request<T = unknown>(path: string, options: RequestOptions = {}): Promise<T> {
  // 统一处理请求体序列化、默认请求头和非 2xx 错误转换。
  const { headers: customHeaders, body, ...rest } = options;
  const headers = new Headers(customHeaders);

  let finalBody: BodyInit | undefined;
  if (body == null) {
    finalBody = undefined;
  } else if (isPlainObject(body)) {
    finalBody = JSON.stringify(body);
    if (!headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
  } else {
    finalBody = body;
  }

  if (!headers.has("Accept")) {
    headers.set("Accept", "application/json");
  }

  let resp: Response;
  try {
    resp = await fetch(`${BASE_URL}${path}`, {
      ...rest,
      headers,
      body: finalBody,
    });
  } catch {
    throw new ApiRequestError({ key: "errors.network" });
  }

  const data: unknown = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    throw new ApiRequestError({
      key: "errors.httpStatus",
      status: resp.status,
      backendMessage: getResponseBackendError(data),
    });
  }
  return data as T;
}

export interface Instance {
  container_id: string;
  name: string;
  status: string;
}

export interface RegistryImage {
  id: string;
  name: string;
  image: string;
  description: string;
  category: string;
  icon: string;
  default_env: Record<string, string>;
  default_ports: string[];
}

export function listInstances(): Promise<Instance[]> {
  return request<Instance[]>("/instances", { method: "GET" });
}

export function listRegistryImages(): Promise<RegistryImage[]> {
  return request<RegistryImage[]>("/registry/images", { method: "GET" });
}

export function createInstance(name: string, imageId: string): Promise<unknown> {
  return request("/instances", {
    method: "POST",
    body: { name, image_id: imageId },
  });
}

export function startInstance(containerId: string): Promise<unknown> {
  return request(`/instances/${containerId}/start`, {
    method: "POST",
  });
}

export function stopInstance(containerId: string): Promise<unknown> {
  return request(`/instances/${containerId}/stop`, {
    method: "POST",
  });
}

export function deleteInstance(containerId: string): Promise<unknown> {
  return request(`/instances/${containerId}`, {
    method: "DELETE",
  });
}
