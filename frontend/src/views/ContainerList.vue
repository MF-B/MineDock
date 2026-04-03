<script setup lang="ts">
import { computed, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { useContainerStore } from "../stores/containers";
import { useInstanceSync } from "../composables/useInstanceSync";

const { t } = useI18n();
const router = useRouter();
const store = useContainerStore();
const instanceSync = useInstanceSync();

const outputText = computed(() => {
  if (store.outputI18n) {
    return t(store.outputI18n.key, store.outputI18n.values ?? {});
  }
  return store.output;
});

onMounted(() => {
  instanceSync.start();
  void initializeList();
});

onUnmounted(() => {
  instanceSync.stop();
});

async function initializeList(): Promise<void> {
  store.print(t("status.waiting"));
  const success = await store.fetchInstances();
  if (!success) {
    return;
  }

  if (store.instances.length > 0) {
    store.print(t("status.listRefreshed"));
  } else {
    store.print(t("status.noContainers"));
  }
}

function goToTemplateMarket(): void {
  void router.push({ name: "ImageRegistry" });
}

function openInstanceDetail(containerId: string): void {
  void router.push({ name: "InstanceDetail", params: { id: containerId } });
}

// 删除属于破坏性操作，执行前必须二次确认。
async function handleDelete(containerId: string): Promise<void> {
  if (!confirm(t("containers.confirmDelete"))) return;
  store.print(t("status.deleting"));
  await store.remove(containerId);
}

// 根据当前运行态切换 start/stop，并输出阶段性反馈以避免静默操作。
async function handleToggle(instance: {
  container_id: string;
  name: string;
  status: string;
}): Promise<void> {
  const running = store.isRunning(instance.status);
  if (running) {
    store.print(t("status.stopping", { name: instance.name }));
  } else {
    store.print(t("status.starting", { name: instance.name }));
  }
  const success = await store.toggle(instance);
  if (!success) return;
  if (running) {
    store.print(t("status.stopSuccess", { name: instance.name }));
  } else {
    store.print(t("status.startSuccess", { name: instance.name }));
  }
}
</script>

<template>
  <header class="page-header">
    <h1 class="page-title">{{ $t("containers.title") }}</h1>
  </header>

  <main class="main-content">
    <!-- 列表操作栏 -->
    <div class="content-actions">
      <button class="create-btn" @click="goToTemplateMarket">
        {{ $t("containers.createBtn") }}
      </button>
    </div>
    <!-- 卡片列表布局 -->
    <div class="card-list">
      <div v-if="store.instances.length === 0" class="empty-state">
        {{ $t("containers.emptyState") }}
      </div>

      <div
        v-for="item in store.instances"
        :key="item.container_id"
        class="card"
        @click="openInstanceDetail(item.container_id)"
      >
        <!-- 左侧：容器名称 -->
        <div class="card-left">
          <span class="card-name">{{ item.name }}</span>
        </div>
        <!-- 右侧：控制按钮（拉杆与删除） -->
        <div class="card-right" @click.stop>
          <label class="switch" :title="$t('containers.toggleTitle')">
            <input
              type="checkbox"
              :checked="store.isRunning(item.status)"
              @change="handleToggle(item)"
            />
            <span class="slider round"></span>
          </label>
          <button class="delete-btn" @click="handleDelete(item.container_id)">
            {{ $t("containers.delete") }}
          </button>
        </div>
      </div>
    </div>

    <!-- 用于显示接口返回的简易日志 -->
    <pre class="output">{{ outputText }}</pre>
  </main>
</template>

<style scoped>
.page-header {
  height: var(--header-height);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  position: relative;
  padding: 0 80px 0 24px; /* 为右侧透明的全局 TopBar 预留 80px 的事件不遮挡空间，左层 24px */
}

@media (max-width: 767px) {
  .page-header {
    padding: 0 80px 0 52px; /* 移动端左侧多让出 52px 的空间给绝对定位的 Hamburger，防止标题偏移碰撞 */
  }
}

.page-title {
  margin: 0;
  color: var(--create-brass-primary);
  font-size: 16px;
  font-weight: bold;
  letter-spacing: 2px;
  font-family: "Segoe UI", "PingFang SC", sans-serif;
}

.content-actions {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}

.create-btn {
  padding: 8px 16px;
  background-color: var(--create-brass-dark);
  color: var(--card-text);
  border: none;
  border-radius: 4px;
  font-size: 14px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.create-btn:hover {
  filter: brightness(1.1);
}

.main-content {
  padding: 0 24px 24px 24px;
  flex: 1;
  display: flex;
  flex-direction: column;
  max-width: 1200px;
  margin: 0 auto;
  width: 100%;
}

/* ========== 卡片列表 ========== */
.card-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-bottom: 24px;
}

.empty-state {
  text-align: center;
  color: var(--text-muted);
  padding: 40px;
  border: 1px dashed var(--border-muted);
  border-radius: 8px;
}

.card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 20px;
  background-color: var(--card-bg);
  border: 3px solid var(--card-border);
  box-shadow:
    inset 0 3px 0 0 var(--card-bg),
    inset 0 -3px 0 0 var(--card-bg),
    inset 0 6px 0 0 var(--card-border-inner),
    inset 0 -6px 0 0 var(--card-border-inner);
  border-radius: 0;
  clip-path: polygon(
    /* 左上角 */ 0 3px,
    3px 3px,
    3px 0,
    /* 右上角 */ calc(100% - 3px) 0,
    calc(100% - 3px) 3px,
    100% 3px,
    /* 右下角 */ 100% calc(100% - 3px),
    calc(100% - 3px) calc(100% - 3px),
    calc(100% - 3px) 100%,
    /* 左下角 */ 3px 100%,
    3px calc(100% - 3px),
    0 calc(100% - 3px)
  );
  transition: filter 0.2s;
}

.card:hover {
  filter: brightness(0.96);
}

.card {
  cursor: pointer;
}

.card-left {
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.card-name {
  color: var(--card-text);
  font-size: 16px;
  font-weight: bold;
}

.card-right {
  display: flex;
  align-items: center;
  gap: 20px;
}

/* ======= 右侧按钮组件 ======= */
.delete-btn {
  padding: 6px 16px;
  background-color: var(--danger-light);
  color: var(--danger);
  border: 1px solid var(--danger);
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s;
}

.delete-btn:hover {
  background-color: var(--danger);
  color: var(--text-on-dark);
}

/* ======= 拉杆样式的开关 ======= */
.switch {
  position: relative;
  display: inline-block;
  width: 48px;
  height: 24px;
}

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--toggle-off);
  transition: 0.4s;
}

.slider:before {
  position: absolute;
  content: "";
  height: 18px;
  width: 18px;
  left: 3px;
  bottom: 3px;
  background-color: white;
  transition: 0.4s;
}

input:checked + .slider {
  background-color: var(--toggle-on);
}

input:focus + .slider {
  box-shadow: 0 0 1px var(--toggle-on);
}

input:checked + .slider:before {
  transform: translateX(24px);
}

.slider.round {
  border-radius: 24px;
}
.slider.round:before {
  border-radius: 50%;
}

/* ========== 底部输出 ========== */
.output {
  margin-top: auto;
  padding: 12px;
  background: var(--output-bg);
  color: var(--output-text);
  border: 1px solid var(--output-border);
  border-radius: 4px;
  min-height: 80px;
  max-height: 160px;
  overflow-y: auto;
  word-wrap: break-word;
  white-space: pre-wrap;
  font-size: 13px;
}

/* ========== 响应式自适应 ========== */
@media (max-width: 1023px) {
  .page-title {
    display: none;
  }
}
</style>
