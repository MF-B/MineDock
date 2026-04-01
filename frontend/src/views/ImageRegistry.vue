<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import type { RegistryImage } from "../api/index";
import { useRegistryStore } from "../stores/registry";

const router = useRouter();
const { t } = useI18n();
const registryStore = useRegistryStore();

const ALL_CATEGORY = "__all__";

const iconEmojiMap: Record<string, string> = {
  "minecraft-java": "\u26CF\uFE0F",
  "minecraft-bedrock": "\u26CF\uFE0F",
  terraria: "\uD83C\uDF33",
};

const selectedCategory = ref(ALL_CATEGORY);

const categoryTabs = computed(() => {
  return [
    { key: ALL_CATEGORY, label: t("registry.filterAll") },
    ...registryStore.categories.map((category) => ({ key: category, label: category })),
  ];
});

const filteredImages = computed(() => {
  if (selectedCategory.value === ALL_CATEGORY) {
    return registryStore.images;
  }
  return registryStore.images.filter((image) => image.category === selectedCategory.value);
});

watch(categoryTabs, (tabs) => {
  if (!tabs.some((tab) => tab.key === selectedCategory.value)) {
    selectedCategory.value = ALL_CATEGORY;
  }
});

onMounted(() => {
  void loadImages();
});

async function loadImages(): Promise<void> {
  try {
    await registryStore.fetchImages();
  } catch {
    // error state is captured by registry store and rendered by the view.
  }
}

function getImageEmoji(image: RegistryImage): string {
  const mapped = iconEmojiMap[image.icon];
  if (mapped) {
    return mapped;
  }
  const fallback = image.name.trim().charAt(0);
  return fallback ? fallback.toUpperCase() : "?";
}

function selectImage(imageId: string): void {
  void router.push({
    name: "ContainerList",
    query: { imageId },
  });
}
</script>

<template>
  <header class="page-header">
    <h1 class="page-title">{{ $t("registry.title") }}</h1>
  </header>

  <main class="main-content">
    <div class="filter-tabs" role="tablist" aria-label="Category filter">
      <button
        v-for="tab in categoryTabs"
        :key="tab.key"
        class="filter-tab"
        :class="{ 'is-active': selectedCategory === tab.key }"
        type="button"
        role="tab"
        :aria-selected="selectedCategory === tab.key"
        @click="selectedCategory = tab.key"
      >
        {{ tab.label }}
      </button>
    </div>

    <div v-if="registryStore.loading && registryStore.images.length === 0" class="state-message">
      {{ $t("registry.loading") }}
    </div>
    <div
      v-else-if="registryStore.error && registryStore.images.length === 0"
      class="state-message state-error"
    >
      {{ $t("registry.loadError") }}
    </div>
    <div v-else-if="filteredImages.length === 0" class="state-message">
      {{ $t("registry.emptyState") }}
    </div>
    <section v-else class="image-grid">
      <button
        v-for="image in filteredImages"
        :key="image.id"
        class="image-card"
        type="button"
        @click="selectImage(image.id)"
      >
        <div class="image-icon">{{ getImageEmoji(image) }}</div>
        <div class="image-name">{{ image.name }}</div>
        <p class="image-description">{{ image.description }}</p>
        <span class="category-badge">{{ image.category }}</span>
      </button>
    </section>
  </main>
</template>

<style scoped>
.page-header {
  height: var(--header-height);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.page-title {
  margin: 0;
  color: var(--create-brass-primary);
  font-size: 16px;
  font-weight: bold;
  letter-spacing: 2px;
  font-family: "Segoe UI", "PingFang SC", sans-serif;
}

.main-content {
  padding: 8px 24px 24px 24px;
  flex: 1;
  display: flex;
  flex-direction: column;
  max-width: 1200px;
  margin: 0 auto;
  width: 100%;
  gap: 16px;
}

.filter-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.filter-tab {
  border: 1px solid var(--create-border-outer);
  background: rgba(0, 0, 0, 0.15);
  color: var(--create-brass-primary);
  padding: 6px 14px;
  border-radius: 999px;
  cursor: pointer;
  font-size: 13px;
  transition: all 0.2s ease;
}

.filter-tab:hover {
  background: rgba(0, 0, 0, 0.3);
}

.filter-tab.is-active {
  background: var(--create-brass-dark);
  color: var(--card-text);
  border-color: var(--create-brass-secondary);
}

.state-message {
  text-align: center;
  color: var(--text-muted);
  border: 1px dashed var(--border-muted);
  border-radius: 8px;
  padding: 40px 16px;
}

.state-error {
  color: var(--danger);
  border-color: rgba(255, 77, 79, 0.5);
  background: var(--danger-light);
}

.image-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.image-card {
  text-align: left;
  border: 3px solid var(--card-border);
  background: var(--card-bg);
  color: var(--card-text);
  border-radius: 0;
  padding: 16px;
  min-height: 220px;
  cursor: pointer;
  box-shadow:
    inset 0 3px 0 0 var(--card-bg),
    inset 0 -3px 0 0 var(--card-bg),
    inset 0 6px 0 0 var(--card-border-inner),
    inset 0 -6px 0 0 var(--card-border-inner);
  clip-path: polygon(
    0 3px,
    3px 3px,
    3px 0,
    calc(100% - 3px) 0,
    calc(100% - 3px) 3px,
    100% 3px,
    100% calc(100% - 3px),
    calc(100% - 3px) calc(100% - 3px),
    calc(100% - 3px) 100%,
    3px 100%,
    3px calc(100% - 3px),
    0 calc(100% - 3px)
  );
  transition:
    transform 0.2s ease,
    filter 0.2s ease;
}

.image-card:hover {
  transform: translateY(-2px);
  filter: brightness(0.97);
}

.image-icon {
  font-size: 32px;
  margin-bottom: 12px;
}

.image-name {
  font-size: 17px;
  font-weight: 700;
  margin-bottom: 8px;
}

.image-description {
  margin: 0 0 16px;
  color: #3b3b3b;
  font-size: 13px;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.category-badge {
  display: inline-block;
  margin-top: auto;
  font-size: 12px;
  color: var(--create-brass-primary);
  background: var(--create-border-dark);
  border: 1px solid var(--create-border-outer);
  border-radius: 999px;
  padding: 3px 10px;
}

@media (max-width: 1023px) {
  .page-title {
    display: none;
  }

  .image-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 767px) {
  .main-content {
    padding: 8px 16px 20px 16px;
  }

  .image-grid {
    grid-template-columns: 1fr;
  }
}
</style>
