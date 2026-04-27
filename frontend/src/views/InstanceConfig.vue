<script setup lang="ts">
import { computed, ref, watch } from "vue";
import {
  ApiRequestError,
  getGameTemplate,
  getInstanceConfig,
  type GameTemplate,
  type InstanceConfig as InstanceConfigPayload,
  type PortMapping,
  type ResourceLimits,
  type TemplateParam,
  updateInstanceConfig,
} from "../api/index";

const props = defineProps<{ containerId: string }>();

const emit = defineEmits<{
  reconfigured: [newContainerId: string];
}>();

const loading = ref(false);
const saving = ref(false);
const loadErrorKey = ref("");
const saveErrorKey = ref("");
const saveSuccess = ref(false);

const config = ref<InstanceConfigPayload | null>(null);
const template = ref<GameTemplate | null>(null);
const ports = ref<PortMapping[]>([]);
const values = ref<Record<string, string>>({});
const resourceEnabled = ref(false);
const resourceMemoryValue = ref(0);
const resourceCPUValue = ref(0);

const isRunning = computed(() => {
  const status = config.value?.status ?? "";
  const normalized = status.trim().toLowerCase();
  return normalized === "running" || normalized.startsWith("up");
});

const editable = computed(() => !loading.value && !saving.value && !isRunning.value);
const hasEditableFields = computed(() => {
  const paramCount = template.value?.params.length ?? 0;
  return ports.value.length > 0 || paramCount > 0 || resourceEnabled.value;
});

watch(
  () => props.containerId,
  () => {
    void loadConfig();
  },
  { immediate: true },
);

