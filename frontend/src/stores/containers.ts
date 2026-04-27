import { ref } from "vue";
import { defineStore } from "pinia";
import type { Instance } from "../api/index";
import {
  ApiRequestError,
  listInstances,
  createInstance as apiCreate,
  type PortMapping,
  type ResourceLimits,
  deleteInstance as apiDelete,
  startInstance as apiStart,
  stopInstance as apiStop,
} from "../api/index";

type OutputI18nPayload = {
  key: string;
  values?: Record<string, string | number>;
};

const backendMessageKeyMap: Record<string, string> = {
  "name is required": "status.emptyName",
  "game_id is required": "errors.gameIdRequired",
  "game not found": "errors.gameNotFound",
  "invalid params": "errors.invalidParams",
  "invalid json body": "errors.invalidJsonBody",
  "invalid container id": "errors.invalidContainerId",
  "instance name already exists": "errors.instanceNameExists",
  "instance is running, stop it before delete": "errors.instanceRunning",
  "container must be stopped to update config": "errors.containerNotStopped",
  "invalid resource limits": "errors.invalidResourceLimits",
};

function mapBackendMessageToKey(message: string): string | undefined {
  const normalized = message.trim().toLowerCase();
  const exact = backendMessageKeyMap[normalized];
  if (exact) {
    return exact;
  }

  if (normalized.includes("invalid params")) {
    return "errors.invalidParams";
  }
  if (normalized.includes("game not found")) {
    return "errors.gameNotFound";
  }
  if (normalized.includes("template not found")) {
    return "errors.templateNotFound";
  }
  if (normalized.includes("invalid template")) {
    return "errors.templateInvalid";
  }

  return undefined;
}

function isLikelyI18nKey(value: string): boolean {
  return /^[a-z][a-z0-9_-]*(?:\.[a-zA-Z0-9_-]+)+$/.test(value);
}

export const useContainerStore = defineStore("containers", () => {
  const instances = ref<Instance[]>([]);
  const wsConnected = ref(false);
  // 统一输出区支持纯文本和 i18n key 两种模式，视图层只负责渲染。
  const output = ref<string>("");
  const outputI18n = ref<OutputI18nPayload | null>(null);

  function print(data: unknown): void {
    outputI18n.value = null;
    output.value = typeof data === "string" ? data : JSON.stringify(data, null, 2);
  }

  function printI18n(key: string, values?: Record<string, string | number>): void {
    outputI18n.value = { key, values };
    output.value = "";
  }

  function mapStatusToError(status?: number): OutputI18nPayload {
    switch (status) {
      case 400:
        return { key: "errors.badRequest" };
      case 404:
        return { key: "errors.notFound" };
      case 409:
        return { key: "errors.conflict" };
      case 500:
        return { key: "errors.internal" };
      default:
        if (typeof status === "number") {
          return { key: "errors.requestFailedWithStatus", values: { status } };
        }
        return { key: "errors.unknown" };
    }
  }

  // 统一将底层异常映射为稳定的 i18n key，避免泄露网络/后端原始文案。
  function mapErrorToI18n(error: unknown): OutputI18nPayload {
    if (error instanceof ApiRequestError) {
      if (error.backendMessage) {
        const mappedKey = mapBackendMessageToKey(error.backendMessage);
        if (mappedKey) {
          return { key: mappedKey };
        }
      }

      if (error.key === "errors.network") {
        return { key: "errors.network" };
      }

      if (error.key === "errors.httpStatus") {
        return mapStatusToError(error.status);
      }

      if (isLikelyI18nKey(error.key)) {
        return { key: error.key };
      }
    }

    if (typeof error === "string" && isLikelyI18nKey(error)) {
      return { key: error };
    }

    if (error instanceof Error && isLikelyI18nKey(error.message)) {
      return { key: error.message };
    }

    return { key: "errors.unknown" };
  }

  function printError(error: unknown): void {
    const mapped = mapErrorToI18n(error);
    printI18n(mapped.key, mapped.values);
  }

  function printErrorKey(key: string, values?: Record<string, string | number>): void {
    printI18n(key, values);
  }

  // 保持列表顺序稳定：优先沿用当前显示顺序，新增项再按名称/ID 排序追加。
  function normalizeInstances(nextInstances: Instance[]): Instance[] {
    const currentOrder = new Map(instances.value.map((item, index) => [item.container_id, index]));
    return [...nextInstances].sort((a, b) => {
      const aIndex = currentOrder.get(a.container_id);
      const bIndex = currentOrder.get(b.container_id);
      if (typeof aIndex === "number" && typeof bIndex === "number") {
        return aIndex - bIndex;
      }
      if (typeof aIndex === "number") {
        return -1;
      }
      if (typeof bIndex === "number") {
        return 1;
      }
      const nameOrder = a.name.localeCompare(b.name);
      if (nameOrder !== 0) {
        return nameOrder;
      }
      return a.container_id.localeCompare(b.container_id);
    });
  }

  // 同步后端实例列表到全局状态，返回值供视图层决定后续提示文案。
  async function fetchInstances(): Promise<boolean> {
    try {
      const data = await listInstances();
      instances.value = normalizeInstances(data);
      return true;
    } catch (err) {
      printError(err);
      return false;
    }
  }

  // WebSocket 推送的全量快照直接覆盖本地列表，避免再次触发 HTTP 拉取。
  function applySnapshot(nextInstances: Instance[]): void {
    instances.value = normalizeInstances(nextInstances);
  }

  function setWsConnected(connected: boolean): void {
    wsConnected.value = connected;
  }

  // 创建实例后立即刷新列表，避免视图层维护后端数据副本。
  async function create(
    name: string,
    gameId: string,
    params: Record<string, string> = {},
    ports: PortMapping[] = [],
    resources?: ResourceLimits,
  ): Promise<boolean> {
    try {
      const data = await apiCreate(name, gameId, params, ports, resources);
      print(data);
      await fetchInstances();
      return true;
    } catch (err) {
      printError(err);
      return false;
    }
  }

  // 删除结果与刷新都在 store 内完成，破坏性确认由视图层负责。
  async function remove(containerId: string): Promise<void> {
    try {
      const data = await apiDelete(containerId);
      print(data);
      await fetchInstances();
    } catch (err) {
      printError(err);
    }
  }

  // start/stop 统一走一个 action，并在 finally 刷新以消除状态漂移。
  async function toggle(instance: Instance): Promise<boolean> {
    const running = isRunning(instance.status);
    let success = true;
    try {
      if (running) {
        await apiStop(instance.container_id);
      } else {
        await apiStart(instance.container_id);
      }
    } catch (err) {
      printError(err);
      success = false;
    } finally {
      await fetchInstances();
    }
    return success;
  }

  function isRunning(status: string | undefined): boolean {
    if (!status) return false;
    return status.toLowerCase().startsWith("up") || status.toLowerCase() === "running";
  }

  return {
    instances,
    wsConnected,
    output,
    outputI18n,
    print,
    printError,
    printErrorKey,
    applySnapshot,
    setWsConnected,
    fetchInstances,
    create,
    remove,
    toggle,
    isRunning,
  };
});
