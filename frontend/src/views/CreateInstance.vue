<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import type { GameTemplate, PortMapping, ResourceLimits, TemplateParam } from "../api/index";
import { listMinecraftLoaderVersions, listMinecraftVersions } from "../api/index";
import { useGameStore } from "../stores/games";
import { useContainerStore } from "../stores/containers";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const gameStore = useGameStore();
const containerStore = useContainerStore();

const loading = ref(true);
const creating = ref(false);
const currentGameID = ref("");
const containerName = ref("");
const pageErrorKey = ref("");
const ports = ref<PortMapping[]>([]);
const paramValues = ref<Record<string, string>>({});
const resourceEnabled = ref(false);
const resourceMemoryValue = ref(0);
const resourceCPUValue = ref(0);
const minecraftVersion = ref("1.21.4");
const minecraftServerType = ref("");
const minecraftServerVersion = ref("");
const minecraftJavaTag = ref("java21");
const minecraftJavaTouched = ref(false);
const containerNameTouched = ref(false);
const minecraftVersionOptions = ref<string[]>([]);
const minecraftVersionLoading = ref(false);
const minecraftVersionLoadFailed = ref(false);
const loaderVersionOptions = ref<string[]>([]);
const loaderVersionLoading = ref(false);
const loaderVersionLoadFailed = ref(false);
const loaderVersionCustom = ref(false);

const fallbackMinecraftVersions = [
  "1.21.4",
  "1.21.1",
  "1.20.6",
  "1.20.4",
  "1.20.1",
  "1.19.4",
  "1.18.2",
  "1.17.1",
  "1.16.5",
  "1.12.2",
  "1.12.1",
];

const minecraftServerTypes = [
  { value: "", label: "Vanilla" },
  { value: "PAPER", label: "Paper" },
  { value: "FABRIC", label: "Fabric" },
  { value: "FORGE", label: "Forge" },
  { value: "NEOFORGE", label: "NeoForge" },
];

const minecraftJavaTags = [
  { value: "java8", label: "Java 8" },
  { value: "java16", label: "Java 16" },
  { value: "java17", label: "Java 17" },
  { value: "java21", label: "Java 21" },
  { value: "java25", label: "Java 25" },
];

const currentGame = computed(() => {
  return gameStore.getGameById(currentGameID.value) ?? null;
});

const currentTemplate = computed<GameTemplate | null>(() => {
  if (!currentGameID.value || gameStore.currentTemplateGameID !== currentGameID.value) {
    return null;
  }
  return gameStore.currentTemplate;
});

const isMinecraftJava = computed(() => currentGameID.value === "minecraft-java");

const showMinecraftServerVersion = computed(() => {
  return ["FABRIC", "FORGE", "NEOFORGE"].includes(minecraftServerType.value);
});

const recommendedMinecraftJavaTag = computed(() => {
  return recommendMinecraftJavaTag(minecraftVersion.value, minecraftServerType.value);
});

const selectableMinecraftVersions = computed(() => {
  return minecraftVersionOptions.value.length > 0
    ? minecraftVersionOptions.value
    : fallbackMinecraftVersions;
});

watch(
  () => route.params.gameId,
  () => {
    void initializeForRoute();
  },
);

onMounted(() => {
  void initializeForRoute();
});

function parseRouteGameID(value: unknown): string {
  if (Array.isArray(value)) {
    return typeof value[0] === "string" ? value[0].trim() : "";
  }
  return typeof value === "string" ? value.trim() : "";
}

