# MDM Shell 客户端技术文档

## 1. 技术范围

客户端运行时只有 [src/shell/cli.sh](src/shell/cli.sh)，兼容 macOS 自带的 `/bin/bash` 3.2，不依赖 Go、Python、Node.js、`jq`、Homebrew 或
GNU coreutils。

`unsafe0.sh`、`unsafe1.sh` 和历史 `mdm/` Go 实现不属于当前调用链。Node.js 服务、Web 页面和 Shell 客户端分别位于 `src/`、
`src/html/` 和 `src/shell/`，由 SCF 直接部署，不生成第二份 Shell 源码。

当 curl 请求服务根路径时，Node.js 服务端使用 `bash-obfuscate` 生成 `cli.sh` 的混淆版本并返回；浏览器访问仍返回首页。服务端优先读取 `/tmp/mdm-cli-obfuscated.sh`，只有文件不存在时才从部署目录读取原始 `cli.sh`、生成私有临时文件并原子写入该缓存路径，后续请求直接复用，不在源码目录写出第二份 Shell。部署新版本后由新的 SCF 运行实例在自己的 `/tmp` 中重新生成；需要在同一实例内强制刷新时必须先删除该缓存文件。生成脚本通过 `eval` 还原并执行原始逻辑，因此混淆只用于降低直接阅读和随手修改的便利性，不属于密码学加密，也不能阻止有经验的使用者还原脚本。服务端生成失败时返回错误，不会回退发送明文客户端。

当前客户端不包含设备授权检查、架构二进制下载、诊断日志上传或通用遥测。正常桌面模式下由普通用户启动时，完成 sudo 密码验证后会在 root Shell 内重新下载并执行服务端返回的
`cli.sh`；除此之外，常规网络请求包括可选的简体中文语言包下载，以及使用者确认法律风险声明后发送的一次确认 Ping。

首页“分析监管”使用独立的 `src/shell/college.sh` 元数据采集器，仅支持正常桌面 macOS。采集器启动时先选择简体中文或英语；交互终端优先使用上下方向键和 Enter，缺少 `tput`、终端能力或交互输入时降级为数字选择，并保持 `mdm_lang=0` 为英语、`mdm_lang=1` 为简体中文的外部约定。所选语言会跨 sudo root 子进程保留，后续数据范围、确认、密码、进度、结果和错误提示只使用所选语言。网页为该模式只显示桌面终端说明，隐藏 Recovery 入口、Recovery 终端位置、磁盘装载和清理完成提示；采集器在 Recovery 或无法确认桌面环境时直接退出，服务端也拒绝 `run_mode=recovery` 的分析载荷。Recovery 维护仍仅由 `cli.sh` 的“绕过监管”流程支持。

主动报告运行只读的 `profiles status -type enrollment`，将 MDM、User Approved 和 ADE/DEP 状态规范化为枚举；同时检查 `ConfigurationProfiles/Settings` 下的 ADE/DEP 标记。读取 `.cloudConfigRecordFound` 时只保留 `CloudConfigProfile.ConfigurationURL` 的主机名，不上传完整 URL、证书或 plist 内容。报告不会执行 `profiles renew`，因此不会为了扫描主动刷新注册记录。进程扫描使用 `ps -axo comm=`，只采集运行中可执行文件的名称或路径，排除用户目录、挂载卷、临时目录以及全部命令参数。

采集器还会读取 `/etc/hosts`，但只上传其中规范化、去重后的 `apple.com` 主机名，不上传对应 IP、注释或非 Apple 条目。服务端允许 `iprofiles.apple.com`、`mdmenrollment.apple.com` 和 `deviceenrollment.apple.com`；发现其他 `apple.com` 主机名时将报告标记为“系统不健康”，提示部分 Apple 服务可能不可用。Apple ManagedClient 和 Apple Declarative Device Management 属于正常 macOS 系统组件，报告会明确提示无需担心。注册状态、ADE/DEP 标记、Hosts 域名和运行进程均为只读证据，服务端不会为这些类型生成删除命令。

