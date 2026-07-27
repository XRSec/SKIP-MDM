# MDM Shell 项目开发约定

本文件适用于整个仓库。后续修改应以这里的约定为准。

## 当前技术范围

- 当前客户端只使用 Shell，Web 服务使用 Node.js，不再使用 Golang 服务端。
- `src/` 是当前应用源码目录：Node.js 服务端位于其根部，客户端 Shell 位于 `src/shell/`，Web 页面位于 `src/html/`。
- `mdm/`、`server_bak/` 和 `custom/` 中的 Go 代码属于历史实现：不要新增、修改或依赖其中的 Go 代码，也不要把 `go build`、`go run` 或 `go test` 作为当前服务端交付流程的一部分。
- `src/html/` 与 `src/shell/` 都是直接部署的源码目录，不要在构建时复制出第二份 Shell 源码。
- 不要引入 Homebrew、Python、Node.js、`jq` 或 GNU coreutils 等客户端运行依赖。
- `src/shell/cli.sh` 是完整、可独立运行的客户端入口。不要恢复“授权后下载架构二进制再执行”的旧流程，也不要让客户端依赖 `unsafe0.sh`、`unsafe1.sh` 或历史 Go 二进制。
- 客户端技术行为同步记录在 `TECHNICAL.md`。修改提权、密码、Recovery、FileVault、文件删除、菜单或语言包机制时必须同步更新该文档。

## 全局变量与函数状态

- 可配置变量、颜色变量和全局运行状态统一放在 `src/shell/cli.sh` 顶部，并用注释分组；不要在函数之间散落新的全局赋值。
- 函数内部的临时状态使用 `local`。只有确实需要跨函数或跨提权进程保存的值才能成为全局变量。
- 保持现有外部覆盖参数：`RUN_MODE`、`DRY_RUN`、`TARGET_VOLUME`、`mdm_lang`、`MDM_SERVER_URL`、`MDM_LANG_PACK_URL`、`MDM_LANG_PACK_FILE`、`CLEAR_TRANSIENT_OUTPUT` 和 `TRASH_DATE_FORMAT`。
- `MDM_KEYWORDS` 是空格分隔的简单关键词集合，值必须是小写且不能包含空格；精确文件名或含空格的名称应使用独立机制，不能硬塞入该变量。

## 支持的运行环境

- 最低支持 macOS 11.5，持续兼容最新稳定版 macOS。
- 同时支持 Intel Mac（`x86_64`/`i386`）和 Apple Silicon（`arm64`）。需要下载架构相关文件时，统一规范为 `amd64` 和 `arm64`。
- 同时支持以下两类环境：
  - 正常桌面系统，包括从 Terminal 启动后通过 `sudo` 提权的场景。
  - macOS Recovery，包括目标系统卷挂载在 `/Volumes` 下的场景。
- Shell 脚本必须兼容 macOS 自带的 `/bin/bash` 3.2。脚本入口使用 `#!/bin/bash`，验证时显式使用 `/bin/bash`。
- Recovery 中的命令和目录可能少于桌面系统。使用 `open`、`sudo`、`tput`、`profiles`、`fdesetup`、`launchctl`、`dscl`、`plutil`、`unzip` 等命令前必须先检查是否存在，并提供可理解的降级或错误提示。

## Bash 3.2 兼容规则

- 禁止使用关联数组、`mapfile`、`readarray`、`${name,,}`、`${name^^}`、nameref、`wait -n` 等新版本 Bash 特性。
- 不要依赖 GNU 专用参数，例如 `readlink -f`、`grep -P`、`sed -i`、`stat -c` 或 `timeout`。
- macOS 原生命令差异应明确处理，例如使用 BSD `stat -f`，需要原地修改时使用临时文件再原子替换。
- 所有路径和变量展开都要加双引号，特别是 `/Volumes` 下可能包含空格或中文的卷名。
- 临时文件放在 `/tmp`，使用当前进程 ID 或 `mktemp` 避免冲突；设置 `umask 077`，并用 `trap` 清理。
- 不要使用 `eval`，也不要通过 `source`、`bash <(...)` 或管道直接执行网络下载的内容。
- 脚本应尽量幂等：重复运行不能持续追加相同配置，也不能因为目标文件已不存在而误报成功。
- 根目录与相对路径必须通过统一路径拼接函数处理，`TARGET_ROOT="/"` 时不能生成 `//etc/hosts` 之类的双斜杠路径。

