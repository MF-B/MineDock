<script setup lang="ts">
import { computed, ref, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { useContainerStore } from "../stores/containers";

const { t } = useI18n();
const store = useContainerStore();

const showCreateModal = ref(false);
const newContainerName = ref("");

const outputText = computed(() => {
  if (store.outputI18n) {
    return t(store.outputI18n.key, store.outputI18n.values ?? {});
  }
  return store.output;
});

onMounted(() => {
  void initializeList();
});

// 页面启动时统一触发列表拉取，并输出首屏可读状态。
async function initializeList(): Promise<void> {
  store.print(t("status.waiting"));
  const success = await store.fetchInstances();
  if (!success) return;

  if (store.instances.length > 0) {
    store.print(t("status.listRefreshed"));
  } else {
    store.print(t("status.noContainers"));
  }
}

// 视图层仅做输入校验和 action 触发，副作用与错误收敛在 store 内完成。
async function handleCreate(): Promise<void> {
  const trimmed = newContainerName.value.trim();
  if (!trimmed) {
    store.printErrorKey("status.emptyName");
    return;
  }

  store.print(t("status.creating"));
  const success = await store.create(trimmed);
  if (success) {
    showCreateModal.value = false;
    newContainerName.value = "";
  }
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
  <!-- 顶部栏 -->
  <header class="page-header">
    <h1 class="page-title">{{ $t("containers.title") }}</h1>
  </header>

  <main class="main-content">
    <!-- 列表操作栏 -->
    <div class="content-actions">
      <button class="create-btn" @click="showCreateModal = true">
        {{ $t("containers.createBtn") }}
      </button>
    </div>
    <!-- 卡片列表布局 -->
    <div class="card-list">
      <div v-if="store.instances.length === 0" class="empty-state">
        {{ $t("containers.emptyState") }}
      </div>

      <div v-for="item in store.instances" :key="item.container_id" class="card">
        <!-- 左侧：容器名称 -->
        <div class="card-left">
          <span class="card-name">{{ item.name }}</span>
        </div>
        <!-- 右侧：控制按钮（拉杆与删除） -->
        <div class="card-right">
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

    <!-- 新建容器的弹窗 (Modal) -->
    <div v-if="showCreateModal" class="modal">
      <div class="modal-content">
        <h3>{{ $t("createModal.title") }}</h3>
        <input
          v-model="newContainerName"
          :placeholder="$t('createModal.placeholder')"
          @keyup.enter="handleCreate"
        />
        <div class="modal-actions">
          <button class="btn-cancel" @click="showCreateModal = false">
            {{ $t("createModal.cancel") }}
          </button>
          <button class="btn-confirm" @click="handleCreate">{{ $t("createModal.confirm") }}</button>
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
}

.page-title {
  margin: 0;
  color: var(--create-brass-primary);
  font-size: 16px;
  font-weight: bold;
  letter-spacing: 2px;
  font-family: "Segoe UI", "PingFang SC", sans-serif;
}

/* 内容区操作栏与新建按钮 */
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
  padding: 8px 24px 24px 24px;
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

/* ========== 弹窗样式 ========== */
.modal {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  background: var(--modal-overlay);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1000;
}

.modal-content {
  background-color: var(--modal-bg);
  padding: 20px;
  border-radius: 8px;
  width: 360px;
  border: 1px solid var(--create-brass-primary);
  display: flex;
  flex-direction: column;
  gap: 12px;
  box-shadow: 0 8px 24px var(--shadow-medium);
}

.modal-content h3 {
  margin: 0;
  color: var(--create-brass-primary);
  font-size: 16px;
}

.modal-content input {
  padding: 8px 10px;
  background: var(--input-bg);
  border: 1px solid var(--input-border);
  color: var(--text-on-dark);
  border-radius: 4px;
  outline: none;
}

.modal-content input:focus {
  border-color: var(--create-brass-primary);
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 8px;
}

.btn-cancel {
  padding: 6px 12px;
  font-size: 13px;
  background: transparent;
  border: 1px solid var(--btn-secondary-border);
  color: var(--btn-secondary-text);
  border-radius: 4px;
  cursor: pointer;
}
.btn-cancel:hover {
  background: var(--hover-lighten);
}

.btn-confirm {
  padding: 6px 12px;
  font-size: 13px;
  background: var(--create-brass-dark);
  color: var(--text-on-dark);
  border: none;
  border-radius: 4px;
  cursor: pointer;
}
.btn-confirm:hover {
  filter: brightness(1.1);
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
