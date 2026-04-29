<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";

interface ServerMetricSample {
  timestamp: number;
  cpuPercent: number;
  cpuCores: number[];
  memoryPercent: number;
  memoryInMbps: number;
  memoryOutMbps: number;
  diskPercent: number;
  diskReadMbps: number;
  diskWriteMbps: number;
  networkRxMbps: number;
  networkTxMbps: number;
}

type MetricPanel = "cpu" | "memory" | "network" | `disk-${number}`;
type MetricValueKey =
  | "memoryInMbps"
  | "memoryOutMbps"
  | "diskReadMbps"
  | "diskWriteMbps"
  | "networkRxMbps"
  | "networkTxMbps";

const MAX_SAMPLES = 60;
const CPU_CORE_COUNT = 8;
const CHART_TOP = 4;
const CHART_BOTTOM = 38;
const MEMORY_TOTAL_GB = 32;
const CPU_MODEL = "AMD Ryzen 7 5800X";
const MEMORY_MODEL = "DDR4-3200 32 GB";
const DISK_DEVICES = [
  {
    label: "硬盘 1",
    name: "Samsung 980 PRO 1 TB",
    totalGb: 1024,
    usageOffset: 3,
    readFactor: 0.62,
    writeFactor: 0.58,
  },
  {
    label: "硬盘 2",
    name: "WD Blue SN570 500 GB",
    totalGb: 512,
    usageOffset: -8,
    readFactor: 0.38,
    writeFactor: 0.42,
  },
];
const NETWORK_MODEL = "Intel I225-V 2.5GbE";
const CPU_LINE_COLORS = [
  "#f5cb6e",
  "#76d6ff",
  "#10b981",
  "#ff8a65",
  "#c084fc",
  "#fef08a",
  "#8f6051",
  "#f472b6",
];

const samples = ref<ServerMetricSample[]>([]);
const expandedPanels = ref<Set<MetricPanel>>(new Set());
let timerId: number | undefined;

const latestSample = computed<ServerMetricSample>(() => {
  const latest = samples.value[samples.value.length - 1];
  return latest ?? createInitialSample(Date.now());
});

const memoryUsedGb = computed(() => {
  return (latestSample.value.memoryPercent / 100) * MEMORY_TOTAL_GB;
});

const networkPeak = computed(() => {
  const peak = samples.value.reduce((max, sample) => {
    return Math.max(max, sample.networkRxMbps, sample.networkTxMbps);
  }, 1);
  return Math.max(peak, 24);
});

const memoryIoPeak = computed(() => {
  const peak = samples.value.reduce((max, sample) => {
    return Math.max(max, sample.memoryInMbps, sample.memoryOutMbps);
  }, 1);
  return Math.max(peak, 48);
});

const diskIoPeak = computed(() => {
  const peak = samples.value.reduce((max, sample) => {
    return Math.max(max, sample.diskReadMbps, sample.diskWriteMbps);
  }, 1);
  return Math.max(peak, 96);
});

const cpuCoreLines = computed(() => {
  return Array.from({ length: CPU_CORE_COUNT }, (_, index) => {
    return {
      index,
      color: CPU_LINE_COLORS[index % CPU_LINE_COLORS.length],
      value: latestSample.value.cpuCores[index] ?? 0,
      points: buildCustomLinePoints(
        samples.value.map((sample) => sample.cpuCores[index] ?? 0),
        100,
      ),
    };
  });
});

const diskRows = computed(() => {
  return DISK_DEVICES.map((device, index) => {
    const percent = clamp(latestSample.value.diskPercent + device.usageOffset, 0, 100);
    const readMbps = latestSample.value.diskReadMbps * device.readFactor;
    const writeMbps = latestSample.value.diskWriteMbps * device.writeFactor;
    return {
      ...device,
      panel: `disk-${index}` as MetricPanel,
      percent,
      usedGb: (percent / 100) * device.totalGb,
      readMbps,
      writeMbps,
      readPoints: buildScaledLinePoints(
        samples.value,
        "diskReadMbps",
        device.readFactor,
        diskIoPeak.value,
      ),
      writePoints: buildScaledLinePoints(
        samples.value,
        "diskWriteMbps",
        device.writeFactor,
        diskIoPeak.value,
      ),
    };
  });
});