## Recovery 与桌面模式

- 环境检测集中在一个函数中，允许通过 `RUN_MODE=normal` 或 `RUN_MODE=recovery` 显式覆盖，便于测试和处理特殊系统版本。
- 不要只通过 `open` 是否存在来判断运行模式。应同时检查目标系统卷、当前控制台用户以及操作所需命令或目录。
- 桌面模式需要 root 权限时，使用 `sudo` 重新执行入口脚本；不要把密码写入命令行、日志、临时文件或语言包。
- 正常桌面模式下，非 root 入口应显示带 `*` 反馈的密码输入，拒绝空密码和包含空白字符的密码，使用 `sudo -S -v` 验证后再以 root 重启入口。密码只能经标准输入传给 root 子进程并暂存在内存中，禁止放入环境变量。
- root 子进程读取继承密码后必须把标准输入恢复为 `/dev/tty`，保证方向键菜单仍能交互；FileVault 使用完密码后立即清空内存变量。
- Recovery 不调用 `sudo`、不显示桌面 root 密码输入框；Recovery 意外不是 root 时直接给出错误提示。
- Recovery 通常已经是 root，但目标系统根目录不是当前 `/`。所有系统文件操作必须基于已经确认的目标卷路径，不能默认使用 `/Volumes/Macintosh HD`。
- 自动发现多个候选系统卷时必须让用户选择；找不到目标卷时提示用户先在“磁盘工具”中装载数据卷。
- 每个修改系统状态的函数都应先验证目标路径属于选定卷，禁止对空变量、`/` 或未确认的卷执行递归删除或权限修改。
- 桌面模式下 `safe_remove` 必须移动项目到当前登录用户的 `~/.Trash`，并以时间戳、进程 ID 和序号重命名以防覆盖；移动失败不得回退为直接删除。Recovery 下才允许对已经校验的目标路径直接删除。

## 菜单和终端输出

- 语言、主菜单、系统卷和用户选择优先使用上下方向键与 Enter；使用 `tput` 前必须检查命令和终端能力，非交互环境或缺少 `tput` 时降级为数字菜单。
- 临时菜单输出通过 `clear_last_lines` 按实际打印的逻辑行数逐行清除。新增临时输出时必须同步维护行数，不能写死与实际界面不一致的常量。
- `CLEAR_TRANSIENT_OUTPUT=0` 必须能够关闭临时输出删除，便于调试和保存终端记录。
- “清理并停用 MDM”完成并询问是否重启后直接退出，不再返回主菜单或重复执行。

## 法律声明同步

- `LEGAL_NOTICE.md` 与 `LEGAL_NOTICE.en.md` 分别是仓库内中、英文完整法律声明的主文档，两种语言的条款含义、章节和确认要求必须保持一致。
- `src/html/legal-notice.html` 与 `src/html/legal-notice.en.html` 是随 Web 页面部署的中、英文完整声明页面，必须分别与对应 Markdown 主文档保持一致；首页只能在使用者首次打开条款弹窗时按当前语言读取、校验并显示对应 HTML 资源，不能在首屏预加载，也不依赖部署根目录文件。
- `src/shell/cli.sh` 中的 `LEGAL_NOTICE_*` 英语兜底文案和 `src/shell/lang/zh-CN.lang` 中的同名中文文案是完整声明的终端精简版，首次执行前必须主动确认。
- 修改允许范围、禁止行为、技术风险、隐私、责任边界、商用限制或确认方式时，必须同步检查并更新上述完整声明资源和 CLI 精简文案，不能只修改其中一处。

## 清理和 FileVault

- `bypass_mdm` 采用尽力执行策略：单个 Hosts、配置目录、厂商文件、服务、profiles 或 FileVault 操作失败时记录状态并继续后续阶段，最后统一报告成功或部分失败。
- `/var/db/ConfigurationProfiles/Store` 重新创建失败是非致命情况，应静默忽略，不得导致整个流程失败。
- 正常模式下 FileVault 使用 `fdesetup status` 判断状态，使用 `fdesetup list` 确定授权用户，并通过 `fdesetup disable -inputplist` 的标准输入传递经过 XML 转义的用户名和密码。
- FileVault 的密码不得进入参数、环境变量或磁盘文件。成功判断必须同时考虑命令状态和输出，包含 `could not be found`、`was not disabled` 或 `Error:` 时禁止显示成功。
- Recovery 中不尝试离线停用 FileVault；目标卷已解锁时继续其他维护并显示明确提示。

