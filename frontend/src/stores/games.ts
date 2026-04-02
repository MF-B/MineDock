import { computed, ref } from "vue";
import { defineStore } from "pinia";
import type { Game, GameTemplate } from "../api/index";
import { getGameTemplate, listGames } from "../api/index";

const DEFAULT_CACHE_WINDOW_MS = 60_000;

type FetchGamesOptions = {
  force?: boolean;
  cacheWindowMs?: number;
};

export const useGameStore = defineStore("games", () => {
  const games = ref<Game[]>([]);
  const currentTemplate = ref<GameTemplate | null>(null);
  const currentTemplateGameID = ref<string>("");
  const loading = ref(false);
  const templateLoading = ref(false);
  const error = ref<unknown>(null);
  const hasFetched = ref(false);
  const fetchedAt = ref<number>(0);
  let inflightGamesRequest: Promise<void> | null = null;
  let inflightTemplateRequest: Promise<void> | null = null;

  const categories = computed<string[]>(() => {
    const values = new Set<string>();
    for (const game of games.value) {
      const category = game.category?.trim();
      if (category) {
        values.add(category);
      }
    }
    return Array.from(values);
  });

  async function fetchGames(options: FetchGamesOptions = {}): Promise<void> {
    const { force = false, cacheWindowMs = DEFAULT_CACHE_WINDOW_MS } = options;
    const cacheAge = Date.now() - fetchedAt.value;
    const canUseCache = hasFetched.value && cacheAge < cacheWindowMs;

    if (!force && canUseCache) {
      return;
    }

    if (inflightGamesRequest) {
      return inflightGamesRequest;
    }

    loading.value = true;
    error.value = null;

    inflightGamesRequest = (async () => {
      try {
        games.value = await listGames();
        hasFetched.value = true;
        fetchedAt.value = Date.now();
      } catch (err) {
        error.value = err;
        throw err;
      } finally {
        loading.value = false;
        inflightGamesRequest = null;
      }
    })();

    return inflightGamesRequest;
  }

  async function fetchTemplate(gameID: string, force = false): Promise<void> {
    const key = gameID.trim();
    if (!key) {
      currentTemplate.value = null;
      currentTemplateGameID.value = "";
      return;
    }

    if (!force && currentTemplate.value && currentTemplateGameID.value === key) {
      return;
    }

    if (inflightTemplateRequest && currentTemplateGameID.value === key) {
      return inflightTemplateRequest;
    }

    templateLoading.value = true;
    error.value = null;
    currentTemplateGameID.value = key;

    inflightTemplateRequest = (async () => {
      try {
        currentTemplate.value = await getGameTemplate(key);
      } catch (err) {
        currentTemplate.value = null;
        error.value = err;
        throw err;
      } finally {
        templateLoading.value = false;
        inflightTemplateRequest = null;
      }
    })();

    return inflightTemplateRequest;
  }

  function getGameById(id: string): Game | undefined {
    const key = id.trim();
    if (!key) {
      return undefined;
    }
    return games.value.find((item) => item.id === key);
  }

  return {
    games,
    currentTemplate,
    currentTemplateGameID,
    loading,
    templateLoading,
    error,
    hasFetched,
    categories,
    fetchGames,
    fetchTemplate,
    getGameById,
  };
});
