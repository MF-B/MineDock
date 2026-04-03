import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import { ref, type Ref } from "vue";
import { consoleWsUrl } from "../api/index";

type Disposable = { dispose: () => void };

export function useConsole(containerId: Ref<string>): {
  terminalRef: Ref<HTMLElement | null>;
  connected: Ref<boolean>;
  error: Ref<string | null>;
  init: () => void;
  dispose: () => void;
} {
  const terminalRef = ref<HTMLElement | null>(null);
  const connected = ref(false);
  const error = ref<string | null>(null);

  let terminal: Terminal | null = null;
  let fitAddon: FitAddon | null = null;
  let inputRelay: Disposable | null = null;
  let socket: WebSocket | null = null;
  let resizeObserver: ResizeObserver | null = null;
  let disposed = true;
  const encoder = new TextEncoder();

  const closeSocket = (): void => {
    if (!socket) {
      return;
    }

    socket.onopen = null;
    socket.onclose = null;
    socket.onerror = null;

    if (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING) {
      socket.close(1000, "dispose");
    }

    socket = null;
  };

  const dispose = (): void => {
    disposed = true;
    connected.value = false;

    if (resizeObserver !== null) {
      resizeObserver.disconnect();
      resizeObserver = null;
    }

    closeSocket();

    inputRelay?.dispose();
    inputRelay = null;

    fitAddon?.dispose();
    fitAddon = null;

    terminal?.dispose();
    terminal = null;
  };

  const init = (): void => {
    const host = terminalRef.value;
    if (!host || !containerId.value) {
      connected.value = false;
      error.value = "console.mountMissing";
      return;
    }

    dispose();
    disposed = false;
    connected.value = false;
    error.value = null;

    terminal = new Terminal({
      cursorBlink: true,
      convertEol: true,
      fontFamily: '"JetBrains Mono", "Cascadia Code", monospace',
      fontSize: 14,
      theme: {
        background: "#0f1411",
        foreground: "#9ee38f",
        cursor: "#f5cb6e",
      },
    });

    fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(host);
    fitAddon.fit();

    resizeObserver = new ResizeObserver(() => {
      fitAddon?.fit();
    });
    resizeObserver.observe(host);

    socket = new WebSocket(consoleWsUrl(containerId.value));
    socket.binaryType = "arraybuffer";

    socket.onmessage = (event: MessageEvent<string | ArrayBuffer | Blob>) => {
      if (disposed) {
        return;
      }

      if (typeof event.data === "string") {
        terminal?.write(event.data);
        return;
      }

      if (event.data instanceof ArrayBuffer) {
        terminal?.write(new Uint8Array(event.data));
        return;
      }

      void event.data
        .arrayBuffer()
        .then((buffer) => {
          if (disposed) {
            return;
          }
          terminal?.write(new Uint8Array(buffer));
        })
        .catch(() => {
          // ignore blob decode errors from unexpected payloads
        });
    };

    inputRelay = terminal.onData((data: string) => {
      if (!socket || socket.readyState !== WebSocket.OPEN) {
        return;
      }

      const bytes = encoder.encode(data);
      if (bytes.length > 0) {
        socket.send(bytes);
      }
    });

    socket.onopen = () => {
      if (disposed) {
        return;
      }
      connected.value = true;
      error.value = null;
      fitAddon?.fit();
      terminal?.focus();
    };

    socket.onerror = () => {
      if (disposed) {
        return;
      }
      connected.value = false;
      if (!error.value) {
        error.value = "console.connectionError";
      }
    };

    socket.onclose = (event: CloseEvent) => {
      if (disposed) {
        return;
      }
      connected.value = false;
      if (event.reason) {
        error.value = event.reason;
        return;
      }
      if (event.code !== 1000) {
        error.value = "console.disconnected";
      }
    };
  };

  return {
    terminalRef,
    connected,
    error,
    init,
    dispose,
  };
}
