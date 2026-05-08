package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"minedock/backend/internal/model"
)

// MaxUploadBytes 是单文件上传的默认大小限制。
const MaxUploadBytes int64 = 128 * 1024 * 1024

// FileService 提供实例 bind mount 文件管理能力。
type FileService struct {
	store    InstanceStore
	registry GameRegistry
	dataDir  string
}

// NewFileService 创建 FileService。
func NewFileService(store InstanceStore, registry GameRegistry, dataDir string) *FileService {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		dataDir = defaultDataDir
	}
	return &FileService{store: store, registry: registry, dataDir: dataDir}
}

type resolvedFileMount struct {
	mount model.FileMount
	root  string
}

// Mounts 返回实例可管理的文件挂载点列表。
func (s *FileService) Mounts(ctx context.Context, containerID string) ([]model.FileMount, error) {
	mounts, err := s.resolveMounts(ctx, containerID)
	if err != nil {
		return nil, err
	}
	out := make([]model.FileMount, 0, len(mounts))
	for _, mount := range mounts {
		out = append(out, mount.mount)
	}
	return out, nil
}

// List 列出指定挂载目录下的文件。
func (s *FileService) List(ctx context.Context, containerID, mountName, apiPath string) ([]model.FileEntry, error) {
	mount, err := s.resolveMount(ctx, containerID, mountName)
	if err != nil {
		return nil, err
	}
	target, err := resolveSafeExistingPath(mount.root, apiPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return nil, mapFileError(err)
	}
	if !info.IsDir() {
		return nil, model.ErrPathInvalid
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, mapFileError(err)
	}
	files := make([]model.FileEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, mapFileError(err)
		}
		size := info.Size()
		if entry.IsDir() {
			size, err = directorySize(ctx, filepath.Join(target, entry.Name()))
			if err != nil {
				return nil, err
			}
		}
		files = append(files, model.FileEntry{
			Name:       entry.Name(),
			IsDir:      entry.IsDir(),
			Size:       size,
			ModifiedAt: info.ModTime(),
		})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})
	return files, nil
}

func directorySize(ctx context.Context, dir string) (int64, error) {
	var size int64
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return mapFileError(walkErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return mapFileError(err)
		}
		size += info.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return size, nil
}

// CreateDir 在指定挂载点中新建目录。
func (s *FileService) CreateDir(ctx context.Context, containerID, mountName, apiPath string) error {
	mount, err := s.resolveWritableMount(ctx, containerID, mountName)
	if err != nil {
		return err
	}
	target, err := resolveSafeNewPath(mount.root, apiPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	return nil
}

// Delete 删除指定挂载点中的文件或目录。
func (s *FileService) Delete(ctx context.Context, containerID, mountName, apiPath string, recursive bool) error {
	mount, err := s.resolveWritableMount(ctx, containerID, mountName)
	if err != nil {
		return err
	}
	target, err := resolveSafeExistingPath(mount.root, apiPath)
	if err != nil {
		return err
	}
	if target == mount.root {
		return model.ErrPathInvalid
	}

	info, err := os.Stat(target)
	if err != nil {
		return mapFileError(err)
	}
	if info.IsDir() && recursive {
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("delete dir: %w", err)
		}
		return nil
	}
	if err := os.Remove(target); err != nil {
		return mapFileError(err)
	}
	return nil
}

// OpenDownload 打开指定挂载点中的文件用于下载。
func (s *FileService) OpenDownload(ctx context.Context, containerID, mountName, apiPath string) (*os.File, os.FileInfo, error) {
	mount, err := s.resolveMount(ctx, containerID, mountName)
	if err != nil {
		return nil, nil, err
	}
	target, err := resolveSafeExistingPath(mount.root, apiPath)
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return nil, nil, mapFileError(err)
	}
	if info.IsDir() {
		return nil, nil, model.ErrPathInvalid
	}
	f, err := os.Open(target)
	if err != nil {
		return nil, nil, mapFileError(err)
	}
	return f, info, nil
}

// SaveUpload 保存上传文件到指定挂载点路径。
func (s *FileService) SaveUpload(ctx context.Context, containerID, mountName, apiPath string, reader io.Reader) error {
	mount, err := s.resolveWritableMount(ctx, containerID, mountName)
	if err != nil {
		return err
	}
	target, err := resolveSafeNewPath(mount.root, apiPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create upload dir: %w", err)
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open upload file: %w", err)
	}
	defer f.Close()

	limited := io.LimitReader(reader, MaxUploadBytes+1)
	written, err := io.Copy(f, limited)
	if err != nil {
		return fmt.Errorf("save upload: %w", err)
	}
	if written > MaxUploadBytes {
		_ = f.Close()
		_ = os.Remove(target)
		return model.ErrUploadTooLarge
	}
	return nil
}

func (s *FileService) resolveWritableMount(ctx context.Context, containerID, mountName string) (resolvedFileMount, error) {
	mount, err := s.resolveMount(ctx, containerID, mountName)
	if err != nil {
		return resolvedFileMount{}, err
	}
	if mount.mount.ReadOnly {
		return resolvedFileMount{}, model.ErrReadOnlyMount
	}
	return mount, nil
}

