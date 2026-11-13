import type { Category, Node } from './types';

const categoryLabels: Record<Category, string> = {
  'build-artifact': '构建产物',
  'package-cache': '软件包缓存',
  'app-cache': '应用缓存',
  toolchain: 'SDK 和工具链',
  'ml-model': '机器学习模型',
  installer: '安装包和压缩文件',
  'git-ignored': 'Git 忽略项',
};

export function categoryLabel(category: Category, fallback: string = category): string {
  return categoryLabels[category] ?? fallback;
}

export function displayNodeName(node: Node): string {
  return node.category && node.path === `category:${node.category}`
    ? categoryLabel(node.category, node.name)
    : node.name;
}

const errors = new Map<string, string>([
  ['node not found', '找不到此项目'],
  ['deletion is disabled', '删除功能已禁用'],
  ['permission denied', '权限不足'],
  ['path is outside the scanned root', '路径超出了扫描范围'],
  ['path changed on disk since the scan and now resolves outside the scanned root', '路径在扫描后已更改，现在指向扫描范围之外'],
  ['cannot delete a scanned root', '无法删除扫描的根目录'],
  ['cannot delete an entry nested inside a browsed archive', '无法删除压缩包内部的项目'],
  ['invalid request body', '请求内容无效'],
  ['path has no directory to reveal', '找不到可定位的目录'],
  ['the item is still there after moving it to the trash', '移入回收站后，该项目仍在原位置'],
  ['actions are only available from the local web UI', '此操作只能从本机网页执行'],
  ['move to trash is not supported on Windows', 'Windows 版 Gdu 不支持移入回收站'],
]);

export function localizeError(message: string): string {
  return errors.get(message) ?? message;
}

// Translate built-in hints while preserving commands and user-defined rules.
const hints = new Map<string, string>([
  ['Remove models with `ollama rm`; deleting blobs by hand breaks Ollama', '请用 `ollama rm` 移除模型。手动删除模型数据会影响 Ollama 正常运行。'],
  ['Prune with `hf cache delete`', '可用 `hf cache delete` 清理缓存'],
  ['Re-downloadable from LM Studio', '可以通过 LM Studio 重新下载'],
  ['PyTorch hub cache, re-downloaded on use', 'PyTorch Hub 缓存，下次使用时会重新下载'],
  ['Go module cache, `go clean -modcache`', 'Go 模块缓存，可用 `go clean -modcache` 清理'],
  ['Go build cache, `go clean -cache`', 'Go 构建缓存，可用 `go clean -cache` 清理'],
  ['Cargo registry cache', 'Cargo 注册表缓存'],
  ['Cargo git checkouts', 'Cargo 下载的 Git 仓库'],
  ['Gradle cache', 'Gradle 缓存'],
  ['Gradle distributions, downloaded again by the wrapper', 'Gradle 发行包，启动脚本会重新下载'],
  ['Gradle daemon logs', 'Gradle 后台进程日志'],
  ['Maven repository; may hold locally installed artifacts', 'Maven 本地仓库，可能包含在本机安装的构建产物'],
  ['npm cache, `npm cache clean --force`', 'npm 缓存，可用 `npm cache clean --force` 清理'],
  ['Bun cache, `bun pm cache rm`', 'Bun 缓存，可用 `bun pm cache rm` 清理'],
  ['Downloaded Node archives, `nvm cache clear`', '已下载的 Node.js 压缩包，可用 `nvm cache clear` 清理'],
  ['npx cache, filled again on the next npx run', 'npx 缓存，下次运行 npx 时会重新生成'],
  ['pnpm store, `pnpm store prune`', 'pnpm 存储，可用 `pnpm store prune` 清理'],
  ['Yarn cache, `yarn cache clean`', 'Yarn 缓存，可用 `yarn cache clean` 清理'],
  ['uv cache, `uv cache prune`', 'uv 缓存，可用 `uv cache prune` 清理'],
  ['pip cache, `pip cache purge`', 'pip 缓存，可用 `pip cache purge` 清理'],
  ['Poetry cache, `poetry cache clear`', 'Poetry 缓存，可用 `poetry cache clear` 清理'],
  ['Homebrew cache, `brew cleanup`', 'Homebrew 缓存，可用 `brew cleanup` 清理'],
  ['pnpm cache', 'pnpm 缓存'],
  ['CocoaPods cache, `pod cache clean --all`', 'CocoaPods 缓存，可用 `pod cache clean --all` 清理'],
  ['CocoaPods specs, fetched again by `pod install`', 'CocoaPods 规格文件，运行 `pod install` 时会重新获取'],
  ['Dart/Flutter packages, `dart pub cache clean`', 'Dart 或 Flutter 软件包，可用 `dart pub cache clean` 清理缓存'],
  ['Kotlin/Native toolchain cache', 'Kotlin/Native 工具链缓存'],
  ['Composer cache, `composer clear-cache`', 'Composer 缓存，可用 `composer clear-cache` 清理'],
  ['conda package cache, `conda clean --all`', 'conda 软件包缓存，可用 `conda clean --all` 清理'],
  ['Stack package index', 'Stack 软件包索引'],
  ['Globally installed Bun packages, `bun remove -g`', '全局安装的 Bun 软件包，可用 `bun remove -g` 移除'],
  ['Remove unused ones with `rustup toolchain uninstall`', '可用 `rustup toolchain uninstall` 移除不用的工具链'],
  ['Remove unused Node versions with `nvm uninstall`', '可用 `nvm uninstall` 移除不用的 Node.js 版本'],
  ['Remove unused Pythons with `pyenv uninstall`', '可用 `pyenv uninstall` 移除不用的 Python 版本'],
  ['Remove unused SDKs with `sdk uninstall`', '可用 `sdk uninstall` 移除不用的 SDK'],
  ['Remove unused GHCs with `ghcup rm`', '可用 `ghcup rm` 移除不用的 GHC 版本'],
  ['Remove unused Pythons with `uv python uninstall`', '可用 `uv python uninstall` 移除不用的 Python 版本'],
  ['Emulator images, re-downloadable with the SDK manager', '模拟器镜像，可以通过 SDK 管理器重新下载'],
  ['Android emulators; their data is lost', 'Android 模拟器，删除会丢失模拟器中的数据'],
  ['Re-created when a device connects', '连接设备时会重新生成'],
  ['App archives; keep the ones you may need to symbolicate', '应用归档，请保留可能用于解析崩溃日志的归档'],
  ['Simulators, `xcrun simctl delete unavailable`', '模拟器，可用 `xcrun simctl delete unavailable` 移除不可用的实例'],
  ['Apps rebuild caches; quit the app first', '应用会重建缓存，清理前请先退出应用'],
  ['Xcode rebuilds it', 'Xcode 会重新生成'],
  ["Restored by your package manager's install", '重新安装项目依赖即可恢复'],
  ['Restored by `pod install`', '运行 `pod install` 即可恢复'],
  ['Restored by `mix deps.get`', '运行 `mix deps.get` 即可恢复'],
  ['Restored by `terraform init`', '运行 `terraform init` 即可恢复'],
  ['Recreate the virtualenv after removing', '移除后需要重建虚拟环境'],
  ['Build output; check it is not committed', '构建产物，请先确认它未提交到版本库'],
  ['Ignored by .gitignore; may still be local data', '被 .gitignore 忽略，可能仍包含本地数据'],
]);

export function localizeHint(hint?: string): string | undefined {
  return hint === undefined ? undefined : hints.get(hint) ?? hint;
}
