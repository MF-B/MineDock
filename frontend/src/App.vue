<script setup>
import { ref } from "vue";
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
</template>