func (s *FileService) resolveMount(ctx context.Context, containerID, mountName string) (resolvedFileMount, error) {
	mounts, err := s.resolveMounts(ctx, containerID)
	if err != nil {
		return resolvedFileMount{}, err
	}
	mountName = strings.TrimSpace(mountName)
	if mountName == "" {
		return resolvedFileMount{}, model.ErrMountNotFound
	}
	for _, mount := range mounts {
		if mount.mount.Name == mountName {
			return mount, nil
		}
	}
	return resolvedFileMount{}, model.ErrMountNotFound
}

func (s *FileService) resolveMounts(ctx context.Context, containerID string) ([]resolvedFileMount, error) {
	inst, ok, err := s.store.Get(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("read instance: %w", err)
	}
	if !ok || strings.TrimSpace(inst.GameID) == "" || strings.TrimSpace(inst.Name) == "" {
		return nil, model.ErrMountNotFound
	}

	tpl, err := s.registry.GetTemplate(ctx, inst.GameID)
	if err != nil {
		return nil, err
	}
	mounts := make([]resolvedFileMount, 0, len(tpl.Container.Volumes))
	for _, volume := range tpl.Container.Volumes {
		root, err := safeVolumeDataDir(s.dataDir, inst.Name, volume.Name)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(root, 0o755); err != nil {
			return nil, fmt.Errorf("create mount root: %w", err)
		}
		mounts = append(mounts, resolvedFileMount{
			mount: model.FileMount(volume),
			root:  root,
		})
	}
	if len(mounts) == 0 {
		return nil, model.ErrMountNotFound
	}
	return mounts, nil
}

func resolveSafeExistingPath(root, apiPath string) (string, error) {
	target, err := resolveSafePath(root, apiPath)
	if err != nil {
		return "", err
	}
	if err := rejectSymlinkPath(root, target); err != nil {
		return "", err
	}
	return target, nil
}

func resolveSafeNewPath(root, apiPath string) (string, error) {
	target, err := resolveSafePath(root, apiPath)
	if err != nil {
		return "", err
	}
	if target == root {
		return "", model.ErrPathInvalid
	}
	if err := rejectSymlinkPath(root, target); err != nil {
		return "", err
	}
	return target, nil
}

func resolveSafePath(root, apiPath string) (string, error) {
	clean, err := cleanAPIPath(apiPath)
	if err != nil {
		return "", err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve mount root: %w", err)
	}
	targetAbs, err := filepath.Abs(filepath.Join(rootAbs, filepath.FromSlash(clean)))
	if err != nil {
		return "", fmt.Errorf("resolve file path: %w", err)
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", fmt.Errorf("validate file path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", model.ErrPathInvalid
	}
	return targetAbs, nil
}

func cleanAPIPath(apiPath string) (string, error) {
	apiPath = strings.TrimSpace(apiPath)
	if apiPath == "" || apiPath == "/" {
		return ".", nil
	}
	if strings.Contains(apiPath, "\\") {
		return "", model.ErrPathInvalid
	}
	for _, part := range strings.Split(apiPath, "/") {
		if part == ".." {
			return "", model.ErrPathInvalid
		}
	}
	clean := path.Clean("/" + apiPath)
	parts := strings.Split(strings.Trim(clean, "/"), "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", model.ErrPathInvalid
		}
	}
	return strings.TrimPrefix(clean, "/"), nil
}

func rejectSymlinkPath(root, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return fmt.Errorf("validate symlink path: %w", err)
	}
	if rel == "." {
		return nil
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return mapFileError(err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return model.ErrPathInvalid
		}
	}
	return nil
}

func mapFileError(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return model.ErrFileNotFound
	}
	return err
}

// MaxEditBytes 是可在线编辑的单文件大小限制（1 MB）。
const MaxEditBytes int64 = 1 * 1024 * 1024

// ReadContent 读取指定文件的文本内容，拒绝二进制和超大文件。
func (s *FileService) ReadContent(ctx context.Context, containerID, mountName, apiPath string) (string, int64, error) {
	mount, err := s.resolveMount(ctx, containerID, mountName)
	if err != nil {
		return "", 0, err
	}
	target, err := resolveSafeExistingPath(mount.root, apiPath)
	if err != nil {
		return "", 0, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", 0, mapFileError(err)
	}
	if info.IsDir() {
		return "", 0, model.ErrPathInvalid
	}
	if info.Size() > MaxEditBytes {
		return "", info.Size(), model.ErrFileTooLarge
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return "", 0, mapFileError(err)
	}
	if isBinaryContent(data) {
		return "", info.Size(), model.ErrFileBinary
	}
	return string(data), info.Size(), nil
}

// WriteContent 将文本内容写入指定文件（覆盖）。
func (s *FileService) WriteContent(ctx context.Context, containerID, mountName, apiPath, content string) error {
	mount, err := s.resolveWritableMount(ctx, containerID, mountName)
	if err != nil {
		return err
	}
	target, err := resolveSafeNewPath(mount.root, apiPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}
	return os.WriteFile(target, []byte(content), 0o644)
}

// isBinaryContent 通过检查 NUL 字节判断内容是否为二进制。
func isBinaryContent(data []byte) bool {
	// 检查前 8 KB 即可覆盖绝大多数情况。
	check := data
	if len(check) > 8192 {
		check = check[:8192]
	}
	for _, b := range check {
		if b == 0 {
			return true
		}
	}
	return false
}
