//go:build !windows

package webui

import "errors"

func localScanDrives() ([]scanDrive, error) { return nil, errors.ErrUnsupported }
func localScanPath(string) (string, error)  { return "", errors.ErrUnsupported }
