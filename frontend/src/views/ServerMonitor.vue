<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { getServerMetrics, type ServerMetrics } from "../api/index";

interface DiskMetricSample {
  id: string;
  label: string;
  name: string;
  percent: number;
  usedGb: number;
  totalGb: number;
  readMbps: number;
  writeMbps: number;
}

interface ServerMetricSample {
  timestamp: number;
  cpuPercent: number;
  cpuCores: number[];
  logicalCores?: number;
  cpuModel?: string;
  memoryPercent: number;
  memoryUsedGb?: number;
  memoryTotalGb?: number;
  memoryInMbps: number;
  memoryOutMbps: number;
  memoryModel?: string;
  disks?: DiskMetricSample[];
  diskPercent: number;
  diskReadMbps: number;
  diskWriteMbps: number;
  networkRxMbps: number;
  networkTxMbps: number;
  networkModel?: string;
}

type MetricPanel = "cpu" | "memory" | "network" | `disk-${string}`;
type MetricValueKey = "memoryInMbps" | "memoryOutMbps" | "networkRxMbps" | "networkTxMbps";

const MAX_SAMPLES = 60;
const BYTES_PER_GIB = 1024 ** 3;
const BYTES_PER_MIB = 1024 ** 2;
const CHART_TOP = 4;
const CHART_BOTTOM = 38;
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
const loading = ref(true);
const loadError = ref(false);
let timerId: number | undefined;
let fetchInFlight = false;
let disposed = false;

const emptySample: ServerMetricSample = {
  timestamp: Date.now(),
  cpuPercent: 0,
  cpuCores: [],
  logicalCores: 0,
  cpuModel: "CPU",
  memoryPercent: 0,
  memoryUsedGb: 0,
  memoryTotalGb: 0,
  memoryInMbps: 0,
  memoryOutMbps: 0,
  memoryModel: "System Memory",
  disks: [],
  diskPercent: 0,
  diskReadMbps: 0,
  diskWriteMbps: 0,
  networkRxMbps: 0,
  networkTxMbps: 0,
  networkModel: "Network",
};

const latestSample = computed<ServerMetricSample>(() => {
  const latest = samples.value[samples.value.length - 1];
  return latest ?? emptySample;
});

const memoryUsedGb = computed(() => {
  return latestSample.value.memoryUsedGb ?? 0;
});

const memoryTotalGb = computed(() => {
  return latestSample.value.memoryTotalGb ?? 0;
});

const cpuCoreCount = computed(() => {
  return Math.max(latestSample.value.logicalCores ?? 0, latestSample.value.cpuCores.length);
});

const networkPeak = computed(() => {
  const peak = samples.value.reduce((max, sample) => {
    return Math.max(max, sample.networkRxMbps, sample.networkTxMbps);
  }, 1);
  return Math.max(peak, 1);
});

const memoryIoPeak = computed(() => {
  const peak = samples.value.reduce((max, sample) => {
    return Math.max(max, sample.memoryInMbps, sample.memoryOutMbps);
  }, 1);
  return Math.max(peak, 1);
});

const diskIoPeak = computed(() => {
  const peak = samples.value.reduce((max, sample) => {
    const dynamicPeak = (sample.disks ?? []).reduce((diskMax, disk) => {
      return Math.max(diskMax, disk.readMbps, disk.writeMbps);
    }, 0);
    return Math.max(max, dynamicPeak, sample.diskReadMbps, sample.diskWriteMbps);
  }, 1);
  return Math.max(peak, 1);
});

