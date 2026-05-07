<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { getContainerStats, type ContainerStats } from "../api/index";

const props = defineProps<{ containerId: string }>();

const MAX_SAMPLES = 60;
const BYTES_PER_MIB = 1024 ** 2;
const BYTES_PER_GIB = 1024 ** 3;
const CHART_TOP = 4;
const CHART_BOTTOM = 38;

interface MetricSample {
  timestamp: number;
  cpuPercent: number;
  memoryPercent: number;
  memoryUsedGb: number;
  memoryMaxGb: number;
  networkRxBytes: number;
  networkTxBytes: number;
  diskReadBytes: number;
  diskWriteBytes: number;
}

interface RateSample {
  netRxMbps: number;
  netTxMbps: number;
  diskReadMbps: number;
  diskWriteMbps: number;
}

const samples = ref<MetricSample[]>([]);
const rates = ref<RateSample[]>([]);
const loading = ref(true);
const loadError = ref(false);
const notRunning = ref(false);
let timerId: number | undefined;
let fetchInFlight = false;
let disposed = false;

const latestSample = computed<MetricSample>(() => {
  const last = samples.value[samples.value.length - 1];
  return (
    last ?? {
      timestamp: 0,
      cpuPercent: 0,
      memoryPercent: 0,
      memoryUsedGb: 0,
      memoryMaxGb: 0,
      networkRxBytes: 0,
      networkTxBytes: 0,
      diskReadBytes: 0,
      diskWriteBytes: 0,
    }
  );
});

const latestRate = computed<RateSample>(() => {
  const last = rates.value[rates.value.length - 1];
  return last ?? { netRxMbps: 0, netTxMbps: 0, diskReadMbps: 0, diskWriteMbps: 0 };
});

const networkPeak = computed(() => {
  const peak = rates.value.reduce((max, r) => Math.max(max, r.netRxMbps, r.netTxMbps), 1);
  return Math.max(peak, 0.01);
});

const diskPeak = computed(() => {
  const peak = rates.value.reduce((max, r) => Math.max(max, r.diskReadMbps, r.diskWriteMbps), 1);
  return Math.max(peak, 0.01);
});

const cpuLine = computed(() =>
  buildLinePoints(
    samples.value.map((s) => s.cpuPercent),
    100,
  ),
);
const memoryLine = computed(() =>
  buildLinePoints(
    samples.value.map((s) => s.memoryPercent),
    100,
  ),
);
const netRxLine = computed(() =>
  buildLinePoints(
    rates.value.map((r) => r.netRxMbps),
    networkPeak.value,
  ),
);
const netTxLine = computed(() =>
  buildLinePoints(
    rates.value.map((r) => r.netTxMbps),
    networkPeak.value,
  ),
);
const diskReadLine = computed(() =>
  buildLinePoints(
    rates.value.map((r) => r.diskReadMbps),
    diskPeak.value,
  ),
);
const diskWriteLine = computed(() =>
  buildLinePoints(
    rates.value.map((r) => r.diskWriteMbps),
    diskPeak.value,
  ),
);

function clamp(v: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, v));
}

function finiteNumber(v: number): number {
  return Number.isFinite(v) ? v : 0;
}

function formatPercent(v: number): string {
  return String(Math.round(clamp(finiteNumber(v), 0, 100)));
}

function formatDecimal(v: number, digits = 1): string {
  return finiteNumber(v).toFixed(digits);
}

