//go:build windows

package webui

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"

	"github.com/dundee/gdu/v5/pkg/remove"
)

const windowsRecycleSupported = true

func recycleFile(path string) error { return remove.RecycleWindowsFile(path) }

func validateRecycleFile(path string, info os.FileInfo) error {
	attrs, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok || !info.Mode().IsRegular() || attrs.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return errors.New("仅支持普通本地文件")
	}
	drive, err := windows.UTF16PtrFromString(filepath.VolumeName(path) + "\\")
	if err != nil || windows.GetDriveType(drive) != windows.DRIVE_FIXED {
		return errors.New("仅支持有 Windows 回收站的本地固定磁盘")
	}
	return nil
}
