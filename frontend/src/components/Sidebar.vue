<script setup lang="ts">
import { ref } from "vue";
import { RouterLink } from "vue-router";

// 同一开关状态同时驱动桌面折叠和移动端抽屉，保持两端导航行为一致。
const isOpen = ref<boolean>(false);
</script>

<template>
  <!-- 移动端：保持悬浮按钮和全屏遮罩弹出边栏 -->
  <button v-if="!isOpen" class="hamburger-btn mobile-only" @click="isOpen = true">
    <svg viewBox="0 0 20 20" class="icon">
      <path fill="currentColor" d="M2 15h16v-2H2v2zm0-5h16v-2H2v2zm0-7v2h16V3H2z" />
    </svg>
  </button>
  <Transition name="fade">
    <div v-if="isOpen" class="sidebar-overlay mobile-only" @click="isOpen = false"></div>
  </Transition>
  <Transition name="slide">
    <aside v-if="isOpen" class="sidebar-mobile mobile-only">
      <div class="mobile-header"></div>
      <div class="sidebar-content">
        <!-- 导航菜单项 -->
        <RouterLink to="/" class="menu-item">
          <div class="menu-icon">
            <svg viewBox="0 0 24 24" fill="currentColor">
              <!-- 象征"容器/集装箱"的图标 -->
              <path d="M4 8h16v12a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8zm0-4h16v2H4V4z" />
            </svg>
          </div>
          <div class="menu-text">{{ $t("sidebar.containerList") }}</div>
        </RouterLink>
        <RouterLink to="/registry" class="menu-item">
          <div class="menu-icon">
            <svg viewBox="0 0 24 24" fill="currentColor">
              <path d="M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z" />
            </svg>
          </div>
          <div class="menu-text">{{ $t("sidebar.imageRegistry") }}</div>
        </RouterLink>
        <RouterLink to="/monitor" class="menu-item">
          <div class="menu-icon">
            <svg viewBox="0 0 24 24" fill="currentColor">
              <path d="M3 19h18v2H3v-2zm2-3h3V8H5v8zm5 0h3V3h-3v13zm5 0h3v-6h-3v6z" />
            </svg>
          </div>
          <div class="menu-text">{{ $t("sidebar.serverMonitor") }}</div>
        </RouterLink>
      </div>
    </aside>
  </Transition>

  <!-- 桌面端：单一侧边栏，通过 flex 挤压右侧 -->
  <aside class="sidebar-desktop desktop-only" :class="{ 'is-expanded': isOpen }">
    <div class="desktop-icon-container">
      <button class="hamburger-btn-narrow" @click="isOpen = !isOpen">
        <svg viewBox="0 0 20 20" class="icon">
          <path fill="currentColor" d="M2 15h16v-2H2v2zm0-5h16v-2H2v2zm0-7v2h16V3H2z" />
        </svg>
      </button>
    </div>

    <div class="sidebar-content">
      <!-- 导航菜单项 -->
      <RouterLink to="/" class="menu-item">
        <div class="menu-icon">
          <svg viewBox="0 0 24 24" fill="currentColor">
            <path d="M4 8h16v12a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8zm0-4h16v2H4V4z" />
          </svg>
        </div>
        <div class="menu-text">{{ $t("sidebar.containerList") }}</div>
      </RouterLink>
      <RouterLink to="/registry" class="menu-item">
        <div class="menu-icon">
          <svg viewBox="0 0 24 24" fill="currentColor">
            <path d="M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z" />
          </svg>
        </div>
        <div class="menu-text">{{ $t("sidebar.imageRegistry") }}</div>
      </RouterLink>
      <RouterLink to="/monitor" class="menu-item">
        <div class="menu-icon">
          <svg viewBox="0 0 24 24" fill="currentColor">
            <path d="M3 19h18v2H3v-2zm2-3h3V8H5v8zm5 0h3V3h-3v13zm5 0h3v-6h-3v6z" />
          </svg>
        </div>
        <div class="menu-text">{{ $t("sidebar.serverMonitor") }}</div>
      </RouterLink>
    </div>
  </aside>
</template>

<style scoped>
/* =========== 移动端悬浮按钮与遮罩 =========== */
.hamburger-btn {
  position: fixed;
  top: 8px; /* header-height(48) / 2 - btn-height(32) / 2 = 8px */
  left: 12px;
  z-index: 40;
  background: transparent;
  border: none;
  color: var(--create-brass-primary);
  padding: 4px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  transition: all 0.2s ease;
}
.hamburger-btn:hover {
  background: var(--hover-darken);
  color: var(--create-brass-secondary);
}
.icon {
  width: 24px;
  height: 24px;
}

.sidebar-overlay {
  position: fixed;
  inset: 0;
  background: var(--shadow-medium);
  z-index: 45;
}