function buildLinePoints(values: number[], maxValue: number): string {
  if (values.length === 0) return "";
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

function toSample(raw: ContainerStats): MetricSample {
  return {
    timestamp: raw.timestamp,
    cpuPercent: finiteNumber(raw.cpu_percent),
    memoryPercent: finiteNumber(raw.memory_percent),
    memoryUsedGb: finiteNumber(raw.memory_used_bytes) / BYTES_PER_GIB,
    memoryMaxGb: finiteNumber(raw.memory_max_bytes) / BYTES_PER_GIB,
    networkRxBytes: raw.network_rx_bytes,
    networkTxBytes: raw.network_tx_bytes,
    diskReadBytes: raw.disk_read_bytes,
    diskWriteBytes: raw.disk_write_bytes,
  };
}

// 通过相邻两次采样的差值计算速率（MB/s）。
function computeRate(current: MetricSample, previous: MetricSample | undefined): RateSample {
  if (!previous || current.timestamp <= previous.timestamp) {
    return { netRxMbps: 0, netTxMbps: 0, diskReadMbps: 0, diskWriteMbps: 0 };
  }
  const dt = (current.timestamp - previous.timestamp) / 1000;
  if (dt <= 0) {
    return { netRxMbps: 0, netTxMbps: 0, diskReadMbps: 0, diskWriteMbps: 0 };
  }
  const rateOf = (cur: number, prev: number) => {
    const delta = cur - prev;
    return delta >= 0 ? delta / dt / BYTES_PER_MIB : 0;
  };
  return {
    netRxMbps: rateOf(current.networkRxBytes, previous.networkRxBytes),
    netTxMbps: rateOf(current.networkTxBytes, previous.networkTxBytes),
    diskReadMbps: rateOf(current.diskReadBytes, previous.diskReadBytes),
    diskWriteMbps: rateOf(current.diskWriteBytes, previous.diskWriteBytes),
  };
}

function appendSample(sample: MetricSample): void {
  const prev = samples.value[samples.value.length - 1];
  samples.value = [...samples.value.slice(-(MAX_SAMPLES - 1)), sample];
  rates.value = [...rates.value.slice(-(MAX_SAMPLES - 1)), computeRate(sample, prev)];
}

async function fetchSample(): Promise<void> {
  if (fetchInFlight) return;
  fetchInFlight = true;
  loading.value = samples.value.length === 0;
  try {
    const raw = await getContainerStats(props.containerId);
    if (!disposed) {
      appendSample(toSample(raw));
      loadError.value = false;
      notRunning.value = false;
    }
  } catch (e) {
    if (!disposed) {
      // 容器未运行时 Docker stats 会返回错误
      const msg = String(e);
      if (msg.includes("is not running") || msg.includes("No such container")) {
        notRunning.value = true;
      } else {
        loadError.value = true;
      }
    }
  } finally {
    if (!disposed) loading.value = false;
    fetchInFlight = false;
  }
}

function startPolling(): void {
  stopPolling();
  disposed = false;
  samples.value = [];
  rates.value = [];
  void fetchSample();
  timerId = window.setInterval(() => {
    void fetchSample();
  }, 1000);
}

function stopPolling(): void {
  disposed = true;
  if (timerId !== undefined) {
    window.clearInterval(timerId);
    timerId = undefined;
  }
}

watch(
  () => props.containerId,
  () => {
    startPolling();
  },
);

onMounted(() => {
  startPolling();
});

onUnmounted(() => {
  stopPolling();
});
</script>

<template>
  <section class="instance-monitor">
    <div v-if="loading && samples.length === 0" class="monitor-state">
      {{ $t("instanceMonitor.loading") }}
    </div>
    <div v-else-if="notRunning && samples.length === 0" class="monitor-state state-warn">
      {{ $t("instanceMonitor.notRunning") }}
    </div>
    <div v-else-if="loadError && samples.length === 0" class="monitor-state state-error">
      {{ $t("instanceMonitor.loadError") }}
    </div>

    <section v-if="samples.length > 0" class="metric-list">
      <!-- CPU -->
      <article class="metric-card">
        <div class="metric-row">
          <span class="metric-name">{{ $t("instanceMonitor.cpu") }}</span>
          <span class="metric-percent">
            <strong class="metric-value">{{ formatPercent(latestSample.cpuPercent) }}</strong>
            <strong class="metric-value">%</strong>
          </span>
        </div>
        <div class="chart-panel">
          <svg
            class="chart"
            viewBox="0 0 100 42"
            preserveAspectRatio="none"
            role="img"
            :aria-label="$t('instanceMonitor.cpuUsage')"
          >
            <line class="chart-grid-line" x1="0" y1="13" x2="100" y2="13" />
            <line class="chart-grid-line" x1="0" y1="26" x2="100" y2="26" />
            <polyline class="chart-line cpu-line" :points="cpuLine" />
          </svg>
        </div>
      </article>

      <!-- Memory -->
      <article class="metric-card">
        <div class="metric-row">
          <span class="metric-name">{{ $t("instanceMonitor.memory") }}</span>
          <span class="metric-percent">
            <strong class="metric-value">{{ formatPercent(latestSample.memoryPercent) }}</strong>
            <strong class="metric-value">%</strong>
          </span>
          <span class="metric-unit-group">
            <strong class="metric-value">
              {{ formatDecimal(latestSample.memoryUsedGb) }} /
              {{ formatDecimal(latestSample.memoryMaxGb, 0) }}
            </strong>
            <span class="metric-meta">GB</span>
          </span>
        </div>
        <div class="chart-panel">
          <svg
            class="chart"
            viewBox="0 0 100 42"
            preserveAspectRatio="none"
            role="img"
            :aria-label="$t('instanceMonitor.memoryUsage')"
          >
            <line class="chart-grid-line" x1="0" y1="13" x2="100" y2="13" />
            <line class="chart-grid-line" x1="0" y1="26" x2="100" y2="26" />
            <polyline class="chart-line memory-line" :points="memoryLine" />
          </svg>
        </div>
      </article>

      <!-- Network I/O -->
      <article class="metric-card">
        <div class="metric-row">
          <span class="metric-name">{{ $t("instanceMonitor.network") }}</span>
          <span class="metric-io">
            <strong class="metric-value">{{ formatDecimal(latestRate.netRxMbps) }}</strong>
            <span class="metric-meta">{{ $t("instanceMonitor.inputUnit") }}</span>
          </span>
          <span class="metric-io">
            <strong class="metric-value">{{ formatDecimal(latestRate.netTxMbps) }}</strong>
            <span class="metric-meta">{{ $t("instanceMonitor.outputUnit") }}</span>
          </span>
        </div>
        <div class="chart-panel">
          <div class="chart-legend">
            <span><i class="legend-mark rx-mark"></i>{{ $t("instanceMonitor.rx") }}</span>
            <span><i class="legend-mark tx-mark"></i>{{ $t("instanceMonitor.tx") }}</span>
          </div>
          <svg class="chart" viewBox="0 0 100 42" preserveAspectRatio="none" aria-hidden="true">
            <line class="chart-grid-line" x1="0" y1="13" x2="100" y2="13" />
            <line class="chart-grid-line" x1="0" y1="26" x2="100" y2="26" />
            <polyline class="chart-line rx-line" :points="netRxLine" />
            <polyline class="chart-line tx-line" :points="netTxLine" />
          </svg>
        </div>
      </article>

      <!-- Disk I/O -->
      <article class="metric-card">
        <div class="metric-row">
          <span class="metric-name">{{ $t("instanceMonitor.disk") }}</span>
          <span class="metric-io">
            <strong class="metric-value">{{ formatDecimal(latestRate.diskReadMbps) }}</strong>
            <span class="metric-meta">{{ $t("instanceMonitor.readUnit") }}</span>
          </span>
          <span class="metric-io">
            <strong class="metric-value">{{ formatDecimal(latestRate.diskWriteMbps) }}</strong>
            <span class="metric-meta">{{ $t("instanceMonitor.writeUnit") }}</span>
          </span>
        </div>
        <div class="chart-panel">
          <div class="chart-legend">
            <span><i class="legend-mark read-mark"></i>{{ $t("instanceMonitor.read") }}</span>
            <span><i class="legend-mark write-mark"></i>{{ $t("instanceMonitor.write") }}</span>
          </div>
          <svg class="chart" viewBox="0 0 100 42" preserveAspectRatio="none" aria-hidden="true">
            <line class="chart-grid-line" x1="0" y1="13" x2="100" y2="13" />
            <line class="chart-grid-line" x1="0" y1="26" x2="100" y2="26" />
            <polyline class="chart-line read-line" :points="diskReadLine" />
            <polyline class="chart-line write-line" :points="diskWriteLine" />
          </svg>
        </div>
      </article>
    </section>
  </section>
</template>

<style scoped>
.instance-monitor {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
}

.monitor-state {
  padding: 24px;
  color: var(--card-text);
  font-size: 14px;
  font-weight: bold;
  text-align: center;
}

.state-error {
  color: var(--danger);
}

.state-warn {
  color: var(--create-brass-secondary);
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

.metric-row {
  display: flex;
  align-items: center;
  gap: clamp(12px, 2vw, 24px);
  padding: 12px 20px;
}

.metric-name {
  color: var(--card-text);
  font-size: 17px;
  font-weight: 800;
  min-width: 56px;
}

.metric-value {
  color: var(--card-text);
  font-size: 16px;
  font-weight: 700;
  white-space: nowrap;
}

.metric-percent {
  display: flex;
  align-items: baseline;
  gap: 0;
}

.metric-unit-group {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.metric-io {
  display: flex;
  align-items: baseline;
  gap: 8px;
  white-space: nowrap;
}

.metric-meta {
  color: var(--text-muted);
  font-size: 12px;
  white-space: nowrap;
}

.chart-panel {
  padding: 0 16px 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.chart-legend {
  display: flex;
  align-items: center;
  gap: 16px;
  font-size: 12px;
  color: var(--card-text);
  justify-content: flex-end;
}

.legend-mark {
  display: inline-block;
  width: 12px;
  height: 3px;
  margin-right: 4px;
  vertical-align: middle;
  border-radius: 0;
}

.rx-mark {
  background: var(--success);
}

.tx-mark {
  background: var(--create-brass-secondary);
}

.read-mark {
  background: #76d6ff;
}

.write-mark {
  background: #ff8a65;
}

.chart {
  width: 100%;
  min-height: 140px;
  border: 3px solid var(--card-border);
  background:
    linear-gradient(90deg, rgba(252, 246, 189, 0.08) 1px, transparent 1px),
    linear-gradient(180deg, rgba(252, 246, 189, 0.07) 1px, transparent 1px), #17241f;
  background-size:
    12.5% 100%,
    100% 50%;
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

.cpu-line {
  stroke: #f5cb6e;
}

.memory-line {
  stroke: var(--success);
}

.rx-line {
  stroke: var(--success);
}

.tx-line {
  stroke: var(--create-brass-secondary);
}

.read-line {
  stroke: #76d6ff;
}

.write-line {
  stroke: #ff8a65;
}
</style>
