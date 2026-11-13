package webui

import (
	"path/filepath"
	"strings"

	"github.com/dundee/gdu/v5/pkg/cleanup"
	"github.com/dundee/gdu/v5/pkg/fs"
)

type cleanupGrade string

const (
	gradeA  cleanupGrade = "A"
	gradeB  cleanupGrade = "B"
	gradeC  cleanupGrade = "C"
	gradeD  cleanupGrade = "D"
	gradeAB cleanupGrade = "AB"
)

func (grade cleanupGrade) includes(fileGrade cleanupGrade) bool {
	return grade == "" || grade == fileGrade || (grade == gradeAB && (fileGrade == gradeA || fileGrade == gradeB))
}

type fileSafety struct {
	Grade  cleanupGrade `json:"grade"`
	Reason string       `json:"reason"`
}

func pathParts(path string) []string {
	path = filepath.Clean(path)
	return strings.FieldsFunc(strings.ToLower(strings.TrimPrefix(path, filepath.VolumeName(path))), func(r rune) bool {
		return r == '/' || r == '\\'
	})
}

func protectedReason(parts []string, category string) string {
	if len(parts) == 0 {
		return "无法确认文件路径"
	}
	switch parts[0] {
	case "windows", "program files", "program files (x86)", "programdata", "windowsapps", "$recycle.bin", "system volume information", "recovery":
		return "位于 Windows 系统、程序或回收站目录"
	}

	for _, part := range parts[:len(parts)-1] {
		switch part {
		case "appdata", ".config", ".codex", ".claude", ".git":
			return "位于应用、账号或版本库数据目录"
		case ".ssh", ".gnupg", ".aws", ".azure", ".kube", "credentials":
			return "位于凭据或密钥目录"
		}
	}

	name := parts[len(parts)-1]
	switch name {
	case ".env", ".netrc", ".npmrc", "credentials", "credentials.json", "auth.json", "cookies", "login data",
		"pagefile.sys", "hiberfil.sys", "swapfile.sys", "ntuser.dat", "desktop.ini":
		return "文件名对应凭据、账号数据或系统文件"
	}
	if strings.HasPrefix(name, ".env.") {
		return "可能含有环境变量中的密钥或账号配置"
	}
	switch filepath.Ext(name) {
	case ".pem", ".key", ".pfx", ".p12", ".ppk", ".kdbx":
		return "可能是密钥、证书或密码库"
	}
	if category == "database" {
		return "数据库及其配套文件可能保存应用或项目状态"
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if base, ok := strings.CutSuffix(name, suffix); ok && fileCategory(base) == "database" {
			return "数据库及其配套文件可能保存应用或项目状态"
		}
	}
	return ""
}

func assessCleanupSafety(item fs.Item, candidates *cleanup.Result) fileSafety {
	return assessCleanupPath(item, item.GetPath(), candidates)
}

func assessCleanupPath(item fs.Item, path string, candidates *cleanup.Result) fileSafety {
	category := fileCategory(item.GetName())
	parts := pathParts(path)
	if reason := protectedReason(parts, category); reason != "" {
		return fileSafety{gradeD, reason}
	}
	if item.GetFlag() == '@' || item.GetFlag() == '!' || item.IsDir() {
		return fileSafety{gradeD, "链接、无法读取的文件或虚拟条目不支持清理"}
	}
	// The upstream classifier supplies project markers and exclusions.
	// Keep this promotion limited to known dependency/cache paths, because
	// a generic build or app-cache match can contain user data.
	if candidates != nil {
		if match, ok := candidates.Match(item); ok && match.Confidence == cleanup.Safe {
			name := parts[len(parts)-1]
			if match.Category == cleanup.BuildArtifact && len(parts) >= 2 && parts[len(parts)-2] == "__pycache__" &&
				strings.HasSuffix(name, ".pyc") && (strings.Contains(name, ".cpython-") || strings.Contains(name, ".pypy-")) {
				return fileSafety{gradeA, "Python 生成的字节码缓存，可在运行时重新生成"}
			}
			for _, part := range parts[:len(parts)-1] {
				if part == "node_modules" && match.Category == cleanup.BuildArtifact {
					return fileSafety{gradeB, "识别为项目依赖，可重新安装；先确认项目未在使用"}
				}
			}
			if match.Category == cleanup.PackageCache {
				return fileSafety{gradeB, "识别为软件包缓存；先退出相关工具并确认可以重新下载"}
			}
		}
	}

	switch category {
	case "installer":
		return fileSafety{gradeC, "安装包或镜像的来源和可重新下载性未确认"}
	case "archive":
		return fileSafety{gradeC, "压缩包可能是唯一备份，请先核对内容和其他副本"}
	case "video", "audio", "image", "project":
		return fileSafety{gradeC, "可能是源素材或工程文件，请先核对引用和备份"}
	case "model":
		return fileSafety{gradeC, "模型可能正在使用，或需要通过模型管理工具移除"}
	default:
		return fileSafety{gradeC, "无法确认用途或是否可恢复，请逐项核对"}
	}
}
