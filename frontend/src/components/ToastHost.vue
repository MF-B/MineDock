<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useToastStore, type ToastKind, type ToastMessage } from "../stores/toasts";

const { t } = useI18n();
const store = useToastStore();

const kindLabel: Record<ToastKind, string> = {
  info: "Info",
  success: "OK",
  warning: "Warn",
  error: "Error",
};

const messages = computed(() => store.messages);

function messageText(message: ToastMessage): string {
  if (message.i18n) {
    return t(message.i18n.key, message.i18n.values ?? {});
  }
  return message.text ?? "";
}
</script>

<template>
  <Teleport to="body">
    <TransitionGroup name="toast" tag="div" class="toast-host">
      <article
        v-for="message in messages"
        :key="message.id"
        class="toast"
        :class="`toast-${message.kind}`"
        role="status"
      >
        <div class="toast-marker">{{ kindLabel[message.kind] }}</div>
        <div class="toast-text">{{ messageText(message) }}</div>
        <button
          class="toast-close"
          type="button"
          aria-label="Close"
          @click="store.remove(message.id)"
        >
          x
        </button>
      </article>
    </TransitionGroup>
  </Teleport>
</template>

<style scoped>
.toast-host {
  position: fixed;
  top: 64px;
  right: 24px;
  z-index: 100;
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: min(360px, calc(100vw - 32px));
  pointer-events: none;
}

.toast {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: start;
  gap: 10px;
  min-height: 52px;
  padding: 10px 12px;
  background: var(--card-bg);
  border: 3px solid var(--card-border);
  color: var(--card-text);
  box-shadow:
    3px 3px 0 0 rgba(0, 0, 0, 0.35),
    inset 0 4px 0 0 rgba(255, 255, 255, 0.25);
  pointer-events: auto;
}

.toast-marker {
  min-width: 44px;
  padding: 3px 6px;
  border: 2px solid var(--card-border);
  background: var(--create-brass-dark);
  color: var(--card-text);
  font-size: 11px;
  font-weight: 800;
  text-align: center;
}

.toast-success .toast-marker {
  background: #c7f7d4;
}

.toast-warning .toast-marker {
  background: #ffe29a;
}

.toast-error .toast-marker {
  background: var(--danger-light);
  color: var(--danger);
}

.toast-text {
  align-self: center;
  min-width: 0;
  font-size: 13px;
  line-height: 1.4;
  overflow-wrap: anywhere;
}

.toast-close {
  width: 24px;
  height: 24px;
  border: 2px solid var(--card-border);
  background: transparent;
  color: var(--card-text);
  cursor: pointer;
  font-weight: 800;
  line-height: 1;
}

.toast-enter-active,
.toast-leave-active {
  transition:
    opacity 0.2s ease,
    transform 0.2s ease;
}

.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateX(16px);
}

@media (max-width: 767px) {
  .toast-host {
    top: 56px;
    right: 12px;
    left: 12px;
    width: auto;
  }
}
</style>
