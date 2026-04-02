const BASE_URL: string =
  (typeof import.meta !== "undefined" && import.meta.env?.VITE_API_BASE_URL) || "/api";

const runtimeOrigin = typeof window === "undefined" ? "http://localhost" : window.location.origin;
const resolvedBaseURL: URL = new URL(BASE_URL, runtimeOrigin);

// WebSocket 仅支持同源连接：固定使用当前页面 origin，只复用 API 基础路径。
const pageProtocol =
  typeof window === "undefined" ? resolvedBaseURL.protocol : window.location.protocol;
const wsProtocol = pageProtocol === "https:" ? "wss:" : "ws:";
const wsHost = typeof window === "undefined" ? resolvedBaseURL.host : window.location.host;
const wsPath = resolvedBaseURL.pathname.replace(/\/+$/, "");
export const WS_BASE_URL = `${wsProtocol}//${wsHost}${wsPath}`;

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

export interface WsInstancesUpdated {
  type: "instances_updated";
  data: Instance[];
}

export type WsMessage = WsInstancesUpdated;

export interface Game {
  id: string;
  name: string;
  description: string;
  category: string;
  icon: string;
}

export interface TemplateImage {
  name: string;
  tag: string;
}

export interface PortMapping {
  host: number;
  container: number;
  protocol: string;
}

export interface VolumeMount {
  name: string;
  container_path: string;
  readonly: boolean;
}

export interface ResourceLimits {
  memory: string;
  cpu: number;
}

export interface HealthCheckConfig {
  test: string[];
  interval: string;
  timeout: string;
  retries: number;
  start_period: string;
}

export interface ContainerConfig {
  ports: PortMapping[];
  env: Record<string, string>;
  volumes: VolumeMount[];
  resources?: ResourceLimits;
  command?: string[];
  health_check?: HealthCheckConfig;
}

export interface ParamOption {
  value: string;
  label: string;
}

export type TemplateParamType = "string" | "number" | "boolean" | "select";

export type TemplateParamDefault = string | number | boolean;

export interface TemplateParam {
  key: string;
  label: string;
  description: string;
  type: TemplateParamType;
  default: TemplateParamDefault;
  options?: ParamOption[];
  env_var?: string;
}

export interface GameTemplate {
  image: TemplateImage;
  container: ContainerConfig;
  params: TemplateParam[];
}

export function listInstances(): Promise<Instance[]> {
  return request<Instance[]>("/instances", { method: "GET" });
}

export function listGames(): Promise<Game[]> {
  return request<Game[]>("/games", { method: "GET" });
}

export function getGameTemplate(id: string): Promise<GameTemplate> {
  return request<GameTemplate>(`/games/${encodeURIComponent(id)}/template`, { method: "GET" });
}

export function createInstance(
  name: string,
  gameId: string,
  params: Record<string, string> = {},
): Promise<unknown> {
  return request("/instances", {
    method: "POST",
    body: { name, game_id: gameId, params },
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
