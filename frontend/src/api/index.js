const BASE_URL =
  (typeof import.meta !== "undefined" && import.meta.env?.VITE_API_BASE_URL) ||
  "http://localhost:8080/api";

async function request(path, options = {}) {
  const resp = await fetch(`${BASE_URL}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });

  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    const message = data?.error || `request failed: ${resp.status}`;
    throw new Error(message);
  }
  return data;
}

export function listInstances() {
  return request("/instances", { method: "GET" });
}

export function createInstance(name) {
  return request("/instances", {
    method: "POST",
    body: JSON.stringify({ name }),
  });
}

export function startInstance(containerId) {
  return request(`/instances/${containerId}/start`, {
    method: "POST",
  });
}

export function stopInstance(containerId) {
  return request(`/instances/${containerId}/stop`, {
    method: "POST",
  });
}

export function deleteInstance(containerId) {
  return request(`/instances/${containerId}`, {
    method: "DELETE",
  });
}
