<script setup lang="ts">
import Sidebar from "./components/Sidebar.vue";
</script>

<template>
  <Sidebar />
  <div class="page-wrapper">
    <router-view />
  </div>
</template>

<style scoped>
.page-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  position: relative;
  height: 100vh;
  overflow-y: auto;
}
</style>
