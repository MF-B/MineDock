package model

import "time"

// FileEntry 描述实例挂载目录中的一个文件或目录。
type FileEntry struct {
	Name       string    `json:"name"`
	IsDir      bool      `json:"is_dir"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}

// FileMount 描述实例可管理的一个文件挂载点。
type FileMount struct {
	Name          string `json:"name"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"readonly"`
}