async function initializeForRoute(): Promise<void> {
  loading.value = true;
  pageErrorKey.value = "";
  containerName.value = "";
  ports.value = [];
  paramValues.value = {};
  resourceEnabled.value = false;
  resourceMemoryValue.value = 0;
  resourceCPUValue.value = 0;
  minecraftVersion.value = "1.21.4";
  minecraftServerType.value = "";
  minecraftServerVersion.value = "";
  minecraftJavaTag.value = "java21";
  minecraftJavaTouched.value = false;
  containerNameTouched.value = false;
  minecraftVersionOptions.value = [];
  minecraftVersionLoading.value = false;
  minecraftVersionLoadFailed.value = false;
  loaderVersionOptions.value = [];
  loaderVersionLoading.value = false;
  loaderVersionLoadFailed.value = false;
  loaderVersionCustom.value = false;

  const gameID = parseRouteGameID(route.params.gameId);
  currentGameID.value = gameID;

  if (!gameID) {
    pageErrorKey.value = "errors.gameNotFound";
    loading.value = false;
    return;
  }

  try {
    await gameStore.fetchGames();
  } catch {
    pageErrorKey.value = "registry.loadError";
    loading.value = false;
    return;
  }

  if (!gameStore.getGameById(gameID)) {
    pageErrorKey.value = "errors.gameNotFound";
    loading.value = false;
    return;
  }

  try {
    await gameStore.fetchTemplate(gameID, true);
    initParamValuesFromTemplate(currentTemplate.value);
    initMinecraftDefaults();
    if (isMinecraftJava.value) {
      await fetchMinecraftVersions();
      await fetchLoaderVersions();
    }
  } catch {
    pageErrorKey.value = "registry.templateLoadError";
  }

  loading.value = false;
}

function initParamValuesFromTemplate(template: GameTemplate | null): void {
  const next: Record<string, string> = {};
  if (template) {
    ports.value = template.container.ports.map((port) => ({
      host: port.host,
      container: port.container,
      protocol: port.protocol,
    }));

    for (const param of template.params) {
      next[param.key] = getDefaultParamValue(param);
    }

    initializeResourcesFromTemplate(template);
  } else {
    ports.value = [];
    initializeResourcesFromTemplate(null);
  }
  paramValues.value = next;
}

function roundFloat(value: number, digits = 2): number {
  const factor = 10 ** digits;
  return Math.round(value * factor) / factor;
}

function formatNumber(value: number, digits = 2): string {
  const fixed = value.toFixed(digits);
  return fixed.replace(/\.0+$/, "").replace(/(\.\d*?)0+$/, "$1");
}

function parseMemoryToMB(raw: string): number | null {
  const trimmed = raw.trim().toLowerCase();
  if (!trimmed) {
    return null;
  }

  const matched = trimmed.match(/^([0-9]*\.?[0-9]+)\s*([gmkb]?)$/);
  if (!matched) {
    return null;
  }

  const value = Number(matched[1]);
  if (!Number.isFinite(value) || value <= 0) {
    return null;
  }

  const unit = matched[2] || "b";
  switch (unit) {
    case "g":
      return value * 1024;
    case "m":
      return value;
    case "k":
      return value / 1024;
    case "b":
      return value / (1024 * 1024);
    default:
      return null;
  }
}

function getTemplateResourceBase(
  template: GameTemplate | null,
): { memoryMB: number; cpu: number } | null {
  const resources = template?.container.resources;
  if (!resources) {
    return null;
  }

  const memoryMB = parseMemoryToMB(resources.memory);
  if (memoryMB == null || memoryMB <= 0) {
    return null;
  }

  const cpu = Number(resources.cpu);
  if (!Number.isFinite(cpu) || cpu <= 0) {
    return null;
  }

  return { memoryMB, cpu };
}

function setResourceInputs(memoryMB: number, cpu: number): void {
  resourceMemoryValue.value = roundFloat(memoryMB / 1024, 2);
  resourceCPUValue.value = roundFloat(cpu, 2);
}

function initializeResourcesFromTemplate(template: GameTemplate | null): void {
  const base = getTemplateResourceBase(template);
  if (!base) {
    resourceEnabled.value = false;
    resourceMemoryValue.value = 0;
    resourceCPUValue.value = 0;
    return;
  }

  resourceEnabled.value = true;
  setResourceInputs(base.memoryMB, base.cpu);
}

