<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import {
  ApiRequestError,
  createInstanceDir,
  deleteInstanceFile,
  downloadInstanceFileUrl,
  type FileEntry,
  type FileMount,
  listInstanceFileMounts,
  listInstanceFiles,
  uploadInstanceFile,
} from "../api";

const props = defineProps<{
  containerId: string;
}>();

const { t } = useI18n();

const currentPath = ref<string>("/");
const mounts = ref<FileMount[]>([]);
const selectedMount = ref("");
const files = ref<FileEntry[]>([]);
const loading = ref(false);
const errorText = ref("");
const uploadInput = ref<HTMLInputElement | null>(null);

const activeMount = computed(() =>
  mounts.value.find((mount) => mount.name === selectedMount.value),
);
const canWrite = computed(() => Boolean(activeMount.value && !activeMount.value.readonly));

const breadcrumbs = computed(() => {
  const parts = currentPath.value.split("/").filter(Boolean);
  const crumbs = [{ name: t("files.root"), path: "/" }];
  let accumPath = "";
  for (const part of parts) {
    accumPath += `/${part}`;
    crumbs.push({ name: part, path: accumPath });
  }
  return crumbs;
});

onMounted(() => {
  void loadMounts();
});

function mapFileError(err: unknown): string {
  if (err instanceof ApiRequestError) {
    const backendMessage = err.backendMessage ?? "";
    if (backendMessage.includes("mount not found")) return t("files.errors.mountNotFound");
    if (backendMessage.includes("read-only")) return t("files.errors.readOnly");
    if (backendMessage.includes("invalid file path")) return t("files.errors.pathInvalid");
    if (backendMessage.includes("file not found")) return t("files.errors.fileNotFound");
    if (backendMessage.includes("upload file too large")) return t("files.errors.uploadTooLarge");
    return t("errors.requestFailedWithStatus", { status: err.status });
  }
  return t("errors.unknown");
}

async function loadMounts(): Promise<void> {
  loading.value = true;
  errorText.value = "";
  try {
    mounts.value = await listInstanceFileMounts(props.containerId);
    selectedMount.value = mounts.value[0]?.name ?? "";
    currentPath.value = "/";
    if (selectedMount.value) {
      await loadFiles("/");
    } else {
      files.value = [];
    }
  } catch (err) {
    files.value = [];
    errorText.value = mapFileError(err);
  } finally {
    loading.value = false;
  }
}

async function loadFiles(path: string): Promise<void> {
  if (!selectedMount.value) return;
  loading.value = true;
  errorText.value = "";
  try {
    files.value = await listInstanceFiles(props.containerId, selectedMount.value, path);
    currentPath.value = path;
  } catch (err) {
    errorText.value = mapFileError(err);
  } finally {
    loading.value = false;
  }
}

function formatSize(bytes?: number): string {
  if (bytes === undefined) return "-";
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
}

function formatDate(isoString?: string): string {
  if (!isoString) return "-";
  const date = new Date(isoString);
  return date.toLocaleString();
}

function childPath(name: string): string {
  return currentPath.value === "/" ? `/${name}` : `${currentPath.value}/${name}`;
}

function navigateTo(path: string): void {
  void loadFiles(path);
}

function handleFileClick(file: FileEntry): void {
  if (file.is_dir) {
    void loadFiles(childPath(file.name));
  }
}

function handleMountChange(): void {
  currentPath.value = "/";
  void loadFiles("/");
}

async function handleCreateDir(): Promise<void> {
  if (!selectedMount.value || !canWrite.value) return;
  const name = window.prompt(t("files.prompts.newFolder"));
  if (!name) return;
  try {
    await createInstanceDir(props.containerId, selectedMount.value, childPath(name));
    await loadFiles(currentPath.value);
  } catch (err) {
    errorText.value = mapFileError(err);
  }
}

function triggerUpload(): void {
  uploadInput.value?.click();
}

async function handleUpload(event: Event): Promise<void> {
  if (!selectedMount.value || !canWrite.value) return;
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (!file) return;
  try {
    await uploadInstanceFile(props.containerId, selectedMount.value, currentPath.value, file);
    await loadFiles(currentPath.value);
  } catch (err) {
    errorText.value = mapFileError(err);
  }
}

