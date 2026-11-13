package webui

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/cleanup"
	"github.com/dundee/gdu/v5/pkg/fs"
)

type fileOwner struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Evidence string `json:"evidence"`
	Impact   string `json:"impact"`
	Recovery string `json:"recovery"`
	impact   ownerImpact
}

type ownerGroup struct {
	fileOwner
	Bytes          int64 `json:"bytes"`
	Count          int   `json:"count"`
	ABytes         int64 `json:"aBytes"`
	BBytes         int64 `json:"bBytes"`
	protectedCount int
}

func (group *ownerGroup) add(file overviewFile) {
	if group.Count == 0 || file.Owner.impact > group.impact {
		group.fileOwner = *file.Owner
	}
	group.Bytes += file.Usage
	group.Count++
	switch file.Grade {
	case gradeA:
		group.ABytes += file.Usage
	case gradeB:
		group.BBytes += file.Usage
	case gradeD:
		group.protectedCount++
	}
}

func (group ownerGroup) summary() ownerGroup {
	if group.protectedCount > 0 && group.protectedCount < group.Count {
		group.Impact = "包含受保护文件，请逐项查看分级。"
		group.Recovery = "按文件核对恢复方式。受保护文件请通过系统或相关软件管理。"
	}
	return group
}

func compareOwnerGroups(a, b ownerGroup) int {
	if order := cmp.Compare(b.Bytes, a.Bytes); order != 0 {
		return order
	}
	return strings.Compare(a.Path, b.Path)
}

type ownerImpact int

const (
	impactCache ownerImpact = iota
	impactPackageCache
	impactDependency
	impactToolDependency
	impactUnknown
	impactProtected
)

var ownerEffects = [...]struct{ impact, recovery string }{
	{"Python 首次重新导入模块时可能稍慢。", "保留源代码，运行对应 Python 环境可重建字节码缓存。"},
	{"后续安装或构建可能需要重新下载软件包，离线操作可能失败。", "确认原软件包来源可访问，再用对应工具重新下载。"},
	{"项目可能无法启动、构建或运行测试。", "保留项目清单和锁文件，用原包管理器恢复依赖，或从回收站还原。"},
	{"相关软件或其功能可能无法启动。当前运行状态未知。", "确认软件支持修复或重装，或从回收站还原原路径。"},
	{"仅凭路径无法判断清理影响，请核对文件用途与引用。", "先确认有备份或原始来源，或从回收站还原原路径。"},
	{"这些文件受保护。", "请使用系统或相关软件提供的管理功能。"},
}

// All files with the same owner and impact reuse one record.
type ownerProfile struct {
	identity fileOwner
	details  [len(ownerEffects)]*fileOwner
}

func (owner *ownerProfile) detail(impact ownerImpact) *fileOwner {
	if owner.details[impact] == nil {
		detail := owner.identity
		detail.impact = impact
		detail.Impact = ownerEffects[impact].impact
		detail.Recovery = ownerEffects[impact].recovery
		owner.details[impact] = &detail
	}
	return owner.details[impact]
}

func ownerAt(path, name, kind, evidence string) *ownerProfile {
	return &ownerProfile{identity: fileOwner{Path: path, Name: name, Kind: kind, Evidence: evidence}}
}

func unassignedOwner(path string) *ownerProfile {
	return ownerAt(path, filepath.Base(path), "folder", "按直接上级目录归组，未识别项目或软件")
}

var softwareOwnerNames = map[string]string{
	".workbuddy": "WorkBuddy",
	".trae":      "Trae",
	".gemini":    "Gemini",
	".dsh":       "DSH",
	".claude":    "Claude",
	".codex":     "Codex",
}

var ownerMarkerPriority = map[string]int{
	"package.json": 1, "pyproject.toml": 2, "go.mod": 3, "cargo.toml": 4,
	"composer.json": 5, "pom.xml": 6, "requirements.txt": 7, ".git": 8,
}

type ownerContext struct {
	profile    *ownerProfile
	dependency bool
}

func directoryOwner(dir fs.Item, children []fs.Item, context ownerContext, candidates *cleanup.Result) ownerContext {
	name := strings.ToLower(dir.GetName())
	// An installed dependency can have its own manifests. Its files still
	// belong to the project or software that installed it.
	switch name {
	case "node_modules":
		context.dependency = true
		if context.profile == nil {
			path := filepath.Dir(dir.GetPath())
			context.profile = ownerAt(path, filepath.Base(path), "folder", "依赖目录的上级，未找到项目标记")
		}
	case ".venv", "venv":
		context.dependency = true
		if context.profile == nil {
			context.profile = ownerAt(dir.GetPath(), dir.GetName(), "environment", "由虚拟环境目录名推断")
		}
	}
	if context.dependency || (context.profile != nil && (context.profile.identity.Kind == "software" || context.profile.identity.Kind == "cache")) {
		return context
	}
	if software := softwareOwnerNames[name]; software != "" {
		context.profile = ownerAt(dir.GetPath(), software, "software", "由软件目录 "+dir.GetName()+" 推断")
		return context
	}
	if candidates != nil {
		if match, ok := candidates.Match(dir); ok && !match.Inherited && match.Category == cleanup.PackageCache {
			context.profile = ownerAt(dir.GetPath(), dir.GetName(), "cache", "扫描规则识别的软件包缓存目录")
			return context
		}
	}
	bestPriority := len(ownerMarkerPriority) + 1
	var marker string
	for _, child := range children {
		name := child.GetName()
		normalized := strings.ToLower(name)
		if priority, ok := ownerMarkerPriority[normalized]; ok && priority < bestPriority &&
			(normalized == ".git" || !child.IsDir()) && child.GetFlag() != '@' {
			bestPriority = priority
			marker = name
		}
	}
	if marker != "" {
		context.profile = ownerAt(dir.GetPath(), dir.GetName(), "project", "目录中存在 "+marker)
	}
	return context
}

func (context ownerContext) impact(grade cleanupGrade) ownerImpact {
	switch grade {
	case gradeA:
		return impactCache
	case gradeB:
		if context.dependency {
			if context.profile.identity.Kind == "software" {
				return impactToolDependency
			}
			return impactDependency
		}
		return impactPackageCache
	case gradeD:
		return impactProtected
	default:
		return impactUnknown
	}
}

// Seed the context for a narrowed overview from ancestors that were scanned.
type fileOwnerResolver map[string]ownerContext

func (resolver fileOwnerResolver) directory(dir fs.Item, candidates *cleanup.Result) ownerContext {
	if dir == nil || analyze.IsVirtualRootDir(dir) {
		return ownerContext{}
	}
	if _, pseudo := dir.(*analyze.ParentDir); pseudo {
		return ownerContext{}
	}
	path := dir.GetPath()
	if cached, ok := resolver[path]; ok {
		return cached
	}
	parent := resolver.directory(dir.GetParent(), candidates)
	children := slices.Collect(dir.GetFilesLocked(fs.SortByName, fs.SortAsc))
	context := directoryOwner(dir, children, parent, candidates)
	resolver[path] = context
	return context
}

func (resolver fileOwnerResolver) describe(item fs.Item, safety fileSafety, candidates *cleanup.Result) *fileOwner {
	parent := item.GetParent()
	context := resolver.directory(parent, candidates)
	if context.profile == nil {
		path := filepath.Dir(item.GetPath())
		key := path + string(filepath.Separator)
		fallback, ok := resolver[key]
		if !ok {
			fallback.profile = unassignedOwner(path)
			resolver[key] = fallback
		}
		context.profile = fallback.profile
	}
	return context.profile.detail(context.impact(safety.Grade))
}
