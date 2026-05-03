<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { getSystemLogs, type SystemLogEntry } from "../api/index";
import { useToastStore } from "../stores/toasts";

const toasts = useToastStore();

const entries = ref<SystemLogEntry[]>([]);
const logPath = ref("");
const loading = ref(false);
const loadError = ref(false);
const activeLevels = ref<Set<string>>(new Set(["debug", "info", "warn", "error"]));
const query = ref("");

let refreshTimer: ReturnType<typeof setInterval> | null = null;

const levels = ["debug", "info", "warn", "error"];

const logListRef = ref<HTMLElement | null>(null);

function formatTime(value: string): string {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  const pad = (n: number, l = 2) => String(n).padStart(l, "0");
  const YYYY = date.getFullYear();
  const MM = pad(date.getMonth() + 1);
  const DD = pad(date.getDate());
  const HH = pad(date.getHours());
  const mm = pad(date.getMinutes());
  const ss = pad(date.getSeconds());
  const SSS = pad(date.getMilliseconds(), 3);

  return `${YYYY}-${MM}-${DD} ${HH}:${mm}:${ss}.${SSS}`;
}

function formatMessage(entry: SystemLogEntry): string {
  if (entry.message === "http request" && entry.attributes) {
    const { method, path, status, duration_ms } = entry.attributes;
    return `[HTTP] ${method} ${path} -> ${status} (${duration_ms}ms)`;
  }
  return entry.message || "";
}

async function loadLogs(showToast = false): Promise<void> {
  loading.value = true;
  loadError.value = false;
  try {
    const levelParam = Array.from(activeLevels.value).join(",");
    const data = await getSystemLogs(500, levelParam, query.value.trim());

    // Check if we are at the bottom before updating
    const el = logListRef.value;
    let isAtBottom = true;
    if (el) {
      isAtBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 10;
    }

    entries.value = data.entries;
    logPath.value = data.path;

    if (showToast) {
      toasts.pushI18n("success", "systemLogs.refresh");
    }

    // Scroll to bottom if we were at the bottom
    if (isAtBottom && el) {
      setTimeout(() => {
        el.scrollTop = el.scrollHeight;
      }, 0);
    }
  } catch {
    loadError.value = true;
    toasts.pushI18n("error", "systemLogs.loadError");
  } finally {
    loading.value = false;
  }
}

function restartTimer(): void {
  if (refreshTimer !== null) {
    clearInterval(refreshTimer);
    refreshTimer = null;
  }
  refreshTimer = setInterval(() => {
    void loadLogs();
  }, 5000);
}

function handleRefresh(): void {
  void loadLogs(false); // No toast for silent refresh
}

function toggleLevel(item: string): void {
  if (activeLevels.value.has(item)) {
    // Prevent unchecking the last one
    if (activeLevels.value.size > 1) {
      activeLevels.value.delete(item);
      handleRefresh();
    }
  } else {
    activeLevels.value.add(item);
    handleRefresh();
  }
}

onMounted(() => {
  void loadLogs();
  restartTimer();
});

onUnmounted(() => {
  if (refreshTimer !== null) {
    clearInterval(refreshTimer);
  }
});
</script>

<template>
  <header class="page-header">
    <h1 class="page-title">{{ $t("systemLogs.title") }}</h1>
  </header>

  <main class="main-content">
    <section class="toolbar" aria-label="System log filters">
      <div class="level-tags">
        <button
          v-for="item in levels"
          :key="item"
          type="button"
          class="level-tag"
          :class="{ 'is-active': activeLevels.has(item) }"
          @click="toggleLevel(item)"
        >
          <span class="tag-icon" :style="{ opacity: activeLevels.has(item) ? 1 : 0 }">✓</span>
          {{ item.toUpperCase() }}
        </button>
      </div>

      <label class="control control-search">
        <span class="visually-hidden">{{ $t("systemLogs.queryPlaceholder") }}</span>
        <input
          v-model="query"
          type="search"
          :placeholder="$t('systemLogs.queryPlaceholder')"
          @keyup.enter="handleRefresh"
        />
      </label>
    </section>

    <p v-if="loadError" class="state-message state-error">{{ $t("systemLogs.loadError") }}</p>

    <section class="log-panel" aria-live="polite">
      <div v-if="!loading && entries.length === 0" class="empty-state">
        {{ $t("systemLogs.empty") }}
      </div>

      <div v-else ref="logListRef" class="log-list">
        <div
          v-for="entry in entries"
          :key="`${entry.time}-${entry.level}-${entry.message}-${entry.raw}`"
          class="log-line"
          :class="`level-${(entry.level || 'info').toLowerCase()}`"
        >
          <span class="log-tag">[{{ formatTime(entry.time) }}]</span>
          <span class="log-tag">[Core]</span>
          <span class="log-tag"
            >[{{ (entry.level || "INFO").padEnd(4, " ").substring(0, 4).toUpperCase() }}]</span
          >
          <span class="log-colon">: </span>
          <span class="log-message">{{ formatMessage(entry) }}</span>
        </div>
      </div>
    </section>
  </main>
