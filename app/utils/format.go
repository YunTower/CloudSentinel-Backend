package utils

import "fmt"

// FormatStorageSize 格式化存储容量大小
func FormatStorageSize(bytes int64) string {
	if bytes == 0 {
		return ""
	}
	if bytes >= 1024*1024*1024*1024 {
		// TB
		return fmt.Sprintf("%.1fTB", float64(bytes)/(1024*1024*1024*1024))
	}
	if bytes >= 1024*1024*1024 {
		// GB
		return fmt.Sprintf("%.1fGB", float64(bytes)/(1024*1024*1024))
	}
	if bytes >= 1024*1024 {
		// MB
		return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
	}
	// KB
	return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
}

