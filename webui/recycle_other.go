//go:build !windows

package webui

import (
	"errors"
	"os"
)

const windowsRecycleSupported = false

func recycleFile(string) error                      { return errors.New("仅支持 Windows 回收站") }
func validateRecycleFile(string, os.FileInfo) error { return errors.New("仅支持 Windows 回收站") }
