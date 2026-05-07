<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { useConsole } from "../composables/useConsole";
import { useContainerStore } from "../stores/containers";
import InstanceConfig from "./InstanceConfig.vue";
import InstanceFiles from "./InstanceFiles.vue";
import InstanceMonitor from "./InstanceMonitor.vue";

type DetailTab = "console" | "config" | "files" | "monitor";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const store = useContainerStore();

const loading = ref(true);
const containerId = ref("");
const activeTab = ref<DetailTab>("console");

const { terminalRef, error, init, dispose } = useConsole(containerId);

const currentInstance = computed(() => {
  return store.instances.find((item) => item.container_id === containerId.value) ?? null;
});

const containerName = computed(() => {
  return currentInstance.value?.name || containerId.value;
});

const canOpenConsole = computed(() => {
  return Boolean(containerId.value && currentInstance.value);
});

const infoKey = computed<string | null>(() => {
  if (!containerId.value) {
    return "errors.invalidContainerId";
  }
  if (loading.value) {
    return "console.loading";
  }
  if (!currentInstance.value) {
    return "console.instanceNotFound";
  }
  return null;
});

const displayError = computed(() => {
  if (!error.value) {
    return "";
  }
  if (error.value.includes("container is not running")) {
    return t("console.instanceNotRunning");
  }
  if (error.value.includes("no such container")) {
    return t("console.instanceNotFound");
  }
  if (error.value.startsWith("console.")) {
    return t(error.value);
  }
  return error.value;
});

function parseContainerID(value: unknown): string {
  if (Array.isArray(value)) {
    return typeof value[0] === "string" ? value[0].trim() : "";
  }
  return typeof value === "string" ? value.trim() : "";
}

async function loadInstance(): Promise<void> {
  loading.value = true;
  dispose();

  containerId.value = parseContainerID(route.params.id);
  if (!containerId.value) {
    loading.value = false;
    return;
  }

  await store.fetchInstances();
  loading.value = false;
}

function switchTab(tab: DetailTab): void {
  activeTab.value = tab;
}

function handleReconfigured(newContainerID: string): void {
  const normalized = parseContainerID(newContainerID);
  if (!normalized || normalized === containerId.value) {
    return;
  }
  void router.push({ name: "InstanceDetail", params: { id: normalized } });
}

function backToList(): void {
  void router.push({ name: "ContainerList" });
}

watch(
  () => route.params.id,
  () => {
    void loadInstance();
  },
  { immediate: true },
);

watch(
  [loading, canOpenConsole, activeTab],
  ([isLoading, hasConsoleTarget, tab]) => {
    if (isLoading || tab !== "console" || !hasConsoleTarget) {
      dispose();
      return;
    }
    void nextTick(() => {
      init();
    });
  },
  { immediate: true },
);

onUnmounted(() => {
  dispose();
});
</script>

<template>
  <header class="page-header">
    <div class="header-left">
      <button class="back-btn" title="Back" @click="backToList">
        <svg
          viewBox="0 0 24 24"
          width="18"
          height="18"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="square"
          stroke-linejoin="miter"
        >
          <path d="M19 12H5M12 19l-7-7 7-7" />
        </svg>
      </button>
    </div>
    <h1 class="page-title">{{ containerName }}</h1>
    <div class="header-right"></div>
  </header>

  <main class="main-content">
    <nav class="tab-nav" role="tablist" aria-label="Instance detail tabs">
      <button
        class="tab-btn"
        :class="{ 'is-active': activeTab === 'console' }"
        role="tab"
        type="button"
        :aria-selected="activeTab === 'console'"
        @click="switchTab('console')"
      >
        {{ $t("tabs.console") }}
      </button>
      <button
        class="tab-btn"
        :class="{ 'is-active': activeTab === 'config' }"
        role="tab"
        type="button"
        :aria-selected="activeTab === 'config'"
        @click="switchTab('config')"
      >
        {{ $t("tabs.config") }}
      </button>
      <button
        class="tab-btn"
        :class="{ 'is-active': activeTab === 'files' }"
        role="tab"
        type="button"
        :aria-selected="activeTab === 'files'"
        @click="switchTab('files')"
      >
        {{ $t("tabs.files") }}
      </button>
      <button
        class="tab-btn"
        :class="{ 'is-active': activeTab === 'monitor' }"
        role="tab"
        type="button"
        :aria-selected="activeTab === 'monitor'"
        @click="switchTab('monitor')"
      >
        {{ $t("tabs.monitor") }}
      </button>
    </nav>

    <section class="tab-content">
      <section v-if="activeTab === 'console'" class="terminal-panel">
        <div v-if="infoKey" class="panel-overlay">
          {{ $t(infoKey) }}
        </div>
        <div v-else ref="terminalRef" class="terminal-host"></div>
      </section>

      <InstanceConfig
        v-else-if="activeTab === 'config'"
        :container-id="containerId"
        @reconfigured="handleReconfigured"
      />

      <InstanceFiles v-else-if="activeTab === 'files'" :container-id="containerId" />

      <InstanceMonitor v-else-if="activeTab === 'monitor'" :container-id="containerId" />
    </section>

    <footer v-if="activeTab === 'console' && displayError" class="status-bar">
      <div class="error-text">{{ displayError }}</div>
    </footer>
  </main>