const memoryInLine = computed(() => {
  return buildLinePoints(samples.value, "memoryInMbps", memoryIoPeak.value);
});

const memoryOutLine = computed(() => {
  return buildLinePoints(samples.value, "memoryOutMbps", memoryIoPeak.value);
});

const networkRxLine = computed(() => {
  return buildLinePoints(samples.value, "networkRxMbps", networkPeak.value);
});

const networkTxLine = computed(() => {
  return buildLinePoints(samples.value, "networkTxMbps", networkPeak.value);
});

function createInitialSample(timestamp: number): ServerMetricSample {
  const cpuCores = Array.from({ length: CPU_CORE_COUNT }, (_, index) => 34 + index * 2);

  return {
    timestamp,
    cpuPercent: average(cpuCores),
    cpuCores,
    memoryPercent: 58,
    memoryInMbps: 26,
    memoryOutMbps: 14,
    diskPercent: 63,
    diskReadMbps: 41,
    diskWriteMbps: 24,
    networkRxMbps: 13,
    networkTxMbps: 7,
  };
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function drift(value: number, min: number, max: number, delta: number): number {
  return clamp(value + (Math.random() - 0.5) * delta, min, max);
}

function average(values: number[]): number {
  if (values.length === 0) {
    return 0;
  }
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

function nextSample(previous: ServerMetricSample, timestamp: number): ServerMetricSample {
  const activityWave = Math.sin(timestamp / 6500) * 4;
  const cpuCores = Array.from({ length: CPU_CORE_COUNT }, (_, index) => {
    const previousValue = previous.cpuCores[index] ?? previous.cpuPercent;
    const corePhase = Math.sin(timestamp / (4200 + index * 460)) * (index % 2 === 0 ? 5 : -4);
    return drift(previousValue + activityWave * 0.1 + corePhase * 0.08, 8, 98, 16);
  });

  return {
    timestamp,
    cpuPercent: average(cpuCores),
    cpuCores,
    memoryPercent: drift(previous.memoryPercent, 44, 82, 4),
    memoryInMbps: drift(previous.memoryInMbps + activityWave * 0.08, 4, 86, 12),
    memoryOutMbps: drift(previous.memoryOutMbps - activityWave * 0.05, 2, 72, 10),
    diskPercent: drift(previous.diskPercent, 59, 68, 0.7),
    diskReadMbps: drift(previous.diskReadMbps + activityWave * 0.16, 2, 150, 28),
    diskWriteMbps: drift(previous.diskWriteMbps - activityWave * 0.1, 1, 120, 20),
    networkRxMbps: drift(previous.networkRxMbps + activityWave * 0.08, 2, 36, 8),
    networkTxMbps: drift(previous.networkTxMbps - activityWave * 0.05, 1, 24, 5),
  };
}

function initializeSamples(): void {
  const now = Date.now();
  const seed: ServerMetricSample[] = [];
  let current = createInitialSample(now - (MAX_SAMPLES - 1) * 1000);

  for (let index = 0; index < MAX_SAMPLES; index += 1) {
    current = nextSample(current, now - (MAX_SAMPLES - 1 - index) * 1000);
    seed.push(current);
  }

  samples.value = seed;
}

function appendSample(): void {
  const previous = samples.value[samples.value.length - 1] ?? createInitialSample(Date.now());
  samples.value = [...samples.value.slice(-(MAX_SAMPLES - 1)), nextSample(previous, Date.now())];
}

function buildLinePoints(
  source: ServerMetricSample[],
  valueKey: MetricValueKey,
  maxValue: number,
): string {
  return buildCustomLinePoints(
    source.map((sample) => sample[valueKey]),
    maxValue,
  );
}

function buildScaledLinePoints(
  source: ServerMetricSample[],
  valueKey: MetricValueKey,
  factor: number,
  maxValue: number,
): string {
  return buildCustomLinePoints(
    source.map((sample) => sample[valueKey] * factor),
    maxValue,
  );
}

function buildCustomLinePoints(values: number[], maxValue: number): string {
  if (values.length === 0) {
    return "";
  }

  const divisor = Math.max(values.length - 1, 1);
  const height = CHART_BOTTOM - CHART_TOP;

  return values
    .map((value, index) => {
      const x = (index / divisor) * 100;
      const normalized = clamp(value / maxValue, 0, 1);
      const y = CHART_BOTTOM - normalized * height;
      return `${x.toFixed(2)},${y.toFixed(2)}`;
    })
    .join(" ");
}

function formatPercent(value: number): string {
  return String(Math.round(value));
}

function formatDecimal(value: number, digits = 1): string {
  return value.toFixed(digits);
}

function togglePanel(panel: MetricPanel): void {
  const nextPanels = new Set(expandedPanels.value);
  if (nextPanels.has(panel)) {
    nextPanels.delete(panel);
  } else {
    nextPanels.add(panel);
  }
  expandedPanels.value = nextPanels;
}

onMounted(() => {
  initializeSamples();
  timerId = window.setInterval(appendSample, 1000);
});

onUnmounted(() => {
  if (timerId !== undefined) {
    window.clearInterval(timerId);
  }
});
</script>

<template>
  <header class="page-header">
    <h1 class="page-title">{{ $t("monitor.title") }}</h1>
  </header>

  <main class="main-content">
    <section class="metric-list" :aria-label="$t('monitor.summary')">
      <article class="metric-card" :class="{ 'is-expanded': expandedPanels.has('cpu') }">
        <button
          class="metric-row"
          type="button"
          :aria-expanded="expandedPanels.has('cpu')"
          @click="togglePanel('cpu')"
        >
          <span class="metric-name">{{ $t("monitor.cpu") }}</span>
          <span class="metric-percent">
            <strong class="metric-value">{{ formatPercent(latestSample.cpuPercent) }}</strong>
            <strong class="metric-value">%</strong>
          </span>
          <span class="metric-unit-group">
            <strong class="metric-value">{{ CPU_CORE_COUNT }}</strong>
            <span class="metric-meta">{{ $t("monitor.logicalCores") }}</span>
          </span>
          <span class="metric-io metric-empty"></span>
          <span class="metric-io metric-empty"></span>
          <span class="expand-indicator">{{ expandedPanels.has("cpu") ? "▲" : "▼" }}</span>
        </button>

        <div v-if="expandedPanels.has('cpu')" class="metric-detail">
          <article class="device-section">
            <div class="detail-toolbar">
              <span class="hardware-name">{{ CPU_MODEL }}</span>
            </div>
            <div class="cpu-core-grid">
              <article v-for="line in cpuCoreLines" :key="line.index" class="mini-chart-card">
                <svg
                  class="chart mini-chart"
                  viewBox="0 0 100 42"
                  preserveAspectRatio="none"
                  role="img"
                  :aria-label="$t('monitor.cpuCore', { index: line.index })"
                >
                  <line class="chart-grid-line" x1="0" y1="21" x2="100" y2="21" />
                  <polyline
                    class="chart-line"
                    :points="line.points"
                    :style="{ stroke: line.color }"
                  />
                </svg>
              </article>
            </div>
          </article>
        </div>
      </article>

      <article class="metric-card" :class="{ 'is-expanded': expandedPanels.has('memory') }">
        <button
          class="metric-row"
          type="button"
          :aria-expanded="expandedPanels.has('memory')"
          @click="togglePanel('memory')"
        >
          <span class="metric-name">{{ $t("monitor.memory") }}</span>
          <span class="metric-percent">
            <strong class="metric-value">{{ formatPercent(latestSample.memoryPercent) }}</strong>
            <strong class="metric-value">%</strong>
          </span>
          <span class="metric-unit-group">
            <strong class="metric-value"
              >{{ formatDecimal(memoryUsedGb) }} / {{ MEMORY_TOTAL_GB }}</strong
            >
            <span class="metric-meta">GB</span>
          </span>
          <span class="metric-io">
            <strong class="metric-value">{{ formatDecimal(latestSample.memoryInMbps) }}</strong>
            <span class="metric-meta">{{ $t("monitor.inputUnit") }}</span>
          </span>
          <span class="metric-io">
            <strong class="metric-value">{{ formatDecimal(latestSample.memoryOutMbps) }}</strong>
            <span class="metric-meta">{{ $t("monitor.outputUnit") }}</span>
          </span>
          <span class="expand-indicator">{{ expandedPanels.has("memory") ? "▲" : "▼" }}</span>
        </button>

        <div v-if="expandedPanels.has('memory')" class="metric-detail">
          <article class="wide-chart-panel">
            <div class="detail-toolbar">
              <span class="hardware-name">{{ MEMORY_MODEL }}</span>
              <div class="chart-legend">
                <span><i class="legend-mark memory-in-mark"></i>{{ $t("monitor.in") }}</span>
                <span><i class="legend-mark out-mark"></i>{{ $t("monitor.out") }}</span>
              </div>
            </div>
            <svg
              class="chart detail-chart"
              viewBox="0 0 100 42"
              preserveAspectRatio="none"
              aria-hidden="true"
            >
              <line class="chart-grid-line" x1="0" y1="13" x2="100" y2="13" />
              <line class="chart-grid-line" x1="0" y1="26" x2="100" y2="26" />
              <polyline class="chart-line memory-in-line" :points="memoryInLine" />
              <polyline class="chart-line memory-out-line" :points="memoryOutLine" />
            </svg>
          </article>
        </div>
      </article>

      <article
        v-for="disk in diskRows"
        :key="disk.label"
        class="metric-card"
        :class="{ 'is-expanded': expandedPanels.has(disk.panel) }"
      >
        <button
          class="metric-row"
          type="button"
          :aria-expanded="expandedPanels.has(disk.panel)"
          @click="togglePanel(disk.panel)"
        >
          <span class="metric-name">{{ disk.label }}</span>
          <span class="metric-percent">
            <strong class="metric-value">{{ formatPercent(disk.percent) }}</strong>
            <strong class="metric-value">%</strong>
          </span>
          <span class="metric-unit-group">
            <strong class="metric-value"
              >{{ formatDecimal(disk.usedGb, 0) }} / {{ disk.totalGb }}</strong
            >
            <span class="metric-meta">GB</span>
          </span>
          <span class="metric-io">
            <strong class="metric-value">{{ formatDecimal(disk.readMbps) }}</strong>
            <span class="metric-meta">{{ $t("monitor.inputUnit") }}</span>
          </span>
          <span class="metric-io">
            <strong class="metric-value">{{ formatDecimal(disk.writeMbps) }}</strong>
            <span class="metric-meta">{{ $t("monitor.outputUnit") }}</span>
          </span>
          <span class="expand-indicator">{{ expandedPanels.has(disk.panel) ? "▲" : "▼" }}</span>
        </button>

        <div v-if="expandedPanels.has(disk.panel)" class="metric-detail">
          <article class="wide-chart-panel">
            <div class="detail-toolbar">
              <span class="hardware-name">{{ disk.name }}</span>
              <div class="chart-legend">
                <span><i class="legend-mark disk-read-mark"></i>{{ $t("monitor.in") }}</span>
                <span><i class="legend-mark out-mark"></i>{{ $t("monitor.out") }}</span>
              </div>
            </div>
            <svg
              class="chart detail-chart"
              viewBox="0 0 100 42"
              preserveAspectRatio="none"
              aria-hidden="true"
            >
              <line class="chart-grid-line" x1="0" y1="13" x2="100" y2="13" />
              <line class="chart-grid-line" x1="0" y1="26" x2="100" y2="26" />
              <polyline class="chart-line disk-read-line" :points="disk.readPoints" />
              <polyline class="chart-line disk-write-line" :points="disk.writePoints" />
            </svg>
          </article>
        </div>
      </article>

      <article class="metric-card" :class="{ 'is-expanded': expandedPanels.has('network') }">
        <button
          class="metric-row"
          type="button"
          :aria-expanded="expandedPanels.has('network')"
          @click="togglePanel('network')"
        >
          <span class="metric-name">{{ $t("monitor.network") }}</span>
          <span class="metric-empty"></span>
          <span class="metric-empty"></span>
          <span class="metric-io">
            <strong class="metric-value">{{ formatDecimal(latestSample.networkRxMbps) }}</strong>
            <span class="metric-meta">{{ $t("monitor.inputUnit") }}</span>
          </span>
          <span class="metric-io">
            <strong class="metric-value">{{ formatDecimal(latestSample.networkTxMbps) }}</strong>
            <span class="metric-meta">{{ $t("monitor.outputUnit") }}</span>
          </span>
          <span class="expand-indicator">{{ expandedPanels.has("network") ? "▲" : "▼" }}</span>
        </button>

        <div v-if="expandedPanels.has('network')" class="metric-detail">
          <article class="wide-chart-panel">
            <div class="detail-toolbar">
              <span class="hardware-name">{{ NETWORK_MODEL }}</span>
              <div class="chart-legend">
                <span><i class="legend-mark network-in-mark"></i>{{ $t("monitor.in") }}</span>
                <span><i class="legend-mark out-mark"></i>{{ $t("monitor.out") }}</span>
              </div>
            </div>
            <svg
              class="chart detail-chart"
              viewBox="0 0 100 42"
              preserveAspectRatio="none"
              aria-hidden="true"
            >
              <line class="chart-grid-line" x1="0" y1="13" x2="100" y2="13" />
              <line class="chart-grid-line" x1="0" y1="26" x2="100" y2="26" />
              <polyline class="chart-line network-rx-line" :points="networkRxLine" />
              <polyline class="chart-line network-tx-line" :points="networkTxLine" />
            </svg>
          </article>
        </div>
      </article>
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
  position: relative;
  padding: 0 80px 0 24px;
}

.page-title {
  margin: 0;
  color: var(--create-brass-primary);
  font-size: 16px;
  font-weight: bold;
  letter-spacing: 2px;
}

.main-content {
  flex: 1;
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
  padding: 8px 24px 24px;
}

.metric-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.metric-card {
  background: var(--card-bg);
  border: 3px solid var(--card-border);
  border-radius: 0;
  box-shadow:
    inset 0 3px 0 0 var(--card-bg),
    inset 0 -3px 0 0 var(--card-bg),
    inset 0 6px 0 0 var(--card-border-inner),
    inset 0 -6px 0 0 var(--card-border-inner);
}

.metric-card {
  transition: filter 0.2s;
}

.metric-card:hover,
.metric-card.is-expanded {
  filter: brightness(0.96);
}

.metric-row {
  box-sizing: border-box;
  width: 100%;
  min-height: 68px;
  padding: 10px 20px;
  display: grid;
  grid-template-columns:
    minmax(72px, 80px) minmax(72px, 100px) minmax(130px, 150px) minmax(104px, 120px)
    minmax(104px, 120px) 18px;
  align-items: center;
  column-gap: clamp(12px, 1.8vw, 24px);
  justify-content: space-between;
  row-gap: 10px;
  border: 0;
  background: transparent;
  color: var(--card-text);
  cursor: pointer;
  font: inherit;
  text-align: left;
}

.metric-name {
  color: var(--card-text);
  font-size: 19px;
  font-weight: 800;
}

.metric-value {
  color: var(--card-text);
  font-size: 17px;
  font-weight: 700;
  line-height: 1;
  white-space: nowrap;
}

.metric-percent,
.metric-unit-group,
.metric-io {
  display: flex;
  align-items: baseline;
  line-height: 1;
  white-space: nowrap;
}

.metric-percent {
  gap: 0;
}

.metric-io {
  justify-content: flex-end;
  gap: 12px;
}

.metric-unit-group {
  gap: 6px;
}

.metric-meta {
  color: var(--text-muted);
  font-size: 13px;
  white-space: nowrap;
}

.expand-indicator {
  color: var(--card-text);
  font-size: 11px;
  grid-column: 6;
  text-align: center;
}

.metric-row > .metric-percent,
.metric-row > .metric-unit-group,
.metric-row > .metric-meta,
.metric-row > .metric-io {
  text-align: right;
}

.metric-row > .metric-percent {
  justify-self: start;
}

.metric-row > .metric-unit-group,
.metric-row > .metric-meta,
.metric-row > .metric-io {
  justify-self: end;
}

.metric-empty {
  min-height: 1px;
}

.metric-detail {
  padding: 0 16px 16px;
}

.cpu-core-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.mini-chart-card,
.wide-chart-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.mini-chart-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.detail-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.hardware-name {
  min-width: 0;
  color: var(--card-text);
  font-size: 13px;
  font-weight: bold;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mini-chart-title {
  margin: 0;
  color: var(--card-text);
  font-size: 13px;
}

.mini-chart-value {
  color: var(--create-bg);
  font-size: 14px;
  white-space: nowrap;
}

.chart {
  width: 100%;
  border: 3px solid var(--card-border);
  background:
    linear-gradient(90deg, rgba(252, 246, 189, 0.08) 1px, transparent 1px),
    linear-gradient(180deg, rgba(252, 246, 189, 0.07) 1px, transparent 1px), #17241f;
  background-size:
    12.5% 100%,
    100% 50%;
}

.detail-chart {
  min-height: 260px;
}

.mini-chart {
  height: 76px;
}

.chart-grid-line {
  stroke: rgba(252, 246, 189, 0.16);
  stroke-width: 0.45;
  vector-effect: non-scaling-stroke;
}

.chart-line {
  fill: none;
  stroke-width: 2.2;
  stroke-linejoin: round;
  stroke-linecap: square;
  vector-effect: non-scaling-stroke;
}

.memory-in-line {
  stroke: var(--success);
}

.memory-out-line {
  stroke: var(--create-brass-secondary);
}

.disk-read-line {
  stroke: #8f6051;
}

.disk-write-line {
  stroke: var(--create-brass-secondary);
}

.network-rx-line {
  stroke: #76d6ff;
}

.network-tx-line {
  stroke: var(--create-brass-secondary);
}

.chart-legend {
  display: flex;
  flex-shrink: 0;
  justify-content: flex-end;
  gap: 18px;
  color: var(--card-text);
  font-size: 12px;
  font-weight: bold;
}

.legend-mark {
  display: inline-block;
  width: 12px;
  height: 12px;
  margin-right: 6px;
  vertical-align: -2px;
  border: 2px solid var(--card-border);
}

.memory-in-mark {
  background: var(--success);
}

.disk-read-mark {
  background: #8f6051;
}

.network-in-mark {
  background: #76d6ff;
}

.out-mark {
  background: var(--create-brass-secondary);
}

@media (max-width: 1023px) {
  .page-header {
    padding: 0 80px 0 64px;
  }

  .page-title {
    display: none;
  }

  .cpu-core-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }
}

@media (max-width: 767px) {
  .main-content {
    padding: 8px 12px 12px;
  }

  .metric-row {
    align-items: flex-start;
    grid-template-columns: 1fr 18px;
    gap: 10px;
  }

  .metric-row > .metric-name {
    grid-column: 1;
  }

  .metric-row > .expand-indicator {
    grid-column: 2;
    grid-row: 1;
    justify-self: end;
  }

  .metric-row > .metric-percent,
  .metric-row > .metric-unit-group,
  .metric-row > .metric-meta,
  .metric-row > .metric-io {
    grid-column: 1 / -1;
    justify-self: start;
    text-align: left;
  }

  .cpu-core-grid {
    grid-template-columns: 1fr;
  }

  .detail-chart {
    min-height: 220px;
  }

  .detail-toolbar {
    align-items: flex-start;
    flex-direction: column;
    gap: 8px;
  }

  .chart-legend {
    justify-content: flex-start;
  }
}
</style>
