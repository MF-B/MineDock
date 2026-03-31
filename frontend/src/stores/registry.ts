import { ref } from "vue";
import { defineStore } from "pinia";
import type { RegistryImage } from "../api/index";
import { listRegistryImages } from "../api/index";

export const useRegistryStore = defineStore("registry", () => {
  const images = ref<RegistryImage[]>([]);
  const loading = ref(false);

  async function fetchImages(): Promise<void> {
    loading.value = true;
    try {
      images.value = await listRegistryImages();
    } finally {
      loading.value = false;
    }
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
    fetchImages,
    getById,
  };
});
