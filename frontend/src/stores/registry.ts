import { computed, ref } from "vue";
import { defineStore } from "pinia";
import type { RegistryImage } from "../api/index";
import { listRegistryImages } from "../api/index";

const DEFAULT_CACHE_WINDOW_MS = 60_000;

type FetchImagesOptions = {
  force?: boolean;
  cacheWindowMs?: number;
};

export const useRegistryStore = defineStore("registry", () => {
  const images = ref<RegistryImage[]>([]);
  const loading = ref(false);
  const error = ref<unknown>(null);
  const hasFetched = ref(false);
  const fetchedAt = ref<number>(0);
  let inflightRequest: Promise<void> | null = null;

  const categories = computed<string[]>(() => {
    const values = new Set<string>();
    for (const image of images.value) {
      const category = image.category?.trim();
      if (category) {
        values.add(category);
      }
    }
    return Array.from(values);
  });

  async function fetchImages(options: FetchImagesOptions = {}): Promise<void> {
    const { force = false, cacheWindowMs = DEFAULT_CACHE_WINDOW_MS } = options;
    const cacheAge = Date.now() - fetchedAt.value;
    const canUseCache = hasFetched.value && cacheAge < cacheWindowMs;

    if (!force && canUseCache) {
      return;
    }

    if (inflightRequest) {
      return inflightRequest;
    }

    loading.value = true;
    error.value = null;

    inflightRequest = (async () => {
      try {
        images.value = await listRegistryImages();
        hasFetched.value = true;
        fetchedAt.value = Date.now();
      } catch (err) {
        error.value = err;
        throw err;
      } finally {
        loading.value = false;
        inflightRequest = null;
      }
    })();

    return inflightRequest;
  }

  function getById(id: string): RegistryImage | undefined {
    const key = id.trim();
    if (!key) {
      return undefined;
    }
    return images.value.find((item) => item.id === key);
  }

  return {
    images,
    loading,
    error,
    hasFetched,
    categories,
    fetchImages,
    getById,
  };
});