`college.sh` 必须取得 root 权限。由普通用户启动时，它会显示带 `*`、支持退格的密码输入，拒绝空密码和包含空白字符的密码，并通过 `sudo -S -v` 最多验证三次，再使用 `sudo -nE` 启动 root 子进程。密码仅保存在 Shell 变量中并通过 stdin 传递；root 子进程先读取继承密码、恢复 stdin 到 `/dev/tty`，随后立即清空密码。取得 root 后，脚本先展示扫描与上传范围并要求输入 `YES`；拒绝时不会创建报告会话、扫描系统或上传数据。确认后才创建报告、扫描、校验载荷大小并上传。密码不进入参数、环境变量或临时文件。

报告使用 128 位随机 ID 作为唯一访问凭据，不再生成或要求额外的 6 位密码。URL 不公开索引并随报告到期失效；获得完整 URL 的人可以在有效期内查看和重新分析报告，因此使用者应将该 URL 视为敏感信息。

刷新报告页面通过 `GET /api/college/:id` 读取已保存的 `analysis`。“重新分析”按钮调用 `POST /api/college/:id/reanalyze`，以同一随机报告 ID 定位原始 `payload`，使用当前 `report-analyzer.js` 规则重新生成结果并覆盖 `analysis`。该操作不重新扫描设备，不改变报告 ID 或过期时间；原始载荷不可用、报告未完成或已经过期时不会写入。

## 2. 启动流程

```text
读取继承密码（仅 sudo 重启后的子进程）
  → 选择语言
  → 加载并验证中文语言包
  → 显示授权与风险声明并要求主动确认
  → 读取并显示设备序列号，发送一次声明确认 Ping
  → 检测 normal / recovery
  → normal 普通用户密码验证并由 root Shell 重新下载启动
  → 选择和校验目标系统卷
  → 显示主菜单
```

语言选择完成后，所有界面只显示所选语言。中文包不可用时切换为英语。

首次进入流程时会以所选语言显示精简的授权与风险声明。声明明确限制为合法授权、非商业用途，提示系统修改与数据丢失风险，并禁止用于未经授权的设备或规避组织的合法管理。声明还会在确认前告知序列号、代理转发链中的全部有效请求
IP、时间和可能根据首个 IP 推导的大致地区等数据处理行为。确认提示为默认拒绝的 `[y/N]`，仅输入 `y`、`Y`、`yes` 或 `YES` 才能继续；其他输入或
stdin 结束时立即退出。正常桌面系统的普通用户在 sudo 提权前确认并发送一次，root 子进程不重复声明或 Ping；直接以 root 运行或在
Recovery 运行时仍必须确认。

确认后，客户端读取并在终端显示序列号，再使用 `curl -fSL` 请求 `${MDM_SERVER_URL%/}/ping?sn=...`。服务端现有 `/ping` 路由会校验序列号并记录序列号、首个请求 IP、完整有效代理 IP 链、时间和位置字段；位置同步任务可能根据首个 IP 补充大致地区。Ping 不限制地址协议，也不设置重试或超时；客户端不检查或展示请求结果，缺少
`curl`、序列号读取失败或请求失败都不会阻断后续维护功能。

## 3. 顶部配置变量

所有可配置变量与全局状态集中在 `cli.sh` 顶部。

| 变量                     | 默认值或格式             | 用途                                                                     |
|--------------------------|--------------------------|--------------------------------------------------------------------------|
| `RUN_MODE`               | 空、`normal`、`recovery` | 空值时自动检测；显式值用于测试或特殊系统                                 |
| `DRY_RUN`                | `0`                      | 设为 `1` 时只打印系统修改命令                                            |
| `TARGET_VOLUME`          | 空                       | Recovery 中显式指定 `/Volumes/...` 目标卷                                |
| `mdm_lang`               | 空、`0`、`1`             | 空值显示语言选择；`0` 英语，`1` 简体中文                                 |
| `MDM_SERVER_URL`         | `http://127.0.0.1:9000`  | Node.js 服务端基址；当前用于本地联调，生产设置为 `https://mdm.xrsec.fun` |
| `MDM_LANG_PACK_URL`      | 语言包 URL               | 覆盖中文语言包下载地址；生产环境必须使用 HTTPS                           |
| `MDM_LANG_PACK_FILE`     | 空                       | 开发测试时从本地文件加载中文包                                           |
| `CLEAR_TRANSIENT_OUTPUT` | `1`                      | 设为 `0` 时保留菜单等临时输出                                            |
| `TRASH_DATE_FORMAT`      | `%Y%m%d%H%M%S`           | 桌面垃圾篓重命名的时间戳格式                                             |
| `MDM_KEYWORDS`           | 小写空格分隔列表         | 覆盖内置的 MDM/UEM/RMM 文件与服务匹配词                                  |
| `MDM_EXTRA_KEYWORDS`     | 小写空格分隔列表         | 在内置或自定义 `MDM_KEYWORDS` 后追加部署专用关键词                       |

