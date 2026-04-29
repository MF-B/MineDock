//go:build !windows

package service

func hostDiskNamesByMountpoint() map[string]string {
	return map[string]string{}
}

func hostMemoryModel(_ uint64) string {
	return ""
}