function getDefaultParamValue(param: TemplateParam): string {
  if (typeof param.default === "boolean") {
    return param.default ? "true" : "false";
  }
  if (param.default == null) {
    return "";
  }
  return String(param.default);
}

function parseMinecraftVersion(raw: string): { major: number; minor: number; patch: number } {
  const trimmed = raw.trim().toUpperCase();
  if (!trimmed || trimmed === "LATEST") {
    return { major: 1, minor: 21, patch: 0 };
  }

  const values = trimmed.split(".").map((part) => Number(part));
  if (values.some((value) => !Number.isFinite(value))) {
    return { major: 1, minor: 21, patch: 0 };
  }

  return {
    major: values[0] ?? 1,
    minor: values[1] ?? 21,
    patch: values[2] ?? 0,
  };
}

function compareMinecraftVersion(
  a: { major: number; minor: number; patch: number },
  b: { major: number; minor: number; patch: number },
): number {
  if (a.major !== b.major) return a.major - b.major;
  if (a.minor !== b.minor) return a.minor - b.minor;
  return a.patch - b.patch;
}

function recommendMinecraftJavaTag(version: string, serverType: string): string {
  const parsed = parseMinecraftVersion(version);
  if (
    serverType === "FORGE" &&
    compareMinecraftVersion(parsed, { major: 1, minor: 18, patch: 0 }) < 0
  ) {
    return "java8";
  }
  if (compareMinecraftVersion(parsed, { major: 1, minor: 16, patch: 5 }) <= 0) {
    return "java8";
  }
  if (parsed.major === 1 && parsed.minor === 17) {
    return "java16";
  }
  if (compareMinecraftVersion(parsed, { major: 1, minor: 20, patch: 4 }) <= 0) {
    return "java17";
  }
  return "java21";
}

function formatMinecraftServerType(value: string): string {
  if (!value) {
    return "Vanilla";
  }
  const match = minecraftServerTypes.find((item) => item.value === value);
  return match?.label ?? value;
}

function generatedMinecraftName(): string {
  const version = minecraftVersion.value.trim() || "LATEST";
  const type = formatMinecraftServerType(minecraftServerType.value);
  const serverVersion = minecraftServerVersion.value.trim();
  if (showMinecraftServerVersion.value && serverVersion) {
    return `${version}-${type}_${serverVersion}`;
  }
  return `${version}-${type}`;
}

function syncMinecraftJavaTag(): void {
  if (!minecraftJavaTouched.value) {
    minecraftJavaTag.value = recommendedMinecraftJavaTag.value;
  }
}

function syncMinecraftName(): void {
  if (isMinecraftJava.value && !containerNameTouched.value) {
    containerName.value = generatedMinecraftName();
  }
}

function initMinecraftDefaults(): void {
  if (!isMinecraftJava.value) {
    return;
  }
  syncMinecraftJavaTag();
  syncMinecraftName();
}

async function fetchMinecraftVersions(): Promise<void> {
  minecraftVersionLoading.value = true;
  minecraftVersionLoadFailed.value = false;
  try {
    const versions = await listMinecraftVersions();
    const releaseVersions = versions
      .filter((item) => item.type === "release")
      .map((item) => item.id)
      .filter((value) => value.trim().length > 0);
    const snapshotVersions = versions
      .filter((item) => item.type !== "release")
      .map((item) => item.id)
      .filter((value) => value.trim().length > 0);
    minecraftVersionOptions.value = [...releaseVersions, ...snapshotVersions];
    if (
      minecraftVersionOptions.value.length > 0 &&
      !minecraftVersionOptions.value.includes(minecraftVersion.value)
    ) {
      minecraftVersion.value = minecraftVersionOptions.value[0];
    }
  } catch {
    minecraftVersionLoadFailed.value = true;
    minecraftVersionOptions.value = [];
  } finally {
    minecraftVersionLoading.value = false;
  }
}