`DRY_RUN=1` 的输出逐项对应真实写入命令，包括 Hosts 文件替换、垃圾篓权限设置、网络缓存刷新和完整的用户创建步骤。读取系统状态、枚举匹配目标及路径校验仍会真实执行，因此不存在或未匹配到的目标不会生成虚假的修改命令。

Ping 会直接使用 `MDM_SERVER_URL` 配置的地址，不校验协议。默认从同一服务的 `/lang/zh-CN.lang` 加载语言包，也可通过
`MDM_LANG_PACK_URL` 覆盖为其他 HTTPS 地址。

## 4. 语言包

英语文案内置在 `cli.sh`。简体中文位于 [src/shell/lang/zh-CN.lang](src/shell/lang/zh-CN.lang)，格式为 UTF-8 纯文本：

```text
KEY<Tab>VALUE
```

语言包加载规则：

- 使用 `curl -fSL`、连接超时和有限重试。
- 下载到 `umask 077` 保护的 `/tmp` 临时文件。
- 拒绝 HTML、JSON、重复键、非法键、非两列数据和缺少必要键的文件。
- 语言包只使用 `awk` 解析为数据，绝不 `source`、`eval` 或执行。
- 校验失败时删除临时文件并切换到英语。
- 授权与风险声明的中文逐行文案属于必要键；缺少任何一行都会使语言包校验失败并回退英语，避免显示不完整声明。

## 5. 权限与密码

### 正常桌面模式

非 root 启动时，脚本取得当前用户名称并显示密码输入框。每个输入字符只显示 `*`，支持退格；空密码和包含空格、Tab 等空白字符的密码会被拒绝。

密码通过以下步骤处理：

1. 经 `sudo -S -v` 验证，最多尝试三次。
2. 验证成功后，通过标准输入传递给重新启动的 root `cli.sh`。
3. 密码不出现在命令行、环境变量、日志或临时文件。
4. root 子进程读取密码后，将标准输入恢复为 `/dev/tty`，保证方向键菜单正常工作。
5. FileVault 使用密码后立即清空内存变量；退出和信号处理也会清空。

`MDM_PASSWORD_STDIN=1` 只表示 root 子进程需要从 stdin 读取一行密码，本身不包含密码。root 子进程通过 sudo 自动设置的 `SUDO_USER` 识别原登录用户，不再额外传递用户名。
提权重启使用 `sudo -nE`：`-n` 保证 sudo 凭据失效时立即失败而不再次显示系统密码提示，`-E` 将本次命令临时设置的非敏感运行配置传给 root 子进程；密码不属于保留的环境变量。

无论入口是本地文件还是 `/dev/fd/*`，正常桌面模式的非 root 脚本都会在 sudo 内启动 `/bin/bash -c`，通过
`curl -kfsSL "${MDM_SERVER_URL%/}/"` 创建进程替换并运行第二层 `cli.sh`，随后非 root 父进程以状态 0 退出。第二层继承 `MDM_PASSWORD_STDIN=1`，
读取密码后恢复 `/dev/tty`，不会重复显示法律声明或发送确认 Ping。

### Recovery

Recovery 默认已经是 root，因此此模式不检查当前 UID、不显示桌面 sudo 密码框，也不调用 `sudo`。选择目标系统后，如果配对的 APFS Data/数据卷
处于加密锁定状态，脚本显示带 `*` 反馈的密码输入，并通过 `diskutil apfs unlockVolume ... -stdinpassphrase` 的标准输入传递密码。密码只暂存在
当前 Shell 内存中，不进入参数、环境变量或临时文件，命令返回后立即清空。

