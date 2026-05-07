import { ref } from "vue";
import { defineStore } from "pinia";

export type ToastKind = "info" | "success" | "warning" | "error";

export interface ToastMessage {
  id: number;
  kind: ToastKind;
  text?: string;
  i18n?: {
    key: string;
    values?: Record<string, string | number>;
  };
}

interface ToastOptions {
  timeoutMs?: number;
}

export const useToastStore = defineStore("toasts", () => {
  const messages = ref<ToastMessage[]>([]);
  const timers = new Map<number, ReturnType<typeof setTimeout>>();
  let nextID = 1;

  function remove(id: number): void {
    const timer = timers.get(id);
    if (timer) {
      clearTimeout(timer);
      timers.delete(id);
    }
    messages.value = messages.value.filter((item) => item.id !== id);
  }

  function push(message: Omit<ToastMessage, "id">, options: ToastOptions = {}): number {
    const id = nextID++;
    const timeoutMs = options.timeoutMs ?? (message.kind === "error" ? 6000 : 3600);
    messages.value = [...messages.value, { ...message, id }];

    if (timeoutMs > 0) {
      timers.set(
        id,
        setTimeout(() => {
          remove(id);
        }, timeoutMs),
      );
    }

    return id;
  }

  function pushText(kind: ToastKind, text: string, options?: ToastOptions): number {
    return push({ kind, text }, options);
  }

  function pushI18n(
    kind: ToastKind,
    key: string,
    values?: Record<string, string | number>,
    options?: ToastOptions,
  ): number {
    return push({ kind, i18n: { key, values } }, options);
  }

  function clear(): void {
    for (const id of timers.keys()) {
      remove(id);
    }
    messages.value = [];
  }

  return { messages, push, pushText, pushI18n, remove, clear };
});