</template>

<style scoped>
.page-header {
  height: var(--header-height);
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  gap: 12px;
  padding: 0 80px 0 24px; /* 右侧留给 Top actions，左侧常规 24px (桌面汉堡在自身 sidebar 中，不挤占 header) */
  flex-shrink: 0;
}

.header-left {
  justify-self: start;
}

.header-right {
  justify-self: end;
}

.back-btn {
  border: 2px solid var(--card-border);
  background: var(--card-bg);
  color: var(--card-text);
  padding: 6px 12px;
  border-radius: 0;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 2px 2px 0 0 var(--create-border-outer);
  transition: all 0.2s ease;
}

.back-btn:hover {
  transform: translate(1px, 1px);
  box-shadow: 1px 1px 0 0 var(--create-border-outer);
  background: var(--hover-lighten);
}

.page-title {
  margin: 0;
  color: var(--create-brass-primary);
  font-size: 16px;
  letter-spacing: 1px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: center;
}

.main-content {
  flex: 1;
  min-height: 0;
  max-width: 1200px;
  width: 100%;
  margin: 0 auto;
  padding: 8px 24px 24px;
  display: grid;
  grid-template-rows: auto 1fr auto;
}

.tab-nav {
  display: flex;
  gap: 4px;
  border-bottom: 3px solid var(--card-border);
  padding-bottom: 0;
  padding-left: 8px;
}

.tab-btn {
  border: 3px solid var(--card-border);
  border-bottom: none;
  border-radius: 0;
  background: var(--create-brass-dark);
  color: var(--card-text);
  padding: 8px 18px;
  cursor: pointer;
  font-size: 13px;
  font-weight: bold;
  transition: all 0.2s ease;
  margin-bottom: -3px; /* overlap bottom border */
  box-shadow: inset 0 -4px 0 0 rgba(0, 0, 0, 0.1);
}

.tab-btn:hover {
  background: var(--create-brass-secondary);
}

.tab-btn.is-active {
  color: var(--card-text);
  background: var(--card-bg);
  box-shadow: none;
  border-bottom: 3px solid var(--card-bg);
}

.tab-content {
  min-height: 0;
  flex: 1;
  background: var(--card-bg);
  border: 3px solid var(--card-border);
  border-top: none;
  border-radius: 0;
  display: flex;
  flex-direction: column;
  padding: 16px;
  box-shadow:
    inset 0 3px 0 0 var(--card-bg),
    inset 0 -3px 0 0 var(--card-bg),
    inset 0 6px 0 0 var(--card-border-inner),
    inset 0 -6px 0 0 var(--card-border-inner);
}

.terminal-panel {
  min-height: 0;
  flex: 1;
  position: relative;
  border: 3px solid var(--card-border);
  background: radial-gradient(circle at top, #213728, #090c0b);
  border-radius: 0;
  overflow: hidden;
  box-shadow: inset 0 0 10px rgba(0, 0, 0, 0.8);
}

.terminal-host {
  width: 100%;
  height: 100%;
  padding: 0;
}

.panel-overlay {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--create-brass-primary);
  background: rgba(0, 0, 0, 0.35);
  font-size: 15px;
}

.status-bar {
  border: 3px solid var(--card-border);
  background: var(--card-bg);
  border-radius: 0;
  padding: 10px 12px;
  margin-top: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  box-shadow:
    inset 0 3px 0 0 var(--card-bg),
    inset 0 -3px 0 0 var(--card-bg),
    inset 0 6px 0 0 var(--card-border-inner),
    inset 0 -6px 0 0 var(--card-border-inner);
}

.error-text {
  color: var(--danger);
  font-size: 13px;
  text-align: right;
  font-weight: 500;
}

@media (max-width: 1023px) {
  .page-header {
    padding: 0 80px 0 64px;
    grid-template-columns: 1fr auto 1fr;
    height: var(--header-height);
  }
}

@media (max-width: 767px) {
  .main-content {
    padding: 8px 12px 12px;
  }

  .tab-btn {
    padding: 8px 12px;
  }

  .status-bar {
    flex-direction: column;
    align-items: flex-start;
  }

  .error-text {
    text-align: left;
  }
}
</style>
