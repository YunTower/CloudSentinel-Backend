package services

// 更新流程中的文件级工具：压缩包安全解压、校验和计算/读取、
// 文件复制（Linux 原子替换）、临时文件清理。
// 从 update_service.go 拆出，便于独立测试与复用。

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"goravel/app/facades"
)

type downloadProgressWriter struct {
	writer       io.Writer
	total        int64
	written      int64
	lastProgress int
	callback     func(int)
}

func (w *downloadProgressWriter) Write(data []byte) (int, error) {
	n, err := w.writer.Write(data)
	w.written += int64(n)
	if w.total > 0 {
		progress := min(int(w.written*100/w.total), 100)
		if progress > w.lastProgress {
			w.lastProgress = progress
			w.callback(progress)
		}
	}
	return n, err
}

// hasPathPrefix 检查路径是否具有前缀
func hasPathPrefix(path, prefix string) bool {
	normalizedPath := filepath.Clean(path)
	normalizedPrefix := filepath.Clean(prefix)

	if runtime.GOOS == "windows" {
		normalizedPath = strings.ToLower(normalizedPath)
		normalizedPrefix = strings.ToLower(normalizedPrefix)
	}

	if normalizedPath == normalizedPrefix {
		return true
	}
	return strings.HasPrefix(normalizedPath, normalizedPrefix+string(os.PathSeparator))
}

// secureArchiveTargetPath 确保归档路径安全
func secureArchiveTargetPath(destDir, archivePath string) (string, error) {
	if archivePath == "" {
		return "", fmt.Errorf("归档路径为空")
	}

	cleanName := filepath.Clean(archivePath)
	if filepath.IsAbs(cleanName) || cleanName == "." || cleanName == "" {
		return "", fmt.Errorf("非法归档路径")
	}
	if cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("归档路径试图逃逸目标目录")
	}

	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return "", fmt.Errorf("解析目标目录失败: %w", err)
	}
	targetAbs, err := filepath.Abs(filepath.Join(destDir, cleanName))
	if err != nil {
		return "", fmt.Errorf("解析目标路径失败: %w", err)
	}
	if !hasPathPrefix(targetAbs, destAbs) {
		return "", fmt.Errorf("归档路径试图逃逸目标目录")
	}

	return targetAbs, nil
}

// ExtractTarGz 解压 tar.gz 文件
func (s *UpdateService) ExtractTarGz(tarGzPath, destDir string) error {
	// 打开 tar.gz 文件
	file, err := os.Open(tarGzPath)
	if err != nil {
		return fmt.Errorf("打开压缩文件失败: %v", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			facades.Log().Warningf("关闭压缩文件失败: %v", closeErr)
		}
	}()

	// 创建 gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("创建 gzip reader 失败: %v", err)
	}
	defer func() {
		if closeErr := gzReader.Close(); closeErr != nil {
			facades.Log().Warningf("关闭 gzip reader 失败: %v", closeErr)
		}
	}()

	// 创建 tar reader
	tarReader := tar.NewReader(gzReader)

	// 解压所有文件
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取 tar 文件失败: %v", err)
		}

		targetPath, err := secureArchiveTargetPath(destDir, header.Name)
		if err != nil {
			return fmt.Errorf("非法归档条目 %q: %w", header.Name, err)
		}

		// 处理目录
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("创建目录失败: %v", err)
			}
			continue
		}

		// 处理文件
		if header.Typeflag == tar.TypeReg {
			// 确保目录存在
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("创建目录失败: %v", err)
			}

			// 创建文件
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("创建文件失败: %v", err)
			}

			// 复制文件内容
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("复制文件内容失败: %v", err)
			}

			// 立即关闭文件
			if err := outFile.Close(); err != nil {
				return fmt.Errorf("关闭解压文件失败: %v", err)
			}

			// 设置文件权限
			if err := os.Chmod(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("设置文件权限失败: %v", err)
			}
			continue
		}

		if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
			return fmt.Errorf("不支持的归档条目类型: %q", header.Name)
		}
	}

	return nil
}