const cpuCoreLines = computed(() => {
  return Array.from({ length: cpuCoreCount.value }, (_, index) => {
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
  return (latestSample.value.disks ?? []).map((disk) => {
    return {
      ...disk,
      panel: `disk-${disk.id}` as MetricPanel,
      readPoints: buildCustomLinePoints(
        samples.value.map((sample) => findDiskSample(sample, disk.id)?.readMbps ?? 0),
        diskIoPeak.value,
      ),
      writePoints: buildCustomLinePoints(
        samples.value.map((sample) => findDiskSample(sample, disk.id)?.writeMbps ?? 0),
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

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
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
  return String(Math.round(clamp(finiteNumber(value), 0, 100)));
}

function formatDecimal(value: number, digits = 1): string {
  return finiteNumber(value).toFixed(digits);
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

function findDiskSample(sample: ServerMetricSample, diskId: string): DiskMetricSample | undefined {
  return (sample.disks ?? []).find((disk) => disk.id === diskId);
}

function finiteNumber(value: number): number {
  return Number.isFinite(value) ? value : 0;
}

function bytesToGiB(bytes: number): number {
  return finiteNumber(bytes) / BYTES_PER_GIB;
}

function bytesPerSecondToMiB(bytesPerSecond: number): number {
  return finiteNumber(bytesPerSecond) / BYTES_PER_MIB;
}

function toSample(metrics: ServerMetrics): ServerMetricSample {
  const disks = metrics.disks.map((disk, index) => {
    return {
      id: disk.id || `disk-${index}`,
      label: disk.label || `Disk ${index + 1}`,
      name: disk.name || disk.mountpoint || disk.id || `Disk ${index + 1}`,
      percent: finiteNumber(disk.percent),
      usedGb: bytesToGiB(disk.used_bytes),
      totalGb: bytesToGiB(disk.total_bytes),
      readMbps: bytesPerSecondToMiB(disk.read_bps),
      writeMbps: bytesPerSecondToMiB(disk.write_bps),
    };
  });
  const diskCount = Math.max(disks.length, 1);
  const diskPercent = disks.reduce((sum, disk) => sum + disk.percent, 0) / diskCount;
  const diskReadMbps = disks.reduce((sum, disk) => sum + disk.readMbps, 0);
  const diskWriteMbps = disks.reduce((sum, disk) => sum + disk.writeMbps, 0);

  return {
    timestamp: metrics.timestamp || Date.now(),
    cpuPercent: finiteNumber(metrics.cpu.percent),
    cpuCores: metrics.cpu.cores.map(finiteNumber),
    logicalCores: Math.max(0, Math.trunc(finiteNumber(metrics.cpu.logical_cores))),
    cpuModel: metrics.cpu.model || "CPU",
    memoryPercent: finiteNumber(metrics.memory.percent),
    memoryUsedGb: bytesToGiB(metrics.memory.used_bytes),
    memoryTotalGb: bytesToGiB(metrics.memory.total_bytes),
    memoryInMbps: bytesPerSecondToMiB(metrics.memory.swap_in_bps),
    memoryOutMbps: bytesPerSecondToMiB(metrics.memory.swap_out_bps),
    memoryModel: metrics.memory.model || "System Memory",
    disks,
    diskPercent,
    diskReadMbps,
    diskWriteMbps,
    networkRxMbps: bytesPerSecondToMiB(metrics.network.rx_bps),
    networkTxMbps: bytesPerSecondToMiB(metrics.network.tx_bps),
    networkModel: metrics.network.name || "Network",
  };
}

function appendServerSample(sample: ServerMetricSample): void {
  samples.value = [...samples.value.slice(-(MAX_SAMPLES - 1)), sample];
}

async function fetchSample(): Promise<void> {
  if (fetchInFlight) {
    return;
  }

  fetchInFlight = true;
  loading.value = samples.value.length === 0;
  try {
    const metrics = await getServerMetrics();
    if (!disposed) {
      appendServerSample(toSample(metrics));
      loadError.value = false;
    }
  } catch {
    if (!disposed) {
      loadError.value = true;
    }
  } finally {
    if (!disposed) {
      loading.value = false;
    }
    fetchInFlight = false;
  }
}

onMounted(() => {
  void fetchSample();
  timerId = window.setInterval(() => {
    void fetchSample();
  }, 1000);
});

onUnmounted(() => {
  disposed = true;
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
    <div v-if="loading && samples.length === 0" class="monitor-state">
      {{ $t("monitor.loading") }}
    </div>
    <div v-else-if="loadError && samples.length === 0" class="monitor-state state-error">
      {{ $t("monitor.loadError") }}
    </div>
    <div v-else-if="loadError" class="monitor-state state-error">
      {{ $t("monitor.loadError") }}
    </div>

    <section v-if="samples.length > 0" class="metric-list" :aria-label="$t('monitor.summary')">
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
            <strong class="metric-value">{{ cpuCoreCount }}</strong>
            <span class="metric-meta">{{ $t("monitor.logicalCores") }}</span>
          </span>
          <span class="metric-io metric-empty"></span>
          <span class="metric-io metric-empty"></span>
          <span class="expand-indicator">{{ expandedPanels.has("cpu") ? "▲" : "▼" }}</span>
        </button>

        <div v-if="expandedPanels.has('cpu')" class="metric-detail">
          <article class="device-section">
            <div class="detail-toolbar">
              <span class="hardware-name">{{ latestSample.cpuModel }}</span>
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
              >{{ formatDecimal(memoryUsedGb) }} / {{ formatDecimal(memoryTotalGb, 0) }}</strong
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
              <span class="hardware-name">{{ latestSample.memoryModel }}</span>
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
        :key="disk.id"
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
              >{{ formatDecimal(disk.usedGb, 0) }} / {{ formatDecimal(disk.totalGb, 0) }}</strong
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
              <span class="hardware-name">{{ latestSample.networkModel }}</span>
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

.monitor-state {
  margin: 0 0 16px;
  padding: 16px;
  color: var(--card-text);
  font-size: 14px;
  font-weight: bold;
  text-align: center;
  background: var(--card-bg);
  border: 3px solid var(--card-border);
}

.state-error {
  color: var(--danger);
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
