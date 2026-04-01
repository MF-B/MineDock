<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from "vue";
import { useI18n } from "vue-i18n";
import { useContainerStore } from "../stores/containers";

const { t, locale } = useI18n();
const store = useContainerStore();

const isOpen = ref(false);
const syncStatusTitle = computed(() => {
  return store.wsConnected ? t("topbar.realtimeConnected") : t("topbar.realtimeDisconnected");
});

const toggleMenu = () => {
  isOpen.value = !isOpen.value;
};

const setLocale = (lang: typeof locale.value) => {
  locale.value = lang;
  isOpen.value = false;
};

// 语言菜单只在组件内维护，点击外部区域时统一收起。
const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement;
  if (!target.closest(".lang-selector")) {
    isOpen.value = false;
  }
};

onMounted(() => {
  // 使用 document 级监听做外部点击检测，卸载时必须成对移除。
  document.addEventListener("click", handleClickOutside);
});

onUnmounted(() => {
  document.removeEventListener("click", handleClickOutside);
});
</script>

<template>
  <div class="top-bar-overlay">
    <div class="top-actions">
      <div class="sync-indicator" :title="syncStatusTitle" :aria-label="syncStatusTitle">
        <span class="sync-dot" :class="{ connected: store.wsConnected }"></span>
      </div>

      <div class="lang-selector">
        <button class="lang-btn" :title="t('topbar.switchLanguage')" @click.stop="toggleMenu">
          <!-- "文A" or Translate Icon -->
          <svg
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            width="20"
            height="20"
            fill="currentColor"
          >
            <path
              d="M12.87 15.07l-2.54-2.51.03-.03A17.52 17.52 0 0014.07 6H17V4h-7V2H8v2H1v2h11.17C11.5 7.92 10.44 9.75 9 11.35 8.07 10.32 7.3 9.19 6.69 8h-2c.73 1.63 1.73 3.17 2.98 4.56l-5.09 5.02L4 19l5-5 3.11 3.11.76-2.04zM18.5 10h-2L12 22h2l1.12-3h4.75L21 22h2l-4.5-12zm-2.62 7l1.62-4.33L19.12 17h-3.24z"
            />
          </svg>
        </button>

        <div v-show="isOpen" class="dropdown-menu">
          <button
            class="dropdown-item"
            :class="{ active: locale === 'zh-CN' }"
            @click="setLocale('zh-CN')"
          >
            {{ t("topbar.zhCN") }}
          </button>
          <button
            class="dropdown-item"
            :class="{ active: locale === 'en-US' }"
            @click="setLocale('en-US')"
          >
            {{ t("topbar.enUS") }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.top-bar-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: var(--header-height);
  --top-actions-gap: 10px;
  --topbar-sync-dot-size: 10px;
  --topbar-sync-dot-radius: 999px;
  pointer-events: none; /* Let clicks pass through */
  display: flex;
  justify-content: flex-end;
  align-items: center;
  padding: 0 16px;
  z-index: 100;
}

.top-actions {
  display: flex;
  align-items: center;
  gap: var(--top-actions-gap);
}

.sync-indicator {
  pointer-events: auto;
  display: flex;
  align-items: center;
  justify-content: center;
}

.sync-dot {
  width: var(--topbar-sync-dot-size);
  height: var(--topbar-sync-dot-size);
  border-radius: var(--topbar-sync-dot-radius);
  background-color: var(--text-muted);
  box-shadow: 0 0 0 2px var(--hover-darken);
}

.sync-dot.connected {
  background-color: var(--success);
}

.lang-selector {
  position: relative;
  pointer-events: auto; /* Enable clicks for this component */
}

.lang-btn {
  background: transparent;
  border: none;
  color: var(--text-on-dark);
  opacity: 0.8;
  cursor: pointer;
  padding: 8px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
}

.lang-btn:hover {
  opacity: 1;
  background-color: var(--hover-lighten);
}

.dropdown-menu {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 4px;
  background-color: var(--create-bg);
  border: 1px solid var(--create-border-outer);
  border-radius: 4px;
  min-width: 120px;
  box-shadow: 0 4px 6px var(--shadow-light);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.dropdown-item {
  background-color: var(--create-brass-dark);
  border: none;
  color: var(--ink);
  padding: 10px 16px;
  text-align: left;
  cursor: pointer;
  font-size: 14px;
  transition: background-color 0.2s;
  font-weight: bold;
}

.dropdown-item:hover {
  background-color: var(--create-brass-secondary);
}

.dropdown-item.active {
  background-color: transparent;
  color: var(--text-on-dark);
  font-weight: normal;
}
</style>
