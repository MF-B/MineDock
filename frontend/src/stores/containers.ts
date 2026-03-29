import { ref } from "vue";
import { defineStore } from "pinia";
import type { Instance } from "../api/index";
import {
  listInstances,
  createInstance as apiCreate,
  deleteInstance as apiDelete,
  startInstance as apiStart,
  stopInstance as apiStop,
} from "../api/index";

export const useContainerStore = defineStore("containers", () => {
  const instances = ref<Instance[]>([]);
  const output = ref<string>("");

  function getErrorMessage(error: unknown): string {
    if (error instanceof Error) return error.message;
    if (typeof error === "string") return error;
    if (error && typeof error === "object") {
      const maybeMessage = (error as { message?: unknown }).message;
      if (typeof maybeMessage === "string" && maybeMessage.trim().length > 0) {
        return maybeMessage;
      }
      try {
        return JSON.stringify(error);
      } catch {
        return String(error);
      }
    }
    return String(error);
  }

  function print(data: unknown): void {
    output.value = typeof data === "string" ? data : JSON.stringify(data, null, 2);
  }

  function printError(error: unknown): void {
    output.value = `ERROR: ${getErrorMessage(error)}`;
  }

  async function fetchInstances(): Promise<boolean> {
    try {
      const data = await listInstances();
      instances.value = data;
      return true;
    } catch (err) {
      printError(err);
      return false;
    }
  }

  async function create(name: string): Promise<boolean> {
    try {
      const data = await apiCreate(name);
      print(data);
      await fetchInstances();
      return true;
    } catch (err) {
      printError(err);
      return false;
    }
  }

  async function remove(containerId: string): Promise<void> {
    try {
      const data = await apiDelete(containerId);
      print(data);
      await fetchInstances();
    } catch (err) {
      printError(err);
    }
  }

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
    output,
    print,
    printError,
    fetchInstances,
    create,
    remove,
    toggle,
    isRunning,
  };
});