## 中英双语与语言包

- 英语文案内置在 `src/shell/cli.sh`，作为始终可用的兜底语言。
- 简体中文保存在 `src/shell/lang/zh-CN.lang`，以 UTF-8、Tab 分隔的 `KEY<TAB>VALUE` 纯文本格式维护。
- `cli.sh` 先完成语言选择，再通过 HTTPS 请求静态语言包。语言包接口不要求设备授权。
- 语言包下载失败、返回 HTML、缺少必要键或格式校验失败时，删除临时文件并自动切换到英语；不能阻断主脚本的基本错误提示。
- 远程语言包只能作为数据解析，严禁将其作为 Shell 代码执行。
- 文案调用使用稳定的语义键，例如 `SERVER_FAILED`，不要恢复奇偶数字下标方案。
- 新增或删除文案时，同步检查英语兜底和所有远程语言包的键集合。
- `mdm_lang=0` 表示英语，`mdm_lang=1` 表示简体中文；在现有调用链完全迁移前保持这个外部约定。

## 网络与隐私

- 客户端不收集、不上传与用户主动请求无关的系统清单、进程列表、用户目录内容或其他诊断日志。
- 用户选择语言并阅读法律风险声明后，终端必须明确告知序列号、请求 IP、时间和可能推导的大致地区等数据处理行为；仅在用户主动输入 `y`、`Y`、`yes` 或 `YES` 后，允许向 `${MDM_SERVER_URL%/}/ping?sn=...` 发送一次序列号以记录本次确认。Ping 失败不得阻断离线功能，sudo 提权后的 root 子进程不得重复发送。除该确认 Ping 和用户另行主动发起的报告功能外，不实现客户端日志上传或通用遥测；错误信息避免包含密码、令牌或完整敏感数据。
- 生产网络请求使用 HTTPS、`curl -fSL`、连接超时和有限重试。本地开发的 loopback 服务可以使用 HTTP；不要对公网地址使用 HTTP，也不要使用 `-k` 跳过 TLS 校验。证书校验失败时应提示用户检查网络和系统时间。
- 下载内容先写入权限受限的临时文件，完成状态、类型和必要字段校验后再使用。
- 不在仓库中新增真实密码、令牌、数据库 DSN、私钥或其他凭据。发现硬编码凭据时不要复制到新文件，应改为部署环境注入。

## 开发和验证

修改 Shell 文件后至少运行：

```bash
/bin/bash -n src/shell/*.sh
git diff --check
```

如果本机安装了 ShellCheck，再运行：

```bash
shellcheck --shell=bash src/shell/*.sh
```

涉及语言包时还要验证：

- 文件是 UTF-8 文本，并使用真实 Tab 分隔键和值。
- 所有调用的文案键在英语兜底中存在。
- 中文包缺失或下载失败时确实回退到英语。
- 文案值不会被 `eval` 或 `source` 执行。

涉及环境或文件系统行为时，至少覆盖以下组合：

- Bash 3.2 语法检查。
- Intel 与 Apple Silicon 架构映射。
- 正常桌面系统的普通用户和 root/sudo 路径。
- Recovery 下单系统卷、多系统卷、卷未挂载三种情况。
- 路径包含空格和中文的系统卷。
- `TARGET_ROOT="/"` 不产生双斜杠路径。
- 桌面删除移动到垃圾篓并生成不冲突的时间戳名称；Recovery 删除不经过垃圾篓。
- 密码框星号与退格反馈、空白密码拒绝、sudo 验证、stdin 跨进程传递和 `/dev/tty` 恢复。
- FileVault 成功、用户不存在、错误码非零和“输出失败但退出码为零”等返回组合。
- 菜单确认后只清除本次打印的临时行，并且 MDM 主流程只运行一次。

无法安全执行真实系统修改的自动化测试，应使用临时目录或 `DRY_RUN=1` 验证路径计算与命令生成；不要在开发机上直接测试破坏性文件操作。

## 修改工作区

- 工作区可能包含用户尚未提交的修改。修改前后都要检查 `git status --short`，保留与当前任务无关的变更。
- 不要使用 `git reset --hard`、`git checkout --` 或其他会覆盖用户改动的命令。
- 不要恢复已经删除的历史构建文件，除非用户明确要求。
- 使用 `apply_patch` 修改文本文件，避免无关的全文件格式化或机械重写。
