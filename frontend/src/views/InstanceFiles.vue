<script setup lang="ts">
import { ref, computed } from "vue";
import { useI18n } from "vue-i18n";

const props = defineProps<{
  containerId: string;
}>();

const { t } = useI18n();

// Mock data interfaces
interface FileItem {
  name: string;
  isDir: boolean;
  size?: number; // Size in bytes
  modifiedAt?: string; // ISO date string
}

// Mock state
const currentPath = ref<string>("/");
const files = ref<FileItem[]>([
  { name: "mods", isDir: true, modifiedAt: "2026-04-20T10:00:00Z" },
  { name: "world", isDir: true, modifiedAt: "2026-04-25T15:30:00Z" },
  { name: "server.properties", isDir: false, size: 1024, modifiedAt: "2026-04-26T09:12:00Z" },
  { name: "eula.txt", isDir: false, size: 45, modifiedAt: "2026-04-01T12:00:00Z" },
]);

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

function navigateTo(path: string) {
  // Mock navigation
  currentPath.value = path;
  if (path === "/") {
    files.value = [
      { name: "mods", isDir: true, modifiedAt: "2026-04-20T10:00:00Z" },
      { name: "world", isDir: true, modifiedAt: "2026-04-25T15:30:00Z" },
      { name: "server.properties", isDir: false, size: 1024, modifiedAt: "2026-04-26T09:12:00Z" },
      { name: "eula.txt", isDir: false, size: 45, modifiedAt: "2026-04-01T12:00:00Z" },
    ];
  } else if (path === "/mods") {
    files.value = [
      {
        name: "OptiFine_1.20.1.jar",
        isDir: false,
        size: 5432100,
        modifiedAt: "2026-04-20T10:05:00Z",
      },
      { name: "jei-1.20.1.jar", isDir: false, size: 1234500, modifiedAt: "2026-04-20T10:06:00Z" },
    ];
  } else {
    files.value = [];
  }
}

function handleFileClick(file: FileItem) {
  if (file.isDir) {
    const newPath = currentPath.value.endsWith("/")
      ? `${currentPath.value}${file.name}`
      : `${currentPath.value}/${file.name}`;
    navigateTo(newPath);
  }
}

function handleAction(action: string, file: FileItem) {
  // Mock action
  console.log(`Action ${action} on file ${file.name} in container ${props.containerId}`);
  alert(`Mock Action: ${action} -> ${file.name}`);
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
        <button class="action-btn">{{ $t("files.actions.newFolder") }}</button>
        <button class="action-btn primary">{{ $t("files.actions.upload") }}</button>
      </div>
    </div>

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
          <tr v-if="files.length === 0">
            <td colspan="4" class="empty-state">{{ $t("files.empty") }}</td>
          </tr>
          <tr
            v-for="file in files"
            :key="file.name"
            class="file-row"
            @dblclick="handleFileClick(file)"
          >
            <td class="col-name">
              <div class="file-name-cell">
                <span class="file-icon" :class="{ 'is-dir': file.isDir }">
                  <svg
                    v-if="file.isDir"
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
            <td class="col-date">{{ formatDate(file.modifiedAt) }}</td>
            <td class="col-actions">
              <div class="row-actions">
                <button
                  class="icon-btn"
                  :title="$t('files.actions.download')"
                  @click.stop="handleAction('download', file)"
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
                  :title="$t('files.actions.delete')"
                  @click.stop="handleAction('delete', file)"
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
  border: 1px solid var(--card-border-inner);
  border-radius: 6px;
  background: #ffffff;
  overflow: hidden;
}

.files-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid var(--card-border-inner);
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

.action-btn {
  border: 1px solid var(--card-border-inner);
  background: #ffffff;
  color: var(--card-text);
  padding: 6px 12px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
  transition: all 0.2s;
}

.action-btn:hover {
  background: #f0f0f0;
  color: var(--card-text);
  border-color: var(--card-border);
}

.action-btn.primary {
  background: var(--create-brass-secondary);
  color: var(--card-text);
  border-color: var(--create-border-dark);
  font-weight: 500;
}

.action-btn.primary:hover {
  background: var(--create-brass-primary);
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
  font-weight: 600;
  text-transform: uppercase;
  border-bottom: 1px solid var(--card-border-inner);
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
  border: 1px solid transparent;
  color: var(--text-muted);
  width: 28px;
  height: 28px;
  border-radius: 4px;
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
