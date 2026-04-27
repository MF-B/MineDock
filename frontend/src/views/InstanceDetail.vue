<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { useConsole } from "../composables/useConsole";
import { useContainerStore } from "../stores/containers";
import InstanceConfig from "./InstanceConfig.vue";
import InstanceFiles from "./InstanceFiles.vue";

type DetailTab = "console" | "config" | "files";

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

const canAttach = computed(() => {
  return currentInstance.value ? store.isRunning(currentInstance.value.status) : false;
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
  if (!canAttach.value) {
    return "console.instanceNotRunning";
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
  [loading, canAttach, activeTab],
  ([isLoading, running, tab]) => {
    if (isLoading || tab !== "console" || !running) {
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
      <button class="back-btn" @click="backToList">&lt;</button>
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
    </nav>

    <section class="tab-content">
      <section v-if="activeTab === 'console'" class="terminal-panel">
        <div v-if="infoKey" class="panel-overlay">
          {{ $t(infoKey) }}
        </div>
        <div v-else ref="terminalRef" class="terminal-host"></div>
      </section>

      <InstanceConfig v-else-if="activeTab === 'config'" :container-id="containerId" @reconfigured="handleReconfigured" />
      
      <InstanceFiles v-else :container-id="containerId" />
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
  border: 1px solid var(--create-border-outer);
  background: rgba(0, 0, 0, 0.25);
  color: var(--create-brass-primary);
  padding: 6px 12px;
  border-radius: 4px;
  cursor: pointer;
  font-weight: bold;
}

.back-btn:hover {
  background: rgba(0, 0, 0, 0.4);
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
  gap: 12px;
}

.tab-nav {
  display: flex;
  gap: 8px;
  border-bottom: 1px solid var(--create-border-outer);
  padding-bottom: 2px;
}

.tab-btn {
  border: 1px solid transparent;
  border-radius: 6px 6px 0 0;
  background: transparent;
  color: var(--text-muted);
  padding: 8px 14px;
  cursor: pointer;
  font-size: 13px;
  transition: all 0.2s ease;
}

.tab-btn:hover {
  color: var(--create-brass-primary);
  background: rgba(0, 0, 0, 0.2);
}

.tab-btn.is-active {
  color: var(--create-brass-primary);
  border-color: var(--create-border-outer);
  border-bottom-color: transparent;
  background: rgba(0, 0, 0, 0.3);
  box-shadow: inset 0 -2px 0 var(--create-brass-primary);
}

.tab-content {
  min-height: 0;
}

.terminal-panel {
  min-height: 0;
  height: 100%;
  position: relative;
  border: 1px solid var(--create-border-outer);
  background: radial-gradient(circle at top, rgba(33, 55, 40, 0.9), rgba(9, 12, 11, 0.95));
  border-radius: 6px;
  overflow: hidden;
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
  border: 1px solid var(--create-border-outer);
  background: rgba(0, 0, 0, 0.28);
  border-radius: 6px;
  padding: 10px 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.error-text {
  color: var(--danger);
  font-size: 13px;
  text-align: right;
}

@media (max-width: 767px) {
  .page-header {
    padding: 0 80px 0 52px; /* 避开左边栏悬浮按钮和右边栏操作区 */
    grid-template-columns: 1fr auto 1fr;
    height: var(--header-height);
  }

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
