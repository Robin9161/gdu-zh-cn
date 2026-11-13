package webui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func localScanDrives() ([]scanDrive, error) {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, err
	}
	drives := []scanDrive{}
	for i := 0; i < 26; i++ {
		if mask&(1<<i) != 0 {
			root := fmt.Sprintf("%c:\\", 'A'+i)
			if isLocalScanVolume(root) {
				drives = append(drives, scanDrive{Path: root})
			}
		}
	}
	return drives, nil
}

func isLocalScanVolume(root string) bool {
	name, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return false
	}
	kind := windows.GetDriveType(name)
	return kind == windows.DRIVE_FIXED || kind == windows.DRIVE_REMOVABLE
}

func localScanPath(raw string) (string, error) {
	path := strings.Trim(strings.TrimSpace(raw), "\"\"")
	if len(path) == 2 && path[1] == ':' {
		path += "\\"
	}
	if !filepath.IsAbs(path) || len(filepath.VolumeName(path)) != 2 {
		return "", errors.New("请输入本地绝对路径，例如 C:\\ 或 C:\\Users")
	}
	path = filepath.Clean(strings.ToUpper(path[:1]) + path[1:])
	if !isLocalScanVolume(filepath.VolumeName(path) + "\\") {
		return "", errors.New("请选择本机磁盘或可移动磁盘")
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil || len(filepath.VolumeName(realPath)) != 2 || !isLocalScanVolume(filepath.VolumeName(realPath)+"\\") {
		return "", errors.New("找不到文件夹，或无法读取其所在的本地磁盘")
	}
	dir, err := os.Open(realPath)
	if err != nil {
		return "", errors.New("文件夹无法访问")
	}
	defer dir.Close()
	info, err := dir.Stat()
	if err != nil || !info.IsDir() {
		return "", errors.New("请选择文件夹或盘符")
	}
	if _, err := dir.Readdirnames(1); err != nil && !errors.Is(err, io.EOF) {
		return "", errors.New("没有读取此文件夹的权限")
	}
	return realPath, nil
}
