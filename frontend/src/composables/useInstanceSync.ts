import { ref, type Ref } from "vue";
import { WS_BASE_URL, type WsMessage } from "../api/index";
import { useContainerStore } from "../stores/containers";

const POLL_INTERVAL_MS = 5000;
const INITIAL_RECONNECT_DELAY_MS = 1000;
const MAX_RECONNECT_DELAY_MS = 30000;

function isInstancesUpdatedMessage(payload: unknown): payload is WsMessage {
  if (!payload || typeof payload !== "object") {
    return false;
  }
  const message = payload as { type?: unknown; data?: unknown };
  return message.type === "instances_updated" && Array.isArray(message.data);
}

// useInstanceSync 管理与后端的 WebSocket 实时连接，
// 收到 instances_updated 消息时自动更新 container store。
// 连接失败或断开时降级为定时轮询。
export function useInstanceSync(): {
  connected: Ref<boolean>;
  start: () => void;
  stop: () => void;
} {
  const store = useContainerStore();
  const connected = ref(false);

  const wsURL = `${WS_BASE_URL}/ws/events`;
  let socket: WebSocket | null = null;
  let started = false;
  let reconnectDelay = INITIAL_RECONNECT_DELAY_MS;
  let reconnectTimer: ReturnType<typeof window.setTimeout> | null = null;
  let pollingTimer: ReturnType<typeof window.setInterval> | null = null;

  const setConnected = (value: boolean): void => {
    connected.value = value;
    store.setWsConnected(value);
  };

  const stopPolling = (): void => {
    if (pollingTimer !== null) {
      window.clearInterval(pollingTimer);
      pollingTimer = null;
    }
  };

  const startPolling = (): void => {
    if (pollingTimer !== null) {
      return;
    }
    pollingTimer = window.setInterval(() => {
      void store.fetchInstances();
    }, POLL_INTERVAL_MS);
  };

  const clearReconnectTimer = (): void => {
    if (reconnectTimer !== null) {
      window.clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
  };

  const closeSocket = (): void => {
    if (!socket) {
      return;
    }
    socket.onopen = null;
    socket.onclose = null;
    socket.onerror = null;
    socket.onmessage = null;
    if (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING) {
      socket.close();
    }
    socket = null;
  };

  const scheduleReconnect = (): void => {
    if (!started || reconnectTimer !== null) {
      return;
    }
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = null;
      connect();
    }, reconnectDelay);
    reconnectDelay = Math.min(reconnectDelay * 2, MAX_RECONNECT_DELAY_MS);
  };

  const handleDisconnected = (): void => {
    setConnected(false);
    startPolling();
    scheduleReconnect();
  };

  const connect = (): void => {
    if (!started) {
      return;
    }

    if (
      socket &&
      (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)
    ) {
      return;
    }

    let nextSocket: WebSocket;
    try {
      nextSocket = new WebSocket(wsURL);
    } catch {
      handleDisconnected();
      return;
    }

    socket = nextSocket;

    nextSocket.onopen = () => {
      reconnectDelay = INITIAL_RECONNECT_DELAY_MS;
      clearReconnectTimer();
      stopPolling();
      setConnected(true);
      void store.fetchInstances();
    };

    nextSocket.onmessage = (event: MessageEvent): void => {
      if (typeof event.data !== "string") {
        return;
      }
      let parsed: unknown;
      try {
        parsed = JSON.parse(event.data);
      } catch {
        return;
      }
      if (!isInstancesUpdatedMessage(parsed)) {
        return;
      }
      store.applySnapshot(parsed.data);
    };

    nextSocket.onerror = () => {
      nextSocket.close();
    };

    nextSocket.onclose = () => {
      if (socket === nextSocket) {
        socket = null;
      }
      if (!started) {
        return;
      }
      handleDisconnected();
    };
  };

  const start = (): void => {
    if (started) {
      return;
    }
    started = true;
    reconnectDelay = INITIAL_RECONNECT_DELAY_MS;
    clearReconnectTimer();
    setConnected(false);
    connect();
  };

  const stop = (): void => {
    if (!started) {
      return;
    }
    started = false;
    clearReconnectTimer();
    stopPolling();
    setConnected(false);
    closeSocket();
  };

  return { connected, start, stop };
}
