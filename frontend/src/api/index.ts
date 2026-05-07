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

// consoleWsUrl 构造容器控制台 WebSocket 地址。
export function consoleWsUrl(containerId: string): string {
  return `${WS_BASE_URL}/ws/console/${encodeURIComponent(containerId)}`;
}

type JsonObject = Record<string, unknown>;

interface RequestOptions extends Omit<RequestInit, "headers" | "body"> {
  headers?: HeadersInit;
  body?: BodyInit | JsonObject | null;
  timeoutMs?: number;
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
  const { headers: customHeaders, body, timeoutMs = 30000, signal, ...rest } = options;
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

  const controller = typeof AbortController !== "undefined" ? new AbortController() : null;
  let timeoutID: ReturnType<typeof setTimeout> | undefined;
  if (controller && timeoutMs > 0) {
    timeoutID = setTimeout(() => controller.abort(), timeoutMs);
  }

  let resp: Response;
  try {
    resp = await fetch(`${BASE_URL}${path}`, {
      ...rest,
      headers,
      body: finalBody,
      signal: signal ?? controller?.signal,
    });
  } catch (err) {
    if (err instanceof DOMException && err.name === "AbortError") {
      throw new ApiRequestError({ key: "errors.timeout" });
    }
    throw new ApiRequestError({ key: "errors.network" });
  } finally {
    if (timeoutID !== undefined) {
      clearTimeout(timeoutID);
    }
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
  game_id?: string;
  status: string;
}

export interface InstanceConfig {
  game_id: string;
  status: string;
  image?: string;
  ports: PortMapping[];
  params: Record<string, string>;
  resources?: ResourceLimits;
  game_config?: Record<string, string>;
}

export interface UpdateConfigResponse {
  status: string;
  container_id: string;
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

export interface FileMount {
  name: string;
  container_path: string;
  readonly: boolean;
}

export interface FileEntry {
  name: string;
  is_dir: boolean;
  size: number;
  modified_at: string;
}

export interface ResourceLimits {
  memory: string;
  cpu: number;
}

export interface MinecraftVersionOption {
  id: string;
  type: string;
}

export interface MinecraftLoaderVersionOption {
  version: string;
  stable?: boolean;
}

export interface ServerCPUMetrics {
  percent: number;
  cores: number[];
  logical_cores: number;
  model: string;
}

export interface ServerMemoryMetrics {
  percent: number;
  used_bytes: number;
  total_bytes: number;
  available_bytes: number;
  swap_in_bps: number;
  swap_out_bps: number;
  model: string;
}

export interface ServerDiskMetrics {
  id: string;
  label: string;
  name: string;
  mountpoint: string;
  percent: number;
  used_bytes: number;
  total_bytes: number;
  read_bps: number;
  write_bps: number;
}

export interface ServerNetworkMetrics {
  name: string;
  rx_bps: number;
  tx_bps: number;
}

export interface ServerMetrics {
  timestamp: number;
  cpu: ServerCPUMetrics;
  memory: ServerMemoryMetrics;
  disks: ServerDiskMetrics[];
  network: ServerNetworkMetrics;
}

export interface SystemLogEntry {
  time: string;
  level: string;
  message: string;
  attributes?: Record<string, unknown>;
  raw?: string;
}

export interface SystemLogsResponse {
  path: string;
  entries: SystemLogEntry[];
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

export function listMinecraftVersions(): Promise<MinecraftVersionOption[]> {
  return request<MinecraftVersionOption[]>("/games/minecraft-java/versions", { method: "GET" });
}

export function listMinecraftLoaderVersions(
  mcVersion: string,
  serverType: string,
): Promise<MinecraftLoaderVersionOption[]> {
  const query = new URLSearchParams({ mc_version: mcVersion, server_type: serverType });
  return request<MinecraftLoaderVersionOption[]>(
    `/games/minecraft-java/loader-versions?${query.toString()}`,
    { method: "GET" },
  );
}

export function createInstance(
  name: string,
  gameId: string,
  params: Record<string, string> = {},
  ports: PortMapping[] = [],
  resources?: ResourceLimits,
): Promise<unknown> {
  return request("/instances", {
    method: "POST",
    body: { name, game_id: gameId, params, ports, resources },
    timeoutMs: 10 * 60 * 1000,
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

export function deleteInstance(containerId: string, purgeData = false): Promise<unknown> {
  const query = purgeData ? "?purge_data=true" : "";
  return request(`/instances/${containerId}${query}`, {
    method: "DELETE",
  });
}

export function listInstanceFileMounts(containerId: string): Promise<FileMount[]> {
  return request<FileMount[]>(`/instances/${encodeURIComponent(containerId)}/files/mounts`, {
    method: "GET",
  });
}

export function listInstanceFiles(
  containerId: string,
  mount: string,
  path: string,
): Promise<FileEntry[]> {
  const query = new URLSearchParams({ mount, path });
  return request<FileEntry[]>(
    `/instances/${encodeURIComponent(containerId)}/files?${query.toString()}`,
    { method: "GET" },
  );
}

export function createInstanceDir(
  containerId: string,
  mount: string,
  path: string,
): Promise<unknown> {
  return request(`/instances/${encodeURIComponent(containerId)}/files/dir`, {
    method: "POST",
    body: { mount, path },
  });
}

export function uploadInstanceFile(
  containerId: string,
  mount: string,
  path: string,
  file: File,
): Promise<unknown> {
  const form = new FormData();
  form.set("mount", mount);
  form.set("path", path);
  form.set("file", file);
  return request(`/instances/${encodeURIComponent(containerId)}/files/upload`, {
    method: "POST",
    body: form,
  });
}

export function downloadInstanceFileUrl(containerId: string, mount: string, path: string): string {
  const query = new URLSearchParams({ mount, path });
  return `${BASE_URL}/instances/${encodeURIComponent(containerId)}/files/download?${query.toString()}`;
}

export function deleteInstanceFile(
  containerId: string,
  mount: string,
  path: string,
  recursive: boolean,
): Promise<unknown> {
  const query = new URLSearchParams({ mount, path, recursive: String(recursive) });
  return request(`/instances/${encodeURIComponent(containerId)}/files?${query.toString()}`, {
    method: "DELETE",
  });
}

export function getInstanceConfig(containerId: string): Promise<InstanceConfig> {
  return request<InstanceConfig>(`/instances/${encodeURIComponent(containerId)}/config`, {
    method: "GET",
  });
}

export function updateInstanceConfig(
  containerId: string,
  params: Record<string, string>,
  ports: PortMapping[],
  resources?: ResourceLimits,
): Promise<UpdateConfigResponse> {
  return request<UpdateConfigResponse>(`/instances/${encodeURIComponent(containerId)}/config`, {
    method: "PUT",
    body: { params, ports, resources },
  });
}

export function getServerMetrics(): Promise<ServerMetrics> {
  return request<ServerMetrics>("/monitor/server", { method: "GET" });
}

export interface ContainerStats {
  timestamp: number;
  cpu_percent: number;
  memory_used_bytes: number;
  memory_max_bytes: number;
  memory_percent: number;
  network_rx_bytes: number;
  network_tx_bytes: number;
  disk_read_bytes: number;
  disk_write_bytes: number;
}

export function getContainerStats(containerId: string): Promise<ContainerStats> {
  return request<ContainerStats>(`/instances/${encodeURIComponent(containerId)}/stats`, {
    method: "GET",
  });
}

export function getSystemLogs(tail: number, level = "", query = ""): Promise<SystemLogsResponse> {
  const params = new URLSearchParams({ tail: String(tail) });
  if (level) {
    params.set("level", level);
  }
  if (query) {
    params.set("q", query);
  }
  return request<SystemLogsResponse>(`/system/logs?${params.toString()}`, { method: "GET" });
}
