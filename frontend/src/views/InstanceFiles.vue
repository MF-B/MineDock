<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from "vue";
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
  readInstanceFileContent,
  uploadInstanceFile,
  writeInstanceFileContent,
} from "../api";
import { useCodeEditor } from "../composables/useCodeEditor";

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

// --- Editor state ---
const editorMode = ref(false);
const editorFileName = ref("");
const editorFilePath = ref("");
const editorLoading = ref(false);
const editorSaving = ref(false);
const editorError = ref("");
const editorFileSize = ref(0);
const editorOriginalContent = ref("");

const { editorRef, create: createEditor, getContent, destroy: destroyEditor } = useCodeEditor();

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

const editorDirty = computed(() => {
  if (!editorMode.value) return false;
  return getContent() !== editorOriginalContent.value;
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
    if (backendMessage.includes("file too large to edit")) return t("files.editor.fileTooLarge");
    if (backendMessage.includes("binary file")) return t("files.editor.fileBinary");
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

// --- Editor functions ---
async function openEditor(file: FileEntry): Promise<void> {
  if (file.is_dir || !selectedMount.value) return;
  const filePath = childPath(file.name);
  editorMode.value = true;
  editorFileName.value = file.name;
  editorFilePath.value = filePath;
  editorLoading.value = true;
  editorError.value = "";
  editorFileSize.value = 0;

  try {
    const resp = await readInstanceFileContent(props.containerId, selectedMount.value, filePath);
    editorFileSize.value = resp.size;
    editorOriginalContent.value = resp.content;
    await nextTick();
    createEditor(resp.content, file.name, !canWrite.value);
  } catch (err) {
    editorError.value = mapFileError(err);
  } finally {
    editorLoading.value = false;
  }
}

function closeEditor(): void {
  if (editorDirty.value && !window.confirm(t("files.editor.unsavedChanges"))) return;
  destroyEditor();
  editorMode.value = false;
  editorError.value = "";
}

async function saveEditor(): Promise<void> {
  if (!selectedMount.value || !canWrite.value || editorSaving.value) return;
  editorSaving.value = true;
  editorError.value = "";
  try {
    const content = getContent();
    await writeInstanceFileContent(
      props.containerId,
      selectedMount.value,
      editorFilePath.value,
      content,
    );
    editorOriginalContent.value = content;
  } catch (err) {
    editorError.value = mapFileError(err);
  } finally {
    editorSaving.value = false;
  }
}
</script>

<template>
  <div class="files-panel">
    <!-- Editor Mode -->
    <template v-if="editorMode">
      <div class="editor-toolbar">
        <div class="editor-title">
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
            <path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"></path>
            <polyline points="13 2 13 9 20 9"></polyline>
          </svg>
          <span class="editor-filename">{{ editorFileName }}</span>
          <span v-if="!canWrite" class="readonly-badge">{{ $t("files.editor.readOnly") }}</span>
        </div>
        <div class="editor-actions">
          <span v-if="editorFileSize" class="editor-size">{{ formatSize(editorFileSize) }}</span>
          <button
            v-if="canWrite"
            class="action-btn primary"
            :disabled="editorSaving || editorLoading"
            @click="saveEditor"
          >
            {{ editorSaving ? $t("files.editor.saving") : $t("files.editor.save") }}
          </button>
          <button class="action-btn" @click="closeEditor">
            {{ $t("files.editor.cancel") }}
          </button>
        </div>
      </div>
      <div v-if="editorError" class="error-state">{{ editorError }}</div>
      <div v-if="editorLoading" class="editor-loading">{{ $t("files.editor.loading") }}</div>
      <div v-show="!editorLoading && !editorError" ref="editorRef" class="editor-host"></div>
    </template>

    <!-- File List Mode -->
    <template v-else>
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
                    v-if="!file.is_dir"
                    class="icon-btn"
                    :title="$t('files.editor.edit')"
                    @click.stop="openEditor(file)"
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
                      <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                      <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                    </svg>
                  </button>
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
    </template>
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

/* --- Editor styles --- */
.editor-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 16px;
  border-bottom: 2px solid var(--card-border-inner);
  background: var(--card-bg);
  gap: 12px;
}

.editor-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--card-text);
  min-width: 0;
}

.editor-title .icon {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
  color: var(--create-border-outer);
}

.editor-filename {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.readonly-badge {
  font-size: 11px;
  font-weight: bold;
  text-transform: uppercase;
  padding: 2px 8px;
  border: 2px solid var(--card-border);
  background: var(--create-brass-dark);
  color: var(--text-muted);
  flex-shrink: 0;
}

.editor-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.editor-size {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: bold;
}

.editor-loading {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-style: italic;
  padding: 40px;
}

.editor-host {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

/* Override CodeMirror to fill container */
.editor-host :deep(.cm-editor) {
  height: 100%;
}

.editor-host :deep(.cm-scroller) {
  overflow: auto;
}

/* --- File list styles (unchanged) --- */
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
  width: 130px;
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