function mapRequestErrorToKey(error: unknown): string {
  if (error instanceof ApiRequestError) {
    const backendMessage = error.backendMessage?.trim().toLowerCase() ?? "";
    if (backendMessage.includes("container must be stopped")) {
      return "errors.containerNotStopped";
    }
    if (backendMessage.includes("invalid container id")) {
      return "errors.invalidContainerId";
    }

    if (error.key === "errors.network") {
      return "errors.network";
    }

    if (error.key === "errors.httpStatus") {
      switch (error.status) {
        case 400:
          return "errors.badRequest";
        case 404:
          return "errors.notFound";
        case 409:
          return "errors.conflict";
        case 500:
          return "errors.internal";
        default:
          return "errors.unknown";
      }
    }

    if (error.key) {
      return error.key;
    }
  }

  return "errors.unknown";
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

function initializeValues(
  currentTemplate: GameTemplate,
  currentConfig: InstanceConfigPayload,
): void {
  const next: Record<string, string> = {};
  for (const param of currentTemplate.params) {
    const fromConfig = currentConfig.params[param.key];
    if (typeof fromConfig === "string") {
      next[param.key] = fromConfig;
      continue;
    }
    next[param.key] = getDefaultParamValue(param);
  }
  values.value = next;
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

function setResourceInputs(memoryMB: number, cpu: number): void {
  resourceMemoryValue.value = roundFloat(memoryMB / 1024, 2);
  resourceCPUValue.value = roundFloat(cpu, 2);
}

function initializeResources(
  currentTemplate: GameTemplate,
  currentConfig: InstanceConfigPayload,
): void {
  const source = currentConfig.resources ?? currentTemplate.container.resources;
  if (!source) {
    resourceEnabled.value = false;
    resourceMemoryValue.value = 0;
    resourceCPUValue.value = 0;
    return;
  }

  const memoryMB = parseMemoryToMB(source.memory);
  const cpu = Number(source.cpu);
  if (memoryMB == null || memoryMB <= 0 || !Number.isFinite(cpu) || cpu <= 0) {
    resourceEnabled.value = false;
    resourceMemoryValue.value = 0;
    resourceCPUValue.value = 0;
    return;
  }

  resourceEnabled.value = true;
  setResourceInputs(memoryMB, cpu);
}

function isBooleanParamEnabled(key: string): boolean {
  return values.value[key] === "true";
}

function onBooleanParamChange(key: string, event: Event): void {
  const target = event.target as HTMLInputElement;
  values.value = {
    ...values.value,
    [key]: target.checked ? "true" : "false",
  };
}

function buildParamsPayload(): Record<string, string> {
  const currentTemplate = template.value;
  if (!currentTemplate) {
    return {};
  }

  const payload: Record<string, string> = {};
  for (const param of currentTemplate.params) {
    const value = values.value[param.key];
    payload[param.key] = typeof value === "string" ? value : "";
  }
  return payload;
}

function buildPortsPayload(): PortMapping[] {
  return ports.value.map((port) => ({
    host: Number(port.host),
    container: Number(port.container),
    protocol: port.protocol,
  }));
}

function buildResourcesPayload(): ResourceLimits | undefined {
  if (!resourceEnabled.value) {
    return undefined;
  }

  const memoryValue = Number(resourceMemoryValue.value);
  const cpuValue = Number(resourceCPUValue.value);

  return {
    memory: `${formatNumber(memoryValue)}g`,
    cpu: roundFloat(cpuValue, 2),
  };
}

function validateResources(resources: ResourceLimits | undefined): boolean {
  if (!resources) {
    return true;
  }

  const memoryMB = parseMemoryToMB(resources.memory);
  if (memoryMB == null || memoryMB < 256) {
    saveErrorKey.value = "errors.invalidResourceLimits";
    return false;
  }

  if (!Number.isFinite(resources.cpu) || resources.cpu < 0.5) {
    saveErrorKey.value = "errors.invalidResourceLimits";
    return false;
  }

  return true;
}

async function loadConfig(): Promise<void> {
  loading.value = true;
  saving.value = false;
  loadErrorKey.value = "";
  saveErrorKey.value = "";
  saveSuccess.value = false;
  config.value = null;
  template.value = null;
  ports.value = [];
  values.value = {};
  resourceEnabled.value = false;
  resourceMemoryValue.value = 0;
  resourceCPUValue.value = 0;

  const id = props.containerId.trim();
  if (!id) {
    loadErrorKey.value = "errors.invalidContainerId";
    loading.value = false;
    return;
  }

  let currentConfig: InstanceConfigPayload;
  try {
    currentConfig = await getInstanceConfig(id);
    config.value = currentConfig;
    ports.value = Array.isArray(currentConfig.ports)
      ? currentConfig.ports.map((item) => ({
          host: item.host,
          container: item.container,
          protocol: item.protocol,
        }))
      : [];
  } catch (error) {
    loadErrorKey.value = mapRequestErrorToKey(error);
    loading.value = false;
    return;
  }

  try {
    const currentTemplate = await getGameTemplate(currentConfig.game_id);
    template.value = currentTemplate;
    initializeValues(currentTemplate, currentConfig);
    initializeResources(currentTemplate, currentConfig);
  } catch {
    loadErrorKey.value = "config.noTemplate";
  }

  loading.value = false;
}

async function handleSave(): Promise<void> {
  if (!editable.value) {
    return;
  }
  if (!template.value || !config.value) {
    return;
  }

  saving.value = true;
  saveErrorKey.value = "";
  saveSuccess.value = false;

  const resources = buildResourcesPayload();
  if (!validateResources(resources)) {
    saving.value = false;
    return;
  }

  try {
    const resp = await updateInstanceConfig(
      props.containerId,
      buildParamsPayload(),
      buildPortsPayload(),
      resources,
    );
    saveSuccess.value = true;
    emit("reconfigured", resp.container_id);
  } catch (error) {
    saveErrorKey.value = mapRequestErrorToKey(error);
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <section class="config-panel">
    <header class="config-header">
      <h2 class="config-title">{{ $t("config.title") }}</h2>
      <p class="config-note">{{ $t("config.restartNote") }}</p>
    </header>

    <div v-if="loading" class="state-message">{{ $t("config.loading") }}</div>

    <div v-else-if="loadErrorKey" class="state-message state-error">{{ $t(loadErrorKey) }}</div>

    <template v-else-if="template && config">
      <div v-if="isRunning" class="state-message state-warning">
        {{ $t("config.mustStopFirst") }}
      </div>

      <div v-if="ports.length > 0" class="port-list">
        <h3 class="section-title">{{ $t("config.portsTitle") }}</h3>
        <article
          v-for="(port, index) in ports"
          :key="`${port.container}-${port.protocol}-${index}`"
          class="port-item"
        >
          <label class="field-label" :for="`cfg-port-${index}`">{{
            $t("config.hostPortLabel")
          }}</label>
          <input
            :id="`cfg-port-${index}`"
            v-model.number="ports[index].host"
            class="text-input"
            type="number"
            min="1"
            :disabled="!editable"
          />
          <p class="field-hint">
            {{
              $t("config.containerPortHint", {
                container: port.container,
                protocol: port.protocol,
              })
            }}
          </p>
        </article>
      </div>

      <div class="port-list">
        <h3 class="section-title">{{ $t("config.resourcesTitle") }}</h3>

        <article v-if="resourceEnabled" class="port-item">
          <label class="field-label" for="cfg-memory">{{ $t("config.memoryLabel") }}</label>
          <div class="resource-input-row">
            <input
              id="cfg-memory"
              v-model.number="resourceMemoryValue"
              class="text-input resource-number-input"
              type="number"
              min="0.25"
              step="0.25"
              :disabled="!editable"
            />
            <span class="resource-unit-badge">GB</span>
          </div>

          <label class="field-label" for="cfg-cpu">{{ $t("config.cpuLabel") }}</label>
          <input
            id="cfg-cpu"
            v-model.number="resourceCPUValue"
            class="text-input resource-number-input"
            type="number"
            min="0.5"
            step="0.1"
            :disabled="!editable"
          />

          <p class="field-hint">{{ $t("config.resourcesHint") }}</p>
        </article>

        <div v-else class="state-message">{{ $t("config.resourcesUnavailable") }}</div>
      </div>

      <div v-if="template.params.length === 0" class="state-message">
        {{ $t("registry.noParams") }}
      </div>

      <div v-else class="param-list">
        <article v-for="param in template.params" :key="param.key" class="param-item">
          <label class="field-label" :for="`cfg-${param.key}`">{{ param.label }}</label>
          <p v-if="param.description" class="field-hint">{{ param.description }}</p>

          <input
            v-if="param.type === 'string'"
            :id="`cfg-${param.key}`"
            v-model="values[param.key]"
            class="text-input"
            type="text"
            :disabled="!editable"
          />

          <input
            v-else-if="param.type === 'number'"
            :id="`cfg-${param.key}`"
            v-model="values[param.key]"
            class="text-input"
            type="number"
            :disabled="!editable"
          />

          <label
            v-else-if="param.type === 'boolean'"
            class="boolean-field"
            :for="`cfg-${param.key}`"
          >
            <input
              :id="`cfg-${param.key}`"
              type="checkbox"
              :checked="isBooleanParamEnabled(param.key)"
              :disabled="!editable"
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
            :id="`cfg-${param.key}`"
            v-model="values[param.key]"
            class="text-input"
            :disabled="!editable"
          >
            <option v-for="option in param.options || []" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </article>
      </div>

      <footer class="actions">
        <button
          class="save-btn"
          type="button"
          :disabled="!editable || !hasEditableFields"
          @click="handleSave"
        >
          {{ saving ? $t("config.saving") : $t("config.save") }}
        </button>
      </footer>

      <p v-if="saveSuccess" class="save-success">{{ $t("config.saveSuccess") }}</p>
      <p v-if="saveErrorKey" class="save-error">{{ $t(saveErrorKey) }}</p>
    </template>
  </section>
</template>

<style scoped>
.config-panel {
  border: 1px solid var(--create-border-outer);
  background: rgba(0, 0, 0, 0.18);
  border-radius: 8px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
}

.config-header {
  display: flex;
  flex-direction: column;
  gap: 6px;
  border-bottom: 1px solid var(--create-border-outer);
  padding-bottom: 10px;
}

.config-title {
  margin: 0;
  color: var(--create-brass-primary);
  font-size: 18px;
}

.config-note {
  margin: 0;
  color: var(--text-muted);
  font-size: 12px;
}

.state-message {
  text-align: center;
  color: var(--text-muted);
  border: 1px dashed var(--border-muted);
  border-radius: 8px;
  padding: 16px;
}

.state-error {
  color: var(--danger);
  border-color: rgba(255, 77, 79, 0.5);
  background: var(--danger-light);
}

.state-warning {
  color: var(--create-brass-primary);
  border-color: var(--create-border-outer);
  background: rgba(0, 0, 0, 0.24);
}

.param-list {
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
}

.port-list {
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
}

.section-title {
  margin: 0;
  font-size: 15px;
  color: var(--create-brass-primary);
}

.port-item {
  border: 1px solid var(--create-border-outer);
  border-radius: 6px;
  padding: 10px;
  background: rgba(0, 0, 0, 0.14);
  display: flex;
  flex-direction: column;
  gap: 8px;
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

.field-label {
  font-size: 13px;
  color: var(--text-on-dark);
}

.field-hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
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
}

.save-btn {
  padding: 8px 16px;
  border: 1px solid var(--create-border-outer);
  background: var(--create-brass-dark);
  color: var(--card-text);
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
}

.save-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.save-success {
  margin: 0;
  color: var(--success);
  font-size: 13px;
}

.save-error {
  margin: 0;
  color: var(--danger);
  font-size: 13px;
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
</style>