/* =========== 移动端弹出的宽侧边栏 =========== */
.sidebar-mobile {
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  width: 300px;
  background-color: var(--create-bg, #2e2824);
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16'%3E%3Cpath fill='%23343a3d' d='M0,0h4v1H0z M6,0h2v1H6z M15,1h1v1H15z M0,1h3v1H0z M5,1h2v1H5z M14,2h2v1H14z M0,2h2v1H0z M4,2h2v1H4z M13,3h3v1H13z M0,3h1v1H0z M3,3h2v1H3z M12,4h4v1H12z M2,4h2v1H2z M11,5h4v1H11z M1,5h2v1H1z M10,6h4v1H10z M0,6h2v1H0z M9,7h4v1H9z M15,7h1v1H15z M0,7h1v1H0z M8,8h4v1H8z M14,8h2v1H14z M7,9h4v1H7z M13,9h2v1H13z M6,10h4v1H6z M12,10h2v1H12z M5,11h4v1H5z M11,11h2v1H11z M4,12h4v1H4z M10,12h2v1H10z M3,13h4v1H3z M9,13h2v1H9z M2,14h4v1H2z M8,14h2v1H8z M1,15h4v1H1z M7,15h2v1H7z'/%3E%3C/svg%3E");
  background-size: 128px 128px;
  image-rendering: pixelated;
  border: 4px solid var(--create-border-outer);
  z-index: 50;
  display: flex;
  flex-direction: column;
  box-shadow: 4px 0 16px var(--shadow-medium);
}

.sidebar-mobile::after {
  content: "";
  position: absolute;
  inset: 0;
  border: 4px solid var(--create-border-inner);
  pointer-events: none;
  box-shadow:
    inset 0 0 0 4px var(--create-border-dark),
    inset -2px 0 4px var(--shadow-light);
  z-index: 20;
}

.mobile-header {
  height: var(--header-height);
  flex-shrink: 0;
  width: 100%;
  z-index: 10;
}

/* =========== 桌面端动态宽度的侧边栏 =========== */
.sidebar-desktop {
  position: relative;
  width: 64px;
  min-width: 64px;
  height: 100vh;
  box-sizing: border-box;
  background-color: var(--create-bg, #2e2824);
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16'%3E%3Cpath fill='%23343a3d' d='M0,0h4v1H0z M6,0h2v1H6z M15,1h1v1H15z M0,1h3v1H0z M5,1h2v1H5z M14,2h2v1H14z M0,2h2v1H0z M4,2h2v1H4z M13,3h3v1H13z M0,3h1v1H0z M3,3h2v1H3z M12,4h4v1H12z M2,4h2v1H2z M11,5h4v1H11z M1,5h2v1H1z M10,6h4v1H10z M0,6h2v1H0z M9,7h4v1H9z M15,7h1v1H15z M0,7h1v1H0z M8,8h4v1H8z M14,8h2v1H14z M7,9h4v1H7z M13,9h2v1H13z M6,10h4v1H6z M12,10h2v1H12z M5,11h4v1H5z M11,11h2v1H11z M4,12h4v1H4z M10,12h2v1H10z M3,13h4v1H3z M9,13h2v1H9z M2,14h4v1H2z M8,14h2v1H8z M1,15h4v1H1z M7,15h2v1H7z'/%3E%3C/svg%3E");
  background-size: 128px 128px;
  image-rendering: pixelated;
  border: 4px solid var(--create-border-outer);
  transition:
    width 0.3s cubic-bezier(0.4, 0, 0.2, 1),
    min-width 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
  box-shadow: 2px 0 8px var(--shadow-light);
  display: none;
  flex-direction: column;
}

.sidebar-desktop.is-expanded {
  width: 300px;
  min-width: 300px;
}

.sidebar-desktop::after {
  content: "";
  position: absolute;
  inset: 0;
  border: 4px solid var(--create-border-inner);
  pointer-events: none;
  box-shadow:
    inset 0 0 0 4px var(--create-border-dark),
    inset -2px 0 4px var(--shadow-light);
  z-index: 20;
}

.desktop-icon-container {
  width: 56px;
  height: var(--header-height);
  padding-top: 12px; /* 把汉堡挪到底侧，为顶部让出刚好空隙 */
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.hamburger-btn-narrow {
  background: transparent;
  border: none;
  color: var(--create-brass-primary);
  cursor: pointer;
  padding: 8px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
}
.hamburger-btn-narrow:hover {
  background: var(--hover-darken);
  color: var(--create-brass-secondary);
}

/* =========== 菜单列表样式 =========== */
.sidebar-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 4px 0 10px 0;
  overflow-y: auto;
  overflow-x: hidden;
  z-index: 10;
}

/* 精心计算的尺寸：未展开时，16(padding)+24(icon)+16(margin)=56，刚填满可显示区域，文字刚好被剪裁不可见！*/
.menu-item {
  display: flex;
  align-items: center;
  height: 52px;
  padding-left: 16px;
  padding-right: 16px;
  color: var(--create-brass-primary);
  text-decoration: none;
  cursor: pointer;
  background: transparent;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.menu-item:hover {
  background: var(--hover-darken);
  color: var(--text-on-dark);
}

.menu-item:is(.router-link-exact-active) {
  background: var(--active-darken);
  border-left: 4px solid var(--create-brass-secondary);
  padding-left: 12px;
  color: var(--create-brass-primary);
}

.menu-icon {
  width: 24px;
  height: 24px;
  display: flex;
  flex-shrink: 0;
  margin-right: 16px;
  align-items: center;
  justify-content: center;
}

.menu-text {
  font-size: 16px;
  font-weight: bold;
  letter-spacing: 1px;
}

/* =========== 响应式媒体查询：屏幕够大时 =========== */
@media (min-width: 1024px) {
  .mobile-only {
    display: none !important;
  }
  .sidebar-desktop.desktop-only {
    display: flex !important;
  }
}

/* Animations */
.slide-enter-active,
.slide-leave-active {
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}
.slide-enter-from,
.slide-leave-to {
  transform: translateX(-100%);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
