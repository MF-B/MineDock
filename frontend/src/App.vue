<script setup>
import { ref } from "vue";
import Sidebar from "./components/Sidebar.vue";
import {
  createInstance,
  deleteInstance,
  listInstances,
  startInstance,
  stopInstance,
} from "./api/index.js";

const name = ref("测试服1号");
const containerId = ref("");
const output = ref("等待操作...");

function print(data) {
  output.value = JSON.stringify(data, null, 2);
}

function printError(error) {
  output.value = `ERROR: ${error.message}`;
}

async function handleList() {
  try {
    const data = await listInstances();
    print(data);
  } catch (error) {
    printError(error);
  }
}

async function handleCreate() {
  const trimmed = name.value.trim();
  if (!trimmed) {
    printError(new Error("name is required"));
    return;
  }

  try {
    const data = await createInstance(trimmed);
    print(data);
    containerId.value = data.container_id || "";
  } catch (error) {
    printError(error);
  }
}

async function handleDelete() {
  const trimmed = containerId.value.trim();
  if (!trimmed) {
    printError(new Error("container id is required"));
    return;
  }

  try {
    const data = await deleteInstance(trimmed);
    print(data);
  } catch (error) {
    printError(error);
  }
}

async function handleStart() {
  const trimmed = containerId.value.trim();
  if (!trimmed) {
    printError(new Error("container id is required"));
    return;
  }

  try {
    const data = await startInstance(trimmed);
    print(data);
  } catch (error) {
    printError(error);
  }
}

async function handleStop() {
  const trimmed = containerId.value.trim();
  if (!trimmed) {
    printError(new Error("container id is required"));
    return;
  }

  try {
    const data = await stopInstance(trimmed);
    print(data);
  } catch (error) {
    printError(error);
  }
}
</script>

<template>
  <Sidebar />
  <div class="page-wrapper">
    <!-- 与左侧汉堡按钮等高的 60px 透明顶部栏 -->
    <header class="page-header">
      <h1 class="page-title">容器列表</h1>
    </header>
    
    <main class="panel">
      <h1>MineDock MVP 控制台</h1>

    <div class="actions">
      <button @click="handleList">获取列表</button>
      <button @click="handleCreate">创建</button>
      <button @click="handleStart">开启</button>
      <button @click="handleStop">关闭</button>
      <button @click="handleDelete">删除</button>
    </div>

    <div class="field">
      <input v-model="name" placeholder="新容器名称，例如：测试服1号" />
      <input v-model="containerId" placeholder="目标容器 ID" />
    </div>

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
  height: 60px; /* 严格对齐左侧固定侧边栏的 60px 图标容器 */
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  /* 透明顶部栏无需背景色 */
}

.page-title {
  margin: 0; /* 之前用 margin 往下推，现在由 flex 完美垂直居中 */
  color: var(--create-brass-primary, #fde285);
  font-size: 24px;
  font-weight: bold;
  letter-spacing: 2px;
  font-family: 'Segoe UI', "PingFang SC", sans-serif;
}

/* =========== 响应式标题隐藏 =========== */
@media (max-width: 1023px) {
  .page-header {
    display: none;
  }
}
</style>