async function fetchLoaderVersions(): Promise<void> {
  loaderVersionOptions.value = [];
  loaderVersionLoadFailed.value = false;
  if (!showMinecraftServerVersion.value) {
    loaderVersionLoading.value = false;
    loaderVersionCustom.value = false;
    return;
  }

  loaderVersionLoading.value = true;
  try {
    const versions = await listMinecraftLoaderVersions(
      minecraftVersion.value,
      minecraftServerType.value,
    );
    loaderVersionOptions.value = versions
      .map((item) => item.version)
      .filter((value) => value.trim().length > 0);
    if (loaderVersionOptions.value.length > 0) {
      if (!loaderVersionOptions.value.includes(minecraftServerVersion.value)) {
        minecraftServerVersion.value = loaderVersionOptions.value[0];
      }
      loaderVersionCustom.value = false;
    } else {
      loaderVersionCustom.value = true;
    }
  } catch {
    loaderVersionLoadFailed.value = true;
    loaderVersionCustom.value = true;
  } finally {
    loaderVersionLoading.value = false;
  }
}

function onMinecraftJavaTagChange(event: Event): void {
  const target = event.target as HTMLSelectElement;
  minecraftJavaTouched.value = true;
  minecraftJavaTag.value = target.value;
}

function useCustomLoaderVersion(): void {
  loaderVersionCustom.value = true;
}

function useListedLoaderVersion(): void {
  loaderVersionCustom.value = false;
  if (loaderVersionOptions.value.length > 0) {
    minecraftServerVersion.value = loaderVersionOptions.value[0];
  }
}

function onContainerNameInput(): void {
  containerNameTouched.value = true;
}

function isBooleanParamEnabled(key: string): boolean {
  return paramValues.value[key] === "true";
}

function onBooleanParamChange(key: string, event: Event): void {
  const target = event.target as HTMLInputElement;
  paramValues.value = {
    ...paramValues.value,
    [key]: target.checked ? "true" : "false",
  };
}

function getCreateParamsPayload(): Record<string, string> {
  if (isMinecraftJava.value) {
    const payload: Record<string, string> = {
      MC_VERSION: minecraftVersion.value.trim(),
      JAVA_TAG: minecraftJavaTag.value,
      JAVA_TAG_SOURCE: minecraftJavaTouched.value ? "manual" : "auto",
    };
    if (minecraftServerType.value) {
      payload.SERVER_TYPE = minecraftServerType.value;
    }
    if (showMinecraftServerVersion.value && minecraftServerVersion.value.trim()) {
      payload.SERVER_VERSION = minecraftServerVersion.value.trim();
    }
    return payload;
  }

  const template = currentTemplate.value;
  if (!template) {
    return {};
  }

  const payload: Record<string, string> = {};
  for (const param of template.params) {
    const value = paramValues.value[param.key];
    if (typeof value === "string") {
      payload[param.key] = value;
    }
  }
  return payload;
}

function getCreatePortsPayload(): PortMapping[] {
  return ports.value.map((port) => ({
    host: Number(port.host),
    container: Number(port.container),
    protocol: port.protocol,
  }));
}

function getCreateResourcesPayload(): ResourceLimits | undefined {
  if (!resourceEnabled.value) {
    return undefined;
  }

  const memoryValue = Number(resourceMemoryValue.value);
  const cpuValue = Number(resourceCPUValue.value);
  const memory = `${formatNumber(memoryValue)}g`;

  return {
    memory,
    cpu: roundFloat(cpuValue, 2),
  };
}

function validateResources(resources: ResourceLimits | undefined): boolean {
  if (!resources) {
    return true;
  }

  const memoryMB = parseMemoryToMB(resources.memory);
  if (memoryMB == null || memoryMB < 256) {
    pageErrorKey.value = "errors.invalidResourceLimits";
    return false;
  }

  if (!Number.isFinite(resources.cpu) || resources.cpu < 0.5) {
    pageErrorKey.value = "errors.invalidResourceLimits";
    return false;
  }

  return true;
}