function handleDownload(file: FileEntry): void {
  if (file.is_dir || !selectedMount.value) return;
  window.open(
    downloadInstanceFileUrl(props.containerId, selectedMount.value, childPath(file.name)),
    "_blank",
  );
}

async function handleDelete(file: FileEntry): Promise<void> {
  if (!selectedMount.value || !canWrite.value) return;
  if (!window.confirm(t("files.prompts.delete", { name: file.name }))) return;
  try {
    await deleteInstanceFile(
      props.containerId,
      selectedMount.value,
      childPath(file.name),
      file.is_dir,
    );
    await loadFiles(currentPath.value);
  } catch (err) {
    errorText.value = mapFileError(err);
  }
}
</script>

<template>
  <div class="files-panel">
    <!-- Toolbar -->
    <div class="files-toolbar">
      <div class="breadcrumbs">
        <template v-for="(crumb, index) in breadcrumbs" :key="crumb.path">
          <span
            class="breadcrumb-item"
            :class="{ active: index === breadcrumbs.length - 1 }"
            @click="navigateTo(crumb.path)"
          >
            {{ crumb.name }}
          </span>
          <span v-if="index < breadcrumbs.length - 1" class="breadcrumb-separator">/</span>
        </template>
      </div>
      <div class="actions">
        <select
          v-if="mounts.length > 1"
          v-model="selectedMount"
          class="mount-select"
          @change="handleMountChange"
        >
          <option v-for="mount in mounts" :key="mount.name" :value="mount.name">
            {{ mount.name }}
          </option>
        </select>
        <button class="action-btn" :disabled="!canWrite" @click="handleCreateDir">
          {{ $t("files.actions.newFolder") }}
        </button>
        <button class="action-btn primary" :disabled="!canWrite" @click="triggerUpload">
          {{ $t("files.actions.upload") }}
        </button>
        <input ref="uploadInput" class="upload-input" type="file" @change="handleUpload" />
      </div>
    </div>
    <div v-if="errorText" class="error-state">{{ errorText }}</div>

    <!-- File List -->
    <div class="files-list-container">
      <table class="files-table">
        <thead>
          <tr>
            <th class="col-name">{{ $t("files.columns.name") }}</th>
            <th class="col-size">{{ $t("files.columns.size") }}</th>
            <th class="col-date">{{ $t("files.columns.modified") }}</th>
            <th class="col-actions">{{ $t("files.columns.actions") }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading">
            <td colspan="4" class="empty-state">{{ $t("files.loading") }}</td>
          </tr>
          <tr v-else-if="files.length === 0">
            <td colspan="4" class="empty-state">
              {{ selectedMount ? $t("files.empty") : $t("files.noMounts") }}
            </td>
          </tr>
          <tr
            v-for="file in files"
            :key="file.name"
            class="file-row"
            @dblclick="handleFileClick(file)"
          >
            <td class="col-name">
              <div class="file-name-cell">
                <span class="file-icon" :class="{ 'is-dir': file.is_dir }">
                  <svg
                    v-if="file.is_dir"
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    class="icon"
                  >
                    <path
                      d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"
                    ></path>
                  </svg>
                  <svg
                    v-else
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    class="icon"
                  >
                    <path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"></path>
                    <polyline points="13 2 13 9 20 9"></polyline>
                  </svg>
                </span>
                <span class="file-name" @click="handleFileClick(file)">{{ file.name }}</span>
              </div>
            </td>
            <td class="col-size">{{ formatSize(file.size) }}</td>
            <td class="col-date">{{ formatDate(file.modified_at) }}</td>
            <td class="col-actions">
              <div class="row-actions">
                <button
                  class="icon-btn"
                  :disabled="file.is_dir"
                  :title="$t('files.actions.download')"
                  @click.stop="handleDownload(file)"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    class="icon"
                  >
                    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
                    <polyline points="7 10 12 15 17 10"></polyline>
                    <line x1="12" y1="15" x2="12" y2="3"></line>
                  </svg>
                </button>
                <button
                  class="icon-btn danger"
                  :disabled="!canWrite"
                  :title="$t('files.actions.delete')"
                  @click.stop="handleDelete(file)"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    class="icon"
                  >
                    <polyline points="3 6 5 6 21 6"></polyline>
                    <path
                      d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"
                    ></path>
                    <line x1="10" y1="11" x2="10" y2="17"></line>
                    <line x1="14" y1="11" x2="14" y2="17"></line>
                  </svg>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.files-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  border: 2px solid var(--card-border-inner);
  border-radius: 0;
  background: #ffffff;
  overflow: hidden;
}

