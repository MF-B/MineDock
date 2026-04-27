<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import type { GameTemplate, PortMapping, ResourceLimits, TemplateParam } from "../api/index";
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

const currentGame = computed(() => {
  return gameStore.getGameById(currentGameID.value) ?? null;
});

const currentTemplate = computed<GameTemplate | null>(() => {
  if (!currentGameID.value || gameStore.currentTemplateGameID !== currentGameID.value) {
    return null;
  }
  return gameStore.currentTemplate;
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

      <div class="field-block">
        <label class="field-label" for="instance-name">{{ $t("createPage.nameLabel") }}</label>
        <input
          id="instance-name"
          v-model="containerName"
          class="text-input"
          :placeholder="$t('createPage.namePlaceholder')"
          @keyup.enter="handleCreate"
        />
      </div>

      <div v-if="gameStore.templateLoading" class="state-message compact">
        {{ $t("createPage.loadingTemplate") }}
      </div>

      <div v-else-if="currentTemplate" class="field-block">
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

        <h3 class="section-title">{{ $t("createPage.paramsTitle") }}</h3>

        <div v-if="currentTemplate.params.length === 0" class="state-message compact">
          {{ $t("createPage.noParams") }}
        </div>

        <div v-else class="param-list">
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
  border: 1px solid var(--create-border-outer);
  background: rgba(0, 0, 0, 0.18);
  border-radius: 8px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.form-header {
  border-bottom: 1px solid var(--create-border-outer);
  padding-bottom: 10px;
}

.game-title {
  margin: 0;
  color: var(--create-brass-primary);
  font-size: 20px;
}

.game-description {
  margin: 8px 0 0;
  color: var(--text-muted);
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
  color: var(--text-on-dark);
}

.field-hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
}

.section-title {
  margin: 0;
  font-size: 15px;
  color: var(--create-brass-primary);
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
  border: 1px solid var(--input-border);
  border-radius: 4px;
  background: var(--input-bg);
  color: var(--text-on-dark);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  width: 100px;
  flex-shrink: 0;
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
  border: 1px solid var(--create-border-outer);
  border-radius: 6px;
  padding: 10px;
  background: rgba(0, 0, 0, 0.14);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.text-input {
  padding: 8px 10px;
  background: var(--input-bg);
  border: 1px solid var(--input-border);
  color: var(--text-on-dark);
  border-radius: 4px;
  outline: none;
  width: 100%;
}

.text-input:focus {
  border-color: var(--create-brass-primary);
}

.boolean-field {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text-on-dark);
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
  background: transparent;
  border: 1px solid var(--btn-secondary-border);
  color: var(--btn-secondary-text);
  border-radius: 4px;
  cursor: pointer;
}

.btn-cancel:hover {
  background: var(--hover-lighten);
}

.btn-confirm {
  padding: 8px 14px;
  font-size: 13px;
  background: var(--create-brass-dark);
  color: var(--text-on-dark);
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

.btn-confirm:hover {
  filter: brightness(1.1);
}

.btn-confirm:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.state-message {
  text-align: center;
  color: var(--text-muted);
  border: 1px dashed var(--border-muted);
  border-radius: 8px;
  padding: 40px 16px;
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