function cancelCreate(): void {
  void router.push({ name: "ImageRegistry" });
}

async function handleCreate(): Promise<void> {
  const name = containerName.value.trim();
  if (!name) {
    pageErrorKey.value = "status.emptyName";
    return;
  }

  const gameID = currentGameID.value.trim();
  if (!gameID) {
    pageErrorKey.value = "errors.gameNotFound";
    return;
  }

  pageErrorKey.value = "";
  creating.value = true;
  containerStore.print(t("status.creating"));

  if (
    isMinecraftJava.value &&
    showMinecraftServerVersion.value &&
    !minecraftServerVersion.value.trim()
  ) {
    pageErrorKey.value = "createPage.serverVersionRequired";
    creating.value = false;
    return;
  }

  const resourcesPayload = getCreateResourcesPayload();
  if (!validateResources(resourcesPayload)) {
    creating.value = false;
    return;
  }

  const success = await containerStore.create(
    name,
    gameID,
    getCreateParamsPayload(),
    getCreatePortsPayload(),
    resourcesPayload,
  );
  creating.value = false;
  if (!success) {
    pageErrorKey.value = containerStore.outputI18n?.key ?? "errors.unknown";
    return;
  }

  void router.push({ name: "ContainerList" });
}

watch([minecraftVersion, minecraftServerType], () => {
  syncMinecraftJavaTag();
  if (!showMinecraftServerVersion.value) {
    minecraftServerVersion.value = "";
  }
  if (isMinecraftJava.value) {
    void fetchLoaderVersions();
  }
});

watch([minecraftVersion, minecraftServerType, minecraftServerVersion], () => {
  syncMinecraftName();
});
</script>

