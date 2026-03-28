const BASE_URL: string =
  (typeof import.meta !== "undefined" && import.meta.env?.VITE_API_BASE_URL) || "/api";

interface RequestOptions extends RequestInit {
  headers?: Record<string, string>;
}

async function request<T = unknown>(path: string, options: RequestOptions = {}): Promise<T> {
  const resp = await fetch(`${BASE_URL}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });

  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    const message = data?.error || `request failed: ${resp.status}`;
    throw new Error(message);
  }
  return data as T;
}

export interface Instance {
  container_id: string;
  name: string;
  status: string;
}

export function listInstances(): Promise<Instance[]> {
  return request<Instance[]>("/instances", { method: "GET" });
}

export function createInstance(name: string): Promise<unknown> {
  return request("/instances", {
    method: "POST",
    body: JSON.stringify({ name }),
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