.files-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 2px solid var(--card-border-inner);
  background: var(--card-bg);
}

.breadcrumbs {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
}

.breadcrumb-item {
  color: var(--card-text);
  cursor: pointer;
  transition: color 0.2s;
}

.breadcrumb-item:hover {
  color: var(--create-border-outer);
}

.breadcrumb-item.active {
  color: var(--card-text);
  font-weight: 600;
  cursor: default;
}

.breadcrumb-separator {
  color: var(--text-muted);
}

.actions {
  display: flex;
  gap: 12px;
}

.mount-select {
  min-width: 140px;
  border: 2px solid var(--card-border);
  background: var(--card-bg);
  color: var(--card-text);
  padding: 6px 8px;
  border-radius: 0;
  font-size: 13px;
  font-weight: bold;
}

.action-btn {
  border: 2px solid var(--card-border);
  background: var(--card-bg);
  color: var(--card-text);
  padding: 6px 12px;
  border-radius: 0;
  cursor: pointer;
  font-size: 13px;
  font-weight: bold;
  box-shadow: 2px 2px 0 0 var(--create-border-outer);
  transition: all 0.2s;
}

.action-btn:hover {
  transform: translate(1px, 1px);
  box-shadow: 1px 1px 0 0 var(--create-border-outer);
  background: var(--create-brass-dark);
}

.action-btn:disabled,
.icon-btn:disabled {
  cursor: not-allowed;
  opacity: 0.45;
  transform: none;
  filter: none;
}

.action-btn.primary {
  background: var(--success);
  color: #fff;
}

.action-btn.primary:hover {
  background: var(--success);
  filter: brightness(1.1);
}

.upload-input {
  display: none;
}

.error-state {
  padding: 10px 16px;
  color: var(--danger);
  border-bottom: 2px solid var(--card-border-inner);
  background: var(--danger-light);
  font-size: 13px;
  font-weight: bold;
}

.files-list-container {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

.files-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
  table-layout: fixed;
}

.files-table th {
  position: sticky;
  top: 0;
  background: var(--card-bg);
  padding: 12px 16px;
  font-size: 12px;
  color: var(--text-muted);
  font-weight: bold;
  text-transform: uppercase;
  border-bottom: 2px solid var(--card-border-inner);
  z-index: 1;
}

.files-table td {
  padding: 10px 16px;
  border-bottom: 1px solid var(--card-border-inner);
  font-size: 14px;
  color: var(--card-text);
}

.file-row {
  transition: background-color 0.2s;
}

.file-row:hover {
  background: rgba(0, 0, 0, 0.04);
}

.col-name {
  width: 100%;
}

.col-size {
  width: 120px;
  text-align: right;
}

.col-date {
  width: 180px;
  text-align: right;
}

.col-actions {
  width: 100px;
  text-align: right;
}

.file-name-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.file-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  width: 20px;
  height: 20px;
}

.file-icon.is-dir {
  color: var(--create-border-outer);
}

.icon {
  width: 100%;
  height: 100%;
}

.file-name {
  cursor: pointer;
  user-select: none;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-name:hover {
  color: var(--create-border-outer);
  text-decoration: underline;
}

.row-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  opacity: 0;
  transition: opacity 0.2s;
}

.file-row:hover .row-actions {
  opacity: 1;
}

.icon-btn {
  background: transparent;
  border: 2px solid transparent;
  color: var(--text-muted);
  width: 28px;
  height: 28px;
  border-radius: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s;
  padding: 4px;
}

.icon-btn:hover {
  background: rgba(0, 0, 0, 0.08);
  color: var(--card-text);
}

.icon-btn.danger:hover {
  background: rgba(220, 53, 69, 0.1);
  color: var(--danger);
  border-color: rgba(220, 53, 69, 0.2);
}

.empty-state {
  text-align: center;
  padding: 40px !important;
  color: var(--text-muted);
  font-style: italic;
}
</style>