<template>
  <header class="page-header">
    <h1 class="page-title">{{ $t("createPage.title") }}</h1>
  </header>

  <main class="main-content">
    <div v-if="loading" class="state-message">
      {{ $t("createPage.loading") }}
    </div>

    <div v-else-if="pageErrorKey" class="state-message state-error">
      {{ $t(pageErrorKey) }}
    </div>

    <section v-else-if="currentGame" class="form-panel">
      <header class="form-header">
        <h2 class="game-title">{{ currentGame.name }}</h2>
        <p class="game-description">{{ currentGame.description }}</p>
      </header>

      <div v-if="!isMinecraftJava" class="field-block">
        <label class="field-label" for="instance-name">{{ $t("createPage.nameLabel") }}</label>
        <input
          id="instance-name"
          v-model="containerName"
          class="text-input"
          :placeholder="$t('createPage.namePlaceholder')"
          @input="onContainerNameInput"
          @keyup.enter="handleCreate"
        />
      </div>

      <div v-if="gameStore.templateLoading" class="state-message compact">
        {{ $t("createPage.loadingTemplate") }}
      </div>

      <div v-else-if="currentTemplate" class="field-block">
        <template v-if="isMinecraftJava">
          <h3 class="section-title">{{ $t("createPage.minecraftTitle") }}</h3>

          <div class="param-list">
            <article class="param-item">
              <label class="field-label" for="minecraft-version">{{
                $t("createPage.minecraftVersionLabel")
              }}</label>
              <select id="minecraft-version" v-model="minecraftVersion" class="text-input">
                <option
                  v-for="version in selectableMinecraftVersions"
                  :key="version"
                  :value="version"
                >
                  {{ version }}
                </option>
              </select>
              <p v-if="minecraftVersionLoading" class="field-hint">
                {{ $t("createPage.loadingVersions") }}
              </p>
              <p v-else-if="minecraftVersionLoadFailed" class="field-hint">
                {{ $t("createPage.versionLoadFailed") }}
              </p>
            </article>

            <article class="param-item">
              <label class="field-label" for="minecraft-server-type">{{
                $t("createPage.serverTypeLabel")
              }}</label>
              <select id="minecraft-server-type" v-model="minecraftServerType" class="text-input">
                <option
                  v-for="serverType in minecraftServerTypes"
                  :key="serverType.value || 'VANILLA'"
                  :value="serverType.value"
                >
                  {{
                    serverType.value
                      ? serverType.label
                      : $t("createPage.noLoaderOption", { type: serverType.label })
                  }}
                </option>
              </select>
            </article>

            <article v-if="showMinecraftServerVersion" class="param-item">
              <label class="field-label" for="minecraft-server-version">{{
                $t("createPage.serverVersionLabel")
              }}</label>
              <select
                v-if="loaderVersionOptions.length > 0 && !loaderVersionCustom"
                id="minecraft-server-version"
                v-model="minecraftServerVersion"
                class="text-input"
              >
                <option v-for="version in loaderVersionOptions" :key="version" :value="version">
                  {{ version }}
                </option>
              </select>
              <input
                v-else
                id="minecraft-server-version"
                v-model="minecraftServerVersion"
                class="text-input"
                type="text"
                :placeholder="$t('createPage.serverVersionPlaceholder')"
              />
              <div class="inline-actions">
                <button
                  v-if="loaderVersionOptions.length > 0 && !loaderVersionCustom"
                  class="inline-btn"
                  type="button"
                  @click="useCustomLoaderVersion"
                >
                  {{ $t("createPage.customVersion") }}
                </button>
                <button
                  v-else-if="loaderVersionOptions.length > 0"
                  class="inline-btn"
                  type="button"
                  @click="useListedLoaderVersion"
                >
                  {{ $t("createPage.useVersionList") }}
                </button>
              </div>
              <p v-if="loaderVersionLoading" class="field-hint">
                {{ $t("createPage.loadingLoaderVersions") }}
              </p>
              <p v-else-if="loaderVersionLoadFailed" class="field-hint">
                {{ $t("createPage.loaderVersionLoadFailed") }}
              </p>
            </article>

            <article class="param-item">
              <label class="field-label" for="minecraft-java-tag">{{
                $t("createPage.javaVersionLabel")
              }}</label>
              <select
                id="minecraft-java-tag"
                class="text-input"
                :value="minecraftJavaTag"
                @change="onMinecraftJavaTagChange"
              >
                <option v-for="tag in minecraftJavaTags" :key="tag.value" :value="tag.value">
                  {{ tag.label }}
                </option>
              </select>
              <p class="field-hint">
                {{
                  $t("createPage.javaRecommendation", {
                    java: recommendedMinecraftJavaTag.replace("java", "Java "),
                  })
                }}
              </p>
            </article>

            <article class="param-item">
              <label class="field-label" for="instance-name">{{
                $t("createPage.nameLabel")
              }}</label>
              <input
                id="instance-name"
                v-model="containerName"
                class="text-input"
                :placeholder="$t('createPage.namePlaceholder')"
                @input="onContainerNameInput"
                @keyup.enter="handleCreate"
              />
              <p class="field-hint">{{ $t("createPage.generatedNameHint") }}</p>
            </article>
          </div>
        </template>

        <h3 class="section-title">{{ $t("createPage.portsTitle") }}</h3>

        <div v-if="ports.length > 0" class="param-list">
          <article
            v-for="(port, index) in ports"
            :key="`${port.container}-${port.protocol}-${index}`"
            class="param-item"
          >
            <label class="field-label" :for="`port-${index}`">{{
              $t("createPage.hostPortLabel")
            }}</label>
            <input
              :id="`port-${index}`"
              v-model.number="ports[index].host"
              class="text-input"
              type="number"
              min="1"
            />
            <p class="field-hint">
              {{
                $t("createPage.containerPortHint", {
                  container: port.container,
                  protocol: port.protocol,
                })
              }}
            </p>
          </article>
        </div>

        <h3 class="section-title">{{ $t("createPage.resourcesTitle") }}</h3>

        <div v-if="resourceEnabled" class="resource-block">
          <article class="param-item">
            <label class="field-label" for="resource-memory">{{
              $t("createPage.memoryLabel")
            }}</label>
            <div class="resource-input-row">
              <input
                id="resource-memory"
                v-model.number="resourceMemoryValue"
                class="text-input resource-number-input"
                type="number"
                min="0.25"
                step="0.25"
              />
              <span class="resource-unit-badge">GB</span>
            </div>

            <label class="field-label" for="resource-cpu">{{ $t("createPage.cpuLabel") }}</label>
            <input
              id="resource-cpu"
              v-model.number="resourceCPUValue"
              class="text-input resource-number-input"
              type="number"
              min="0.5"
              step="0.1"
            />

            <p class="field-hint">{{ $t("createPage.resourcesHint") }}</p>
          </article>
        </div>

        <div v-else class="state-message compact">
          {{ $t("createPage.resourcesUnavailable") }}
        </div>

        <h3 v-if="!isMinecraftJava" class="section-title">{{ $t("createPage.paramsTitle") }}</h3>

        <div
          v-if="!isMinecraftJava && currentTemplate.params.length === 0"
          class="state-message compact"
        >
          {{ $t("createPage.noParams") }}
        </div>

        <div v-else-if="!isMinecraftJava" class="param-list">
          <article v-for="param in currentTemplate.params" :key="param.key" class="param-item">
            <label class="field-label" :for="`param-${param.key}`">{{ param.label }}</label>
            <p v-if="param.description" class="field-hint">{{ param.description }}</p>

            <input
              v-if="param.type === 'string'"
              :id="`param-${param.key}`"
              v-model="paramValues[param.key]"
              class="text-input"
              type="text"
            />

            <input
              v-else-if="param.type === 'number'"
              :id="`param-${param.key}`"
              v-model="paramValues[param.key]"
              class="text-input"
              type="number"
            />

            <label
              v-else-if="param.type === 'boolean'"
              class="boolean-field"
              :for="`param-${param.key}`"
            >
              <input
                :id="`param-${param.key}`"
                type="checkbox"
                :checked="isBooleanParamEnabled(param.key)"
                @change="onBooleanParamChange(param.key, $event)"
              />
              <span>
                {{
                  isBooleanParamEnabled(param.key)
                    ? $t("createPage.booleanEnabled")
                    : $t("createPage.booleanDisabled")
                }}
              </span>
            </label>

            <select
              v-else-if="param.type === 'select'"
              :id="`param-${param.key}`"
              v-model="paramValues[param.key]"
              class="text-input"
            >
              <option
                v-for="option in param.options || []"
                :key="option.value"
                :value="option.value"
              >
                {{ option.label }}
              </option>
            </select>
          </article>
        </div>
      </div>

      <div class="actions">
        <button class="btn-cancel" type="button" @click="cancelCreate">
          {{ $t("createPage.cancel") }}
        </button>
        <button class="btn-confirm" type="button" :disabled="creating" @click="handleCreate">
          {{ creating ? $t("createPage.creating") : $t("createPage.confirm") }}
        </button>
      </div>
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
  max-width: 920px;
  margin: 0 auto;
  width: 100%;
  gap: 14px;
}

