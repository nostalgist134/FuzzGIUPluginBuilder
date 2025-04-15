package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// IsFile 检查路径是否是文件，如果路径访问出错则返回错误信息
func IsFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err // 返回错误信息，便于调试
	}
	return !info.IsDir(), nil
}

// GetFileDir 返回文件所在的目录；如果路径本身是目录，则直接返回该路径
func GetFileDir(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("无法访问 %s: %w", path, err)
	}
	if info.IsDir() {
		return path, nil // 如果是目录，直接返回路径
	}
	return filepath.Dir(path), nil // 否则返回文件所在目录
}

// 复制文件到目标目录
func copyFileToDir(srcFile, destDir string) error {
	// 确保目标目录存在
	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// 获取源文件名
	filename := filepath.Base(srcFile)
	destFile := filepath.Join(destDir, filename)

	// 打开源文件
	src, err := os.Open(srcFile)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	// 创建目标文件
	dst, err := os.Create(destFile)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// 复制文件内容
	_, err = io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// 同步文件
	err = dst.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	return nil
}