// CalculateSHA256 计算文件的 SHA256 值
func (s *UpdateService) CalculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %v", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			facades.Log().Warningf("关闭文件失败: %v", closeErr)
		}
	}()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("计算哈希失败: %v", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CopyFile 复制文件
func (s *UpdateService) CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := sourceFile.Close(); closeErr != nil {
			facades.Log().Warningf("关闭源文件失败: %v", closeErr)
		}
	}()

	// 在 Linux 系统上，如果目标文件存在且可能正在运行，使用原子替换
	if runtime.GOOS != "windows" {
		// 检查目标文件是否存在
		if _, err := os.Stat(dst); err == nil {
			// 目标文件存在，使用原子替换方式
			tempDst := dst + ".new"
			destFile, err := os.Create(tempDst)
			if err != nil {
				return fmt.Errorf("创建临时文件失败: %v", err)
			}

			// 复制文件内容
			if _, err := io.Copy(destFile, sourceFile); err != nil {
				destFile.Close()
				os.Remove(tempDst)
				return fmt.Errorf("复制文件内容失败: %v", err)
			}

			// 确保数据写入磁盘
			if err := destFile.Sync(); err != nil {
				destFile.Close()
				os.Remove(tempDst)
				return fmt.Errorf("同步文件失败: %v", err)
			}

			// 关闭文件
			if err := destFile.Close(); err != nil {
				os.Remove(tempDst)
				return fmt.Errorf("关闭临时文件失败: %v", err)
			}

			// 设置可执行权限（如果需要）
			if err := os.Chmod(tempDst, 0755); err != nil {
				facades.Log().Warningf("设置临时文件权限失败: %v", err)
			}

			// 使用原子替换
			if err := os.Rename(tempDst, dst); err != nil {
				os.Remove(tempDst)
				return fmt.Errorf("原子替换文件失败: %v", err)
			}

			return nil
		}
	}

	// Windows 系统或目标文件不存在，使用常规方式
	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %v", err)
	}
	defer func() {
		if closeErr := destFile.Close(); closeErr != nil {
			facades.Log().Warningf("关闭目标文件失败: %v", closeErr)
		}
	}()

	// 复制文件内容
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("复制文件内容失败: %v", err)
	}

	// 确保数据写入磁盘
	if err := destFile.Sync(); err != nil {
		return fmt.Errorf("同步文件失败: %v", err)
	}

	return nil
}

// ReadSHA256File 读取 SHA256 文件内容
func (s *UpdateService) ReadSHA256File(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("读取 SHA256 文件失败: %v", err)
	}

	// SHA256 文件格式通常是 "hash  filename" 或只有 "hash"
	content := strings.TrimSpace(string(data))
	parts := strings.Fields(content)
	if len(parts) > 0 {
		return parts[0], nil
	}

	return "", fmt.Errorf("SHA256 文件格式错误")
}

// CleanupTempFiles 清理更新过程中的临时文件
func (s *UpdateService) CleanupTempFiles(files ...string) {
	// 如果提供了具体的文件路径，优先清理这些文件
	if len(files) > 0 {
		for _, file := range files {
			if file == "" {
				continue
			}
			if err := os.RemoveAll(file); err != nil {
				facades.Log().Warningf("删除临时文件失败 (%s): %v", file, err)
			} else {
				facades.Log().Infof("已清理临时文件: %s", file)
			}
		}
	}

	// 保留原有的模式匹配清理逻辑作为后备
	tempDir := os.TempDir()
	patterns := []string{
		"dashboard-*.tar.gz",
		"dashboard-*.sha256",
		"dashboard-*.sha256.asc",
		"update_extract",
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(tempDir, pattern))
		if err != nil {
			facades.Log().Warningf("清理临时文件失败 (pattern: %s): %v", pattern, err)
			continue
		}

		for _, match := range matches {
			// 跳过已经在上面清理过的文件
			skip := false
			for _, file := range files {
				if match == file {
					skip = true
					break
				}
			}
			if skip {
				continue
			}

			if err := os.RemoveAll(match); err != nil {
				facades.Log().Warningf("删除临时文件失败 (%s): %v", match, err)
			} else {
				facades.Log().Infof("已清理临时文件: %s", match)
			}
		}
	}
}