## 6. 模式和目标卷

环境检测不能只依赖 `open`。出现 `/System/Installation` 或 Recovery Springboard 时优先识别为 Recovery；自动识别正常桌面模式则同时要求
`open`、Finder、用户目录，并综合当前控制台用户、本地用户数据库和 `/Volumes`。这可以避免 Recovery 临时系统中的控制台用户或用户数据库被误当作已启动的桌面系统。

正常模式的 `TARGET_ROOT` 与 `SYSTEM_ROOT` 都是 `/`。路径必须通过统一函数拼接，因此生成 `/etc/hosts`，不会生成
`//etc/hosts`。

降级扫描 `/Volumes` 时会查找包含 macOS 系统目录，并能通过 firmlink 访问 `Users`、`var/db` 和 `Library` 的 System 卷：

```text
/Volumes/Macintosh HD
  ↔ /Volumes/Macintosh HD - Data
```

扫描会排除 `macOS Base System`、`OS X Base System`、Recovery 支持卷，以及与当前 `/` 使用同一文件系统设备的卷，避免把正在运行的
Recovery 系统本身选作维护目标。显式 `TARGET_VOLUME` 同样不能指向这些运行卷。

Recovery 优先解析 `diskutil apfs list` 中角色为 `System` 和 `Data` 的卷，再使用 `diskutil info -plist` 返回的 `APFSVolumeGroupID` 精确配对，
不根据 `- Data` 或 `- 数据` 后缀猜测。只有一个合格系统时自动选中；双系统或更多候选时使用 `diskutil` 返回的真实卷名让用户选择。

选择完成后，脚本检查配对 Data/数据卷的 `Locked` 状态。锁定时要求用户输入密码并由脚本通过 stdin 调用 `diskutil` 解锁；已解锁但未挂载时直接
调用 `diskutil mount`。Data/数据卷只用于解锁和建立可用的卷组命名空间，不会成为操作根目录。随后挂载并校验 System 卷；成功后
`SYSTEM_ROOT` 与 `TARGET_ROOT` 都指向例如 `/Volumes/Macintosh HD`，脚本通过该路径的 firmlink 访问 `Users`、`var/db` 和 `Library`。
`diskutil` 或 `plutil` 缺失、无法提供 APFS 列表时才降级扫描 `/Volumes`。

多个候选系统卷必须由用户选择。目标根目录必须经过路径校验，空值、`/` 误判或目标卷以外路径不得执行递归删除。

## 7. 菜单与临时输出

语言、主菜单、系统卷和用户选择使用上下方向键及 Enter。若 `tput` 不存在、终端能力不可用或 stdin/stderr 不是 TTY，则降级为数字菜单。

`clear_last_lines N` 会逐行向上移动并清除 N 个逻辑行。菜单会根据标题、提示、空行、选项和错误次数动态统计打印行数，确认后删除临时界面。终端自动换行形成的物理行不在逻辑计数内；调试时可设置：

```bash
CLEAR_TRANSIENT_OUTPUT=0 /bin/bash src/shell/cli.sh
```

“清理并停用 MDM”执行完并询问是否重启后直接退出，不返回主菜单。

## 8. 删除与垃圾篓

所有删除入口经过 `safe_remove` 和目标路径校验。

正常桌面模式不直接删除，而是移动到当前登录用户的 `~/.Trash`，并重命名为：

```text
原文件名_YYYYmmddHHMMSS_进程ID_序号
```

这可避免同名覆盖。无法确定登录用户、创建垃圾篓或移动失败时会报错，不会回退为直接删除。

Recovery 对已经校验且属于目标卷的路径直接执行删除，因为离线目标用户垃圾篓不适合作为恢复机制。

## 9. 清理并停用 MDM

`bypass_mdm` 按以下顺序尽力执行：

