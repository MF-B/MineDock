<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { useContainerStore } from "../stores/containers";
import { useInstanceSync } from "../composables/useInstanceSync";

type DangerAction = "delete" | "force-stop" | "force-delete";
type DangerCopyKey = "delete" | "forceStop" | "forceDelete";

const { t } = useI18n();
const router = useRouter();
const store = useContainerStore();
const instanceSync = useInstanceSync();
const pendingDanger = ref<{ action: DangerAction; containerId: string; name: string } | null>(null);
const purgeDangerData = ref(false);

const dangerCopyKeys: Record<DangerAction, DangerCopyKey> = {
  delete: "delete",
  "force-stop": "forceStop",
  "force-delete": "forceDelete",
};

onMounted(() => {
  instanceSync.start();
  void initializeList();
});

onUnmounted(() => {
  instanceSync.stop();
});

async function initializeList(): Promise<void> {
  await store.fetchInstances();
}

function goToTemplateMarket(): void {
  void router.push({ name: "ImageRegistry" });
}

function openInstanceDetail(containerId: string): void {
  void router.push({ name: "InstanceDetail", params: { id: containerId } });
}

// 删除属于破坏性操作，执行前必须二次确认。
function openDanger(action: DangerAction, containerId: string, name: string): void {
  pendingDanger.value = { action, containerId, name };
  purgeDangerData.value = false;
}

function cancelDanger(): void {
  pendingDanger.value = null;
  purgeDangerData.value = false;
}

function dangerCopyKey(action: DangerAction): DangerCopyKey {
  return dangerCopyKeys[action];
}

