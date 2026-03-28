<script setup>
import { ref, onMounted } from "vue";
import Sidebar from "./components/Sidebar.vue";
import {
  createInstance,
  deleteInstance,
  listInstances,
  startInstance,
  stopInstance,
} from "./api/index.js";

const instances = ref([]);
const showCreateModal = ref(false);
const newContainerName = ref("");
const output = ref("等待操作...");

// 简单的日志输出封装
function print(data) {
  output.value = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
}

function printError(error) {
  output.value = `ERROR: ${error.message}`;
}

// 获取容器列表并更新状态
async function fetchInstances() {
  try {
    const data = await listInstances();
    // 假设接口返回数组
    instances.value = Array.isArray(data) ? data : (data.instances || []);
    if (instances.value.length > 0) {
      print("已刷新容器列表。");
    } else {
      print("当前暂无容器。");
    }
  } catch (error) {
    printError(error);
  }
}

// 页面加载完成后自动获取列表
onMounted(() => {
  fetchInstances();
});

// 处理创建容器
async function handleCreate() {
  const trimmed = newContainerName.value.trim();
  if (!trimmed) {
    printError(new Error("容器名称不能为空"));
    return;
  }

  try {
    output.value = "正在创建容器...";
    const data = await createInstance(trimmed);
    print(data);
    showCreateModal.value = false;
    newContainerName.value = "";
    // 创建完成后重新刷新列表
    await fetchInstances();
  } catch (error) {
    printError(error);
  }
}

// 处理删除容器
async function handleDelete(containerId) {
  if (!confirm("确认彻底删除该容器吗？")) return;
  
  try {
    output.value = "正在删除容器...";
    const data = await deleteInstance(containerId);
    print(data);
    await fetchInstances();
  } catch (error) {
    printError(error);
  }
}

// 处理拉杆开关（开启/关闭容器）
async function handleToggle(instance) {
  const isRunning = isInstanceRunning(instance.status);
  
  try {
    // 拉杆点击时可能因为网络延迟导致状态闪烁，可以在调 API 之前可以先加个 loading 或者直接请求
    if (isRunning) {
      output.value = `正在关闭容器: ${instance.name}...`;
      await stopInstance(instance.container_id);
      print(`容器 ${instance.name} 请求关闭成功`);
    } else {
      output.value = `正在开启容器: ${instance.name}...`;
      await startInstance(instance.container_id);
      print(`容器 ${instance.name} 请求开启成功`);
    }
  } catch (error) {
    printError(error);
  } finally {
    // 无论请求成功与否，都要拉取最新状态刷新列表
    await fetchInstances();
  }
}

// 判断容器当前是否为运行状态
function isInstanceRunning(status) {
  if (!status) return false;
  // Docker 运行状态通常带有 "Up" 字样，或者用 "running" 表示
  return status.toLowerCase().startsWith("up") || status.toLowerCase() === "running";
}
</script>

<template>
  <Sidebar />
  <div class="page-wrapper">
    <!-- 顶部栏 -->
    <header class="page-header">
      <h1 class="page-title">容器列表</h1>
    </header>
    
    <main class="main-content">
      <!-- 列表操作栏 -->
      <div class="content-actions">
        <button class="create-btn" @click="showCreateModal = true">新建容器</button>
      </div>
      <!-- 卡片列表布局 -->
      <div class="card-list">
        <div v-if="instances.length === 0" class="empty-state">
          暂无容器，请点击右上角新建容器。
        </div>
        
        <div class="card" v-for="item in instances" :key="item.container_id">
          <!-- 左侧：容器名称 -->
          <div class="card-left">
            <span class="card-name">{{ item.name }}</span>
            <!-- (需求：暂时不显示 ID) -->
          </div>
          <!-- 右侧：控制按钮（拉杆与删除） -->
          <div class="card-right">
            <label class="switch" title="开启/关闭">
              <input type="checkbox" :checked="isInstanceRunning(item.status)" @change="handleToggle(item)" />
              <span class="slider round"></span>
            </label>
            <button class="delete-btn" @click="handleDelete(item.container_id)">删除</button>
          </div>
        </div>
      </div>

      <!-- 新建容器的弹窗 (Modal) -->
      <div class="modal" v-if="showCreateModal">
        <div class="modal-content">
          <h3>新建容器</h3>
          <input 
            v-model="newContainerName" 
            placeholder="请输入新容器名称..." 
            @keyup.enter="handleCreate"
          />
          <div class="modal-actions">
            <button class="btn-cancel" @click="showCreateModal = false">取消</button>
            <button class="btn-confirm" @click="handleCreate">确定</button>
          </div>
        </div>
      </div>

      <!-- 用于显示接口返回的简易日志 -->
      <pre class="output">{{ output }}</pre>
    </main>
  </div>
</template>

<style scoped>
.page-wrapper {
  flex: 1; /* 占据侧边栏右侧的全部剩余空间 */
  display: flex;
  flex-direction: column;
  position: relative;
  height: 100vh;
  overflow-y: auto;
}

.page-header {
  height: var(--header-height); /* 使用 CSS 全局变量保证模块化同步 */
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  position: relative; /* 供右上角按钮绝对定位使用 */
}

.page-title {
  margin: 0;
  color: var(--create-brass-primary);
  font-size: 16px;
  font-weight: bold;
  letter-spacing: 2px;
  font-family: 'Segoe UI', "PingFang SC", sans-serif;
}

/* 内容区操作栏与新建按钮 */
.content-actions {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px; /* 减少按钮与下方卡片的距离 */
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
  padding: 8px 24px 24px 24px; /* 原为 24px，缩小顶端 padding 以拉近与标题的距离 */
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
  padding: 10px 20px; /* 增加上下纵深的内含空间，避免过粗的(12px)内连线压入核心文字中 */
  background-color: var(--card-bg);
  border: 3px solid var(--card-border);
  box-shadow: 
    inset 0 3px 0 0 var(--card-bg),
    inset 0 -3px 0 0 var(--card-bg),
    inset 0 6px 0 0 var(--card-border-inner),
    inset 0 -6px 0 0 var(--card-border-inner);
  border-radius: 0;
  /* 裁掉四个角的 3px x 3px 方块，每个角用 3 个点走直角台阶 */
  clip-path: polygon(
    /* 左上角 */  0 3px, 3px 3px, 3px 0,
    /* 右上角 */  calc(100% - 3px) 0, calc(100% - 3px) 3px, 100% 3px,
    /* 右下角 */  100% calc(100% - 3px), calc(100% - 3px) calc(100% - 3px), calc(100% - 3px) 100%,
    /* 左下角 */  3px 100%, 3px calc(100% - 3px), 0 calc(100% - 3px)
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
  top: 0; left: 0; right: 0; bottom: 0;
  background-color: var(--toggle-off);
  transition: .4s;
}

.slider:before {
  position: absolute;
  content: "";
  height: 18px;
  width: 18px;
  left: 3px;
  bottom: 3px;
  background-color: white;
  transition: .4s;
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
    display: none; /* 移动端只隐藏标题 */
  }
}
</style>