1. 正常模式下尽力清理暂存的 kext，并暂时移除本工具写入的 Apple 注册 Hosts 项。
2. 执行 `profiles renew -type enrollment`，刷新 `.cloudConfigRecordFound` 后读取精确注册域名。
3. 幂等写回 Apple 注册域名和读取到的精确域名 Hosts 屏蔽项。
4. 清理并重建 `ConfigurationProfiles/Settings` 和必要标记文件。
5. 清理 `Store`；重新创建 `Store` 失败不影响总体状态。
6. 按 `MDM_KEYWORDS` 清理系统级 LaunchDaemons、LaunchAgents、Application Support、Preferences、Managed Preferences 和 Applications；再通过目标系统 dscl 数据库遍历所有 UID 501–60000 的普通用户，清理各用户的 LaunchAgents、Application Support、Preferences、Managed Preferences 和 Applications。部署方可通过 `MDM_EXTRA_KEYWORDS` 追加足够精确的关键词。
7. 正常模式下对匹配的 launchd label 尽力执行 `system`、`gui/<uid>` 和 `user/<uid>` 三个 domain 的 `disable` 与 `bootout`。
8. 正常模式下处理 FileVault。
9. 正常模式下依次尝试 `profiles -D -f` 和 `profiles remove -all -f`；任一语法成功即视为 profiles 阶段成功。
10. 刷新 DNS 与 mDNS 缓存。
11. Recovery 的目标系统为 macOS 13 或更高版本且没有普通用户时，进入管理员创建流程。
12. 正常模式完成后，在 `open` 可用时打开 `${MDM_SERVER_URL%/}`，然后询问是否重启并退出主菜单。

单个阶段失败不会中断后续阶段。最后显示“操作完成”或“已尝试执行全部阶段，但部分操作失败”。

### 管理员创建

管理员创建菜单和 Recovery 自动创建流程使用同一实现：

- 默认选择 501–60000 范围内的下一个可用 UID，允许输入同范围内尚未使用的 UID。
- 用户名默认是 `mac<UID>`，也允许输入符合本地账户规则的自定义用户名；显示名称同样可自定义。
- 密码由 `dscl -passwd` 在终端中交互设置，不使用固定密码、环境变量或仓库配置。
- 始终创建 `GeneratedUID`；缺少 `uuidgen` 时使用 Bash 3.2 可用的本地字段生成 UUID。
- 写入后回读验证 `AuthenticationAuthority` 包含 `ShadowHash`，再设置密码并加入 admin 组。
- 创建前拒绝覆盖已有账户、UID 或同名主目录；中途失败会撤销 admin 组成员、账户记录以及本次新建的主目录。
- 创建成功后写入 `.AppleSetupDone`。不恢复旧实现中包含固定密码的 `AuthenticationHint`。

## 10. FileVault

正常模式下：

1. 使用 `fdesetup status` 判断是否已关闭或正在解密。
2. 使用 `fdesetup list` 获取 FileVault 授权用户；多个用户使用方向键选择。
3. 若登录用户在授权列表中，复用 sudo 阶段已验证的内存密码；否则重新显示密码框。
4. 用户名和密码经过 XML 转义后组成 plist，通过 stdin 交给 `fdesetup disable -inputplist`，plist 不落盘。
5. 同时检查退出码、明确成功文本和失败文本。出现 `could not be found`、`was not disabled` 或 `Error:` 时一定判定失败。

Recovery 只为访问目标系统而临时解锁并挂载 Data/数据卷，主流程不调用 FileVault 检查或关闭函数，也不显示相关提示。

## 11. 验证

每次修改 Shell 至少运行：

```bash
/bin/bash -n src/shell/*.sh
git diff --check -- src/shell/cli.sh
```

涉及语言包时还应验证 UTF-8、真实 Tab、键唯一性、必要键集合、HTML/JSON 拒绝以及英语回退。涉及终端、权限或文件行为时使用
PTY、临时目录或 `DRY_RUN=1` 覆盖：

- 方向键、Enter、退格和星号密码反馈。
- normal 普通用户、sudo 重启、root 和 Recovery 路径。
- 单卷、多卷、卷未挂载以及包含空格或中文的卷名。
- `/` 根目录路径拼接和目标路径拒绝。
- 桌面垃圾篓时间戳移动与 Recovery 删除。
- FileVault 成功、用户不存在、非零退出码和错误输出但零退出码。
- 单阶段失败后继续执行以及 MDM 主流程只运行一次。

若安装了 ShellCheck，再运行：

```bash
shellcheck --shell=bash src/shell/*.sh
```

不要在开发机上直接测试真实的破坏性系统修改。