.form-panel {
  background: var(--card-bg);
  border: 3px solid var(--card-border);
  border-radius: 0;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
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
}

.form-header {
  border-bottom: 1px solid var(--create-border-outer);
  padding-bottom: 10px;
}

.game-title {
  margin: 0;
  color: var(--card-text);
  font-size: 20px;
}

.game-description {
  margin: 8px 0 0;
  color: #3b3b3b;
  line-height: 1.5;
  font-size: 13px;
}

.field-block {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.field-label {
  font-size: 13px;
  color: var(--card-text);
  font-weight: 600;
}

.field-hint {
  margin: 0;
  font-size: 12px;
  color: #555555;
}

.section-title {
  margin: 0;
  font-size: 15px;
  color: var(--card-text);
  border-bottom: 2px solid var(--card-border-inner);
  padding-bottom: 4px;
}

.param-list {
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
}

.resource-block {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.resource-input-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.resource-unit-badge {
  min-width: 64px;
  height: 34px;
  padding: 0 10px;
  border: 2px solid var(--card-border);
  border-radius: 0;
  background: var(--create-brass-dark);
  color: var(--card-text);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  font-weight: bold;
  width: 100px;
  flex-shrink: 0;
  box-shadow: 2px 2px 0 0 var(--create-border-outer);
}

.resource-number-input {
  appearance: textfield;
  -moz-appearance: textfield;
}

.resource-number-input::-webkit-outer-spin-button,
.resource-number-input::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.param-item {
  border: 2px solid var(--card-border-inner);
  border-radius: 0;
  padding: 10px;
  background: rgba(0, 0, 0, 0.03);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.text-input {
  padding: 8px 10px;
  background: #ffffff;
  border: 2px solid var(--card-border);
  color: var(--card-text);
  border-radius: 0;
  outline: none;
  width: 100%;
  box-shadow: inset 2px 2px 0 0 rgba(0, 0, 0, 0.1);
}

.text-input:focus {
  border-color: var(--create-border-outer);
}

.boolean-field {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--card-text);
}

.inline-actions {
  display: flex;
  justify-content: flex-start;
}

.inline-btn {
  padding: 6px 10px;
  font-size: 12px;
  background: var(--card-bg);
  border: 2px solid var(--card-border);
  color: var(--card-text);
  border-radius: 0;
  cursor: pointer;
  box-shadow: 2px 2px 0 0 var(--create-border-outer);
  font-weight: bold;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 8px;
}

.btn-cancel {
  padding: 8px 14px;
  font-size: 13px;
  background: var(--card-bg);
  border: 2px solid var(--card-border);
  color: var(--card-text);
  border-radius: 0;
  cursor: pointer;
  box-shadow: 2px 2px 0 0 var(--create-border-outer);
  transition: all 0.2s ease;
  font-weight: bold;
}

.btn-cancel:hover {
  transform: translate(1px, 1px);
  box-shadow: 1px 1px 0 0 var(--create-border-outer);
  background: var(--create-brass-dark);
}

.btn-confirm {
  padding: 8px 14px;
  font-size: 13px;
  background: var(--success);
  color: #fff;
  border: 2px solid var(--card-border);
  border-radius: 0;
  cursor: pointer;
  box-shadow: 2px 2px 0 0 var(--create-border-outer);
  transition: all 0.2s ease;
  font-weight: bold;
}

.btn-confirm:hover {
  transform: translate(1px, 1px);
  box-shadow: 1px 1px 0 0 var(--create-border-outer);
  filter: brightness(1.1);
}

.btn-confirm:disabled {
  opacity: 0.55;
  cursor: not-allowed;
  transform: none;
  box-shadow: 2px 2px 0 0 var(--create-border-outer);
}

.state-message {
  text-align: center;
  color: var(--text-muted);
  border: 2px dashed var(--border-muted);
  border-radius: 0;
  padding: 40px 16px;
  background: rgba(0, 0, 0, 0.2);
}

.state-message.compact {
  padding: 16px;
}

.state-error {
  color: var(--danger);
  border-color: rgba(255, 77, 79, 0.5);
  background: var(--danger-light);
}

@media (max-width: 1023px) {
  .page-title {
    display: none;
  }
}

@media (max-width: 767px) {
  .main-content {
    padding: 8px 16px 20px 16px;
  }

  .resource-grid {
    grid-template-columns: 1fr;
  }

  .actions {
    flex-direction: column-reverse;
  }

  .btn-cancel,
  .btn-confirm {
    width: 100%;
  }
}
</style>