</template>

<style scoped>
.page-header {
  height: var(--header-height);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  padding: 0 80px 0 24px;
}

.page-title {
  margin: 0;
  color: var(--create-brass-primary);
  font-size: 16px;
  font-weight: bold;
  letter-spacing: 1px;
}

.main-content {
  flex: 1;
  min-height: 0;
  max-width: 1200px;
  width: 100%;
  margin: 0 auto;
  padding: 8px 24px 24px;
  display: grid;
  grid-template-rows: auto auto auto 1fr;
  gap: 12px;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding: 12px;
  background: var(--card-bg);
  border: 3px solid var(--card-border);
  box-shadow:
    inset 0 3px 0 0 var(--card-bg),
    inset 0 -3px 0 0 var(--card-bg),
    inset 0 6px 0 0 var(--card-border-inner),
    inset 0 -6px 0 0 var(--card-border-inner);
}

.level-tags {
  display: flex;
  gap: 8px;
  margin-right: auto;
}

.level-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 12px;
  border-radius: 0;
  background: var(--card-bg);
  color: var(--text-muted);
  border: 2px solid var(--card-border);
  font-size: 13px;
  font-weight: bold;
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
  box-shadow: 2px 2px 0 0 var(--create-border-outer);
}

.level-tag:hover {
  background: var(--hover-lighten);
  transform: translate(1px, 1px);
  box-shadow: 1px 1px 0 0 var(--create-border-outer);
}

.level-tag.is-active {
  background: var(--create-brass-dark);
  color: var(--card-text);
  box-shadow: inset 0 -3px 0 0 rgba(0, 0, 0, 0.15);
  transform: translate(1px, 1px);
}

.tag-icon {
  font-size: 14px;
  line-height: 1;
  width: 14px;
}

.control {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-on-dark);
  font-size: 13px;
  font-weight: bold;
}

.control-search {
  flex: 1;
  min-width: 220px;
}

.control-search input {
  width: 100%;
}

.control input,
.control select {
  height: 34px;
  border: 2px solid var(--card-border);
  background: var(--card-bg);
  color: var(--card-text);
  padding: 0 10px;
  font: inherit;
}

.log-path {
  margin: 0;
  color: var(--text-on-dark);
  font-size: 12px;
  overflow-wrap: anywhere;
}

.state-message,
.empty-state {
  padding: 18px;
  color: var(--text-muted);
  text-align: center;
}

.state-error {
  color: var(--danger);
}

.log-panel {
  min-height: 0;
  border: 3px solid var(--card-border);
  background: #0f1411;
  color: #c5d4cc;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: inset 0 0 10px rgba(0, 0, 0, 0.8);
}

.log-list {
  flex: 1;
  overflow: auto;
  font-family: "JetBrains Mono", "Cascadia Code", monospace;
  font-size: 13px;
  line-height: 1.6;
  padding: 16px;
}

.log-line {
  margin-bottom: 2px;
  word-break: break-all;
  padding: 0 4px;
}

.log-line:hover {
  background: rgba(255, 255, 255, 0.05);
}

.log-tag {
  display: inline-block;
  margin-right: 6px;
  white-space: nowrap;
}

.log-colon {
  margin-right: 6px;
}

.log-message {
  white-space: pre-wrap;
}

/* Base level styling */
.level-info {
  color: #00e5e5;
}

.level-warn {
  color: #ffff00;
}

.level-error {
  color: #ff4d4d;
  background: rgba(232, 106, 106, 0.08);
}
.level-error:hover {
  background: rgba(232, 106, 106, 0.15);
}

.level-debug {
  color: #a0a0a0;
}

.log-attributes {
  display: none;
}

.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

@media (max-width: 900px) {
  .main-content {
    padding: 8px 12px 12px;
  }
}
</style>