async function confirmDanger(): Promise<void> {
  const pending = pendingDanger.value;
  if (!pending) return;
  const purgeData = purgeDangerData.value;
  cancelDanger();

  if (pending.action === "force-stop") {
    store.print(t("status.forceStopping", { name: pending.name }));
    const success = await store.forceStop({
      container_id: pending.containerId,
      name: pending.name,
      status: "Running",
    });
    if (success) {
      store.print(t("status.forceStopSuccess", { name: pending.name }));
    }
    return;
  }

  if (pending.action === "force-delete") {
    store.print(t("status.forceDeleting", { name: pending.name }));
    await store.forceRemove(pending.containerId, purgeData);
    return;
  }

  store.print(t("status.deleting"));
  await store.remove(pending.containerId, purgeData);
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

async function handleRestart(instance: {
  container_id: string;
  name: string;
  status: string;
}): Promise<void> {
  store.print(t("status.restarting", { name: instance.name }));
  const success = await store.restart(instance);
  if (success) {
    store.print(t("status.restartSuccess", { name: instance.name }));
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
          <template v-if="store.isRunning(item.status)">
            <button class="secondary-btn compact-btn" @click="handleRestart(item)">
              {{ $t("containers.restart") }}
            </button>
            <button
              class="danger-outline-btn compact-btn"
              @click="openDanger('force-stop', item.container_id, item.name)"
            >
              {{ $t("containers.forceStop") }}
            </button>
            <button
              class="delete-btn compact-btn"
              @click="openDanger('force-delete', item.container_id, item.name)"
            >
              {{ $t("containers.forceDelete") }}
            </button>
          </template>
          <button
            v-else
            class="delete-btn compact-btn"
            @click="openDanger('delete', item.container_id, item.name)"
          >
            {{ $t("containers.delete") }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="pendingDanger" class="modal-overlay" @click.self="cancelDanger">
      <section class="delete-dialog" role="dialog" aria-modal="true">
        <h2 class="dialog-title">
          {{ $t(`containers.${dangerCopyKey(pendingDanger.action)}Title`) }}
        </h2>
        <p class="dialog-message">
          {{
            $t(`containers.${dangerCopyKey(pendingDanger.action)}Message`, {
              name: pendingDanger.name,
            })
          }}
        </p>
        <label v-if="pendingDanger.action !== 'force-stop'" class="purge-option">
          <input v-model="purgeDangerData" type="checkbox" />
          <span>{{ $t("containers.confirmPurgeData") }}</span>
        </label>
        <div class="dialog-actions">
          <button class="secondary-btn" @click="cancelDanger">
            {{ $t("containers.cancelDelete") }}
          </button>
          <button class="delete-btn" @click="confirmDanger">
            {{ $t(`containers.${dangerCopyKey(pendingDanger.action)}Action`) }}
          </button>
        </div>
      </section>
    </div>
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
  border: 2px solid var(--card-border);
  border-radius: 0;
  font-size: 14px;
  font-weight: bold;
  cursor: pointer;
  box-shadow: 2px 2px 0 0 var(--create-border-outer);
  transition: all 0.2s ease;
}

.create-btn:hover {
  transform: translate(1px, 1px);
  box-shadow: 1px 1px 0 0 var(--create-border-outer);
  background-color: var(--create-brass-secondary);
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
  border: 2px dashed var(--border-muted);
  border-radius: 0;
  background: rgba(0, 0, 0, 0.2);
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
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

/* ======= 右侧按钮组件 ======= */
.compact-btn {
  min-height: 32px;
  padding: 6px 12px;
  white-space: nowrap;
}

.delete-btn {
  padding: 6px 16px;
  background-color: var(--danger-light);
  color: var(--danger);
  border: 2px solid var(--danger);
  border-radius: 0;
  font-weight: bold;
  cursor: pointer;
  box-shadow: 2px 2px 0 0 rgba(255, 77, 79, 0.5);
  transition: all 0.2s;
}

.delete-btn:hover {
  transform: translate(1px, 1px);
  box-shadow: 1px 1px 0 0 rgba(255, 77, 79, 0.5);
  background-color: var(--danger);
  color: var(--text-on-dark);
}

.danger-outline-btn {
  background: var(--card-bg);
  color: var(--danger);
  border: 2px solid var(--danger);
  border-radius: 0;
  font-weight: bold;
  cursor: pointer;
  box-shadow: 2px 2px 0 0 rgba(255, 77, 79, 0.35);
  transition: all 0.2s;
}

.danger-outline-btn:hover {
  transform: translate(1px, 1px);
  box-shadow: 1px 1px 0 0 rgba(255, 77, 79, 0.35);
  background: var(--danger-light);
}

.modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 20;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: rgba(0, 0, 0, 0.55);
}

.delete-dialog {
  width: min(420px, 100%);
  padding: 20px;
  background: var(--card-bg);
  border: 3px solid var(--card-border);
  box-shadow:
    inset 0 3px 0 0 var(--card-bg),
    inset 0 -3px 0 0 var(--card-bg),
    inset 0 6px 0 0 var(--card-border-inner),
    inset 0 -6px 0 0 var(--card-border-inner);
}

.dialog-title {
  margin: 0 0 12px;
  color: var(--card-text);
  font-size: 16px;
}

.dialog-message {
  margin: 0 0 16px;
  color: var(--text-muted);
}

.purge-option {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 20px;
  color: var(--card-text);
  font-weight: bold;
}

.dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.secondary-btn {
  padding: 6px 16px;
  background-color: var(--card-bg);
  color: var(--card-text);
  border: 2px solid var(--card-border);
  border-radius: 0;
  font-weight: bold;
  cursor: pointer;
  box-shadow: 2px 2px 0 0 var(--create-border-outer);
  transition: all 0.2s;
}

.secondary-btn:hover {
  transform: translate(1px, 1px);
  box-shadow: 1px 1px 0 0 var(--create-border-outer);
  background: var(--hover-lighten);
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
  border-radius: 0;
}
.slider.round:before {
  border-radius: 0;
}

/* ========== 响应式自适应 ========== */
@media (max-width: 1023px) {
  .page-title {
    display: none;
  }
}
</style>
