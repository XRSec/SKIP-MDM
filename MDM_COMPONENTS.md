# MDM 与企业监管组件匹配记录

> Shell 客户端的架构、运行流程、配置项和安全约束请参阅 [TECHNICAL.md](TECHNICAL.md)。本文档主要维护 MDM、UEM、RMM
> 和企业安全组件的识别关键词与证据。

本文档记录 macOS 上可能与 MDM、UEM、RMM、资产盘点、EDR、DLP 和企业内部管理有关的名称。分类很重要：出现企业安全软件不等于设备已经注册
MDM，出现 Apple 自带进程也不等于设备正受组织管理。

样本来自历史 `client_logs.json`，共 2,428 条记录、1,142 台不同设备，包含 186 种不同的 LaunchAgent 和 353 种不同的
LaunchDaemon。“样本”列使用“记录数 / 设备数”；`0 / 0` 表示历史清单中曾记录该关键词，但本次样本没有发现。匹配时应统一转为小写。

## 主动分析与报告接口

`src/shell/college.sh` 是用户主动运行的一次性采集器，不是后台服务或遥测。它在本机读取系统级 Launch plist、App bundle
元数据、代码签名、系统扩展、顶层 Application Support/Preferences 和安装包 Receipt
ID；明确排除进程列表、用户目录、序列号、主机名、文件内容以及可执行路径之后的命令参数。上传前会显示项目数和字节数，并要求输入
`YES`。

| 接口                           | 方法   | 用途                                                |
|--------------------------------|--------|-----------------------------------------------------|
| `/college.sh`                  | `GET`  | 下载 Bash 3.2 兼容的主动分析脚本                    |
| `/api/college/session`         | `POST` | 扫描前创建空报告，返回随机 URL 和 6 位密码          |
| `/api/college/:id/upload`      | `POST` | 使用报告密码上传不超过 1 MiB 的 JSON 清单并完成分析 |
| `/api/college/:id/unlock`      | `POST` | 使用 6 位密码读取等待中或已完成的报告               |
| `/college/:id?password=123456` | `GET`  | 通过密码参数自动打开报告；验证后从地址栏移除密码    |
| `/college/:id`                 | `GET`  | 显示密码输入框，验证后查看证据和逐路径命令          |

脚本先创建报告并立即在终端输出 URL 和密码，再扫描本机，确认后上传到同一报告。报告默认保存 168 小时，可通过服务端
`COLLEGE_REPORT_TTL_HOURS` 调整，最大 720 小时。报告 URL 使用 128 位随机 ID，不提供公开列表；6 位密码按临时访问码明文存储并随报告一起过期。Apple
原生组件只显示证据，不生成删除命令。第三方命令不会自动执行，用户必须在报告页面手动复制。

直接运行本地脚本：

```bash
sudo /bin/bash src/shell/college.sh
```

本地联调 HTTP 服务时必须显式允许，生产环境仍强制 HTTPS：

```bash
COLLEGE_SERVER_URL=http://127.0.0.1:9000 COLLEGE_ALLOW_HTTP=1 sudo -E /bin/bash src/shell/college.sh
```

没有 `sudo` 时可以设置 `COLLEGE_ALLOW_PARTIAL=1` 降级为普通用户扫描；脚本会明确警告，无法读取的系统元数据不会进入报告。

## Apple 自身的设备管理组件

这些组件由 Apple 随 macOS 提供，不属于第三方监管公司。系统路径中存在它们是正常现象；判断设备是否已注册 MDM，还需要结合
`profiles status -type enrollment`、配置描述文件及 ADE/DEP 激活记录。

| 所属公司 | 类型                   | 建议关键词                         | 已观察到的文件或进程                                                                                                                                                       |        样本 | 判断与注意事项                                               |
|----------|------------------------|------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------:|--------------------------------------------------------------|
| Apple    | MDM 客户端             | `com.apple.mdmclient`              | `com.apple.mdmclient.agent.plist`、`com.apple.mdmclient.daemon.plist`、`com.apple.mdmclient.daemon.runatboot.plist`、`com.apple.mdmclient.plist`、`/usr/libexec/mdmclient` | 2111 / 1125 | 高可信 Apple MDM 组件；仅存在组件不能证明已注册 MDM          |
| Apple    | 托管偏好与注册         | `com.apple.managedclient`          | `com.apple.ManagedClient.agent.plist`、`com.apple.ManagedClient.cloudconfigurationd.plist`、`com.apple.ManagedClient.enroll.plist`、`ManagedClient`                        |     12 / 11 | 高可信 Apple 管理组件；名称匹配应忽略大小写                  |
| Apple    | 声明式设备管理         | `com.apple.devicemanagementclient` | `com.apple.devicemanagementclient.managedeventsd.plist`、`com.apple.devicemanagementclient.teslad.plist`                                                                   |       0 / 0 | Apple 系统组件；比原来的 `com.apple.devicemanagement` 更精确 |
| Apple    | 声明式设备管理守护进程 | `teslad`                           | `/usr/libexec/teslad`、`com.apple.devicemanagementclient.teslad.plist`                                                                                                     |       0 / 0 | 属于 Apple，不是 Tesla 公司代理；不能据此判断设备属于特斯拉  |

本机系统目录还可见 `com.apple.ManagedClient.mechanism.plist`、`com.apple.ManagedClient.startup.plist`、
`com.apple.ManagedClientAgent.agent.plist` 和 `com.apple.ManagedClientAgent.enrollagent.plist`。清理逻辑不应删除
`/System/Library` 或 `/usr/libexec` 中的 Apple 原生文件。

## 商业 MDM、UEM 与设备管理服务商

| 产品或服务                 | 所属公司                | 建议关键词                    | 已观察到的文件或目录                                                                                                       |    样本 | 可信度与处理建议                                                         |
|----------------------------|-------------------------|-------------------------------|----------------------------------------------------------------------------------------------------------------------------|--------:|--------------------------------------------------------------------------|
| Addigy                     | Addigy                  | `addigy`                      | `com.addigy.agent.plist`、`com.addigy.auditor.plist`、`com.addigy.compliance-agent.plist`、`com.addigy.self-service.plist` |   1 / 1 | 高；建议保留                                                             |
| Ivanti UEM                 | Ivanti                  | `ivanti`                      | 本次样本未发现                                                                                                             |   0 / 0 | 产品归属明确，保留作历史兼容                                             |
| MobileIron                 | Ivanti（原 MobileIron） | `mobileiron`                  | 本次样本未发现                                                                                                             |   0 / 0 | 产品归属明确，保留作旧版兼容；MobileIron 已被 Ivanti 收购                |
| Jamf Pro、Connect、Protect | Jamf                    | `jamf`                        | `com.jamf.management.*`、`com.jamf.connect.*`、`com.jamfsoftware.jamf.plist`、`JamfProtect.app`                            | 23 / 15 | 高；会同时覆盖 Jamf MDM 与 Jamf 安全产品                                 |
| Kandji                     | Kandji                  | `kandji`                      | `io.kandji.kandji-agent.plist`、`io.kandji.kandji-daemon.plist`、`io.kandji.Kandji.plist`                                  |   2 / 1 | 高；建议保留                                                             |
| Mosyle                     | Mosyle                  | `mosyle`                      | `com.mosyle.macos.mdm.agent.plist`、`com.mosyle.macos.mdm.msnotificationd.plist`、`MosyleSecurity.app`                     |   7 / 4 | 高；会同时覆盖 Mosyle MDM 与安全组件                                     |
| Rippling IT                | Rippling                | `rippling`                    | `com.rippling.it.tray.plist`、`com.rippling.open_desktop.plist`、`RipplingCompanyId.txt`                                   |   2 / 1 | 高；建议保留                                                             |
| Workspace ONE（新命名）    | Omnissa                 | `workspaceone`、`com.ws1`     | `WorkspaceONE`、`com.ws1.deemd.plist`、`com.ws1.ws1etlm.plist`                                                             |   1 / 1 | 高；仅有 `workspaceone` 会漏掉 `com.ws1.*` 服务                          |
| Workspace ONE（旧命名）    | VMware，现 Omnissa      | `com.vmware.deem`             | `com.vmware.deem.MacUIEvents.plist`、`com.vmware.deemd.plist`、`com.vmware.vmwetlm.plist`                                  |   1 / 1 | 高；不要使用宽泛的 `vmware`，否则会误匹配 Fusion 和 Horizon              |
| AirWatch                   | VMware，现 Omnissa      | `airwatch`                    | 本次样本未发现                                                                                                             |   0 / 0 | 旧版 Workspace ONE 名称，保留作历史兼容                                  |
| Microsoft Intune Agent     | Microsoft               | `intune`                      | `com.microsoft.intuneMDMAgent.plist`、`com.microsoft.intuneMDMAgent.daemon.plist`                                          |   2 / 2 | 高；建议保留                                                             |
| Intune Company Portal      | Microsoft               | 精确匹配 `Company Portal.app` | `Company Portal.app`                                                                                                       |   3 / 3 | 高；当前空格分隔关键词机制无法安全表达，不要退化成 `company` 或 `portal` |
| JumpCloud                  | JumpCloud               | `jumpcloud`                   | `com.jumpcloud.darwin-agent.plist`、`com.jumpcloud.agent-updater.plist`、`Jumpcloud.app`                                   |   1 / 1 | 高；仓库现有规则已经包含                                                 |
| Fleet / fleetd             | Fleet Device Management | `fleetdm`                     | `com.fleetdm.orbit.plist`、`Fleet Desktop.app`、`/opt/orbit/`                                                              |   2 / 1 | 高；使用 `osquery` 不能完整匹配 Fleet                                    |
| NinjaOne RMM               | NinjaOne                | `ninjarmm`                    | `com.ninjarmm.agentd.plist`、`com.ninjarmm.patcher.plist`、`NinjaRMMAgent`                                                 |   3 / 1 | 高；属于 RMM/终端管理，不是纯 Apple MDM                                  |
| Automox                    | Automox                 | `automox`                     | `com.automox.agent.plist`、`com.automox.agent-ui.plist`、`Automox`                                                         |   1 / 1 | 高；属于补丁与终端管理                                                   |
| Tanium Client              | Tanium                  | `tanium`                      | `com.tanium.taniumclient.plist`                                                                                            |   2 / 1 | 高；属于终端管理与资产/安全平台                                          |

## 企业资产盘点、权限控制与内部管理代理

这些项目可能收集资产信息、下发企业策略或控制权限，但不一定负责 Apple MDM 注册。

| 产品或内部系统                   | 所属公司或组织                                 | 建议关键词                         | 已观察到的文件                                                                                                            |    样本 | 说明                                                                   |
|----------------------------------|------------------------------------------------|------------------------------------|---------------------------------------------------------------------------------------------------------------------------|--------:|------------------------------------------------------------------------|
| CorpLink                         | 火山引擎/字节跳动生态（根据 bundle ID 推断）   | `corplink`                         | `com.corplink.mdm.policy.plist`、`com.volcengine.corplink.service.plist`、`com.corplink.appblocker.plist`、`CorpLink.app` |   8 / 4 | 高可信企业管理套件；包含明确的 MDM policy 组件                         |
| Disney Enrollment                | The Walt Disney Company（根据 bundle ID 推断） | `com.disney.enroll`                | `com.disney.enrollboot.plist`、`com.disney.enroll-check.plist`、`Disney Enrollment.app`                                   |   2 / 1 | 中高；企业内部注册工具，不一定是完整 MDM 服务商                        |
| Google ezmac                     | Google                                         | `com.google.corp.ezmac`            | `com.google.corp.ezmac.plist`、`com.google.corp.ezmac-postlogin.plist`                                                    |   6 / 5 | 高；名称明确指向 Google 企业内部 Mac 管理流程                          |
| Santa                            | Google                                         | `com.google.santa`                 | `com.google.santa.plist`、`bundleservice.plist`、`metricservice.plist`、`syncservice.plist`                               |   1 / 1 | 高；应用执行控制和安全策略工具，不是 MDM                               |
| Nudge                            | Mac Admins 开源社区                            | `com.github.macadmins.nudge`       | `com.github.macadmins.Nudge.plist`                                                                                        |   5 / 2 | 中高；通常由企业管理系统部署，用于催促 macOS 更新                      |
| Outset                           | Mac Admins 开源社区                            | `com.github.outset`                | `boot.plist`、`login.plist`、`login-privileged.plist`、`on-demand.plist`                                                  |   3 / 1 | 中高；企业登录/启动脚本执行框架，本身不是 MDM                          |
| Baseline                         | Second Son Consulting                          | `com.secondsonconsulting.baseline` | `com.secondsonconsulting.baseline.plist`                                                                                  |   1 / 1 | 中高；企业合规基线工具                                                 |
| DEPNotify                        | 开源 Mac 管理工具                              | `com.arekdreyer.depnotify`         | `com.arekdreyer.DEPNotify-prestarter.plist`                                                                               |   2 / 1 | 中；常用于 ADE/DEP 部署流程，但不代表具体 MDM 厂商                     |
| Freshservice Discovery Agent     | Freshworks                                     | `freshservice`                     | 官方名称为 `freshservice.agent.daemon.plist`，本次样本未发现                                                              |   0 / 0 | 资产发现与库存代理，不是纯 MDM；会采集硬件和软件清单                   |
| CatchOn Agent                    | CatchOn / Lightspeed 生态                      | `catchon`                          | `com.catchon.agent.plist`、`com.catchon.agent.updater.plist`                                                              |   4 / 1 | 中；资产与应用使用分析代理                                             |
| AppMeter                         | Atos                                           | `com.atos.appmeter`                | `com.atos.appmeter2.plist`                                                                                                |   2 / 1 | 中；名称和 bundle ID 指向应用计量/资产统计                             |
| BeyondTrust Privilege Management | BeyondTrust                                    | `beyondtrust`、`avecto`            | `com.beyondtrust.interrogator.plist`、`com.avecto.defendpointd.plist`、`PrivilegeManagement.app`                          |   5 / 2 | 高；`avecto` 是历史命名，属于端点权限控制而非 MDM                      |
| Microsoft DLP                    | Microsoft                                      | `com.microsoft.dlp`                | `com.microsoft.dlp.install_monitor.plist`                                                                                 |   4 / 1 | 高；数据防泄漏策略组件，不是 MDM                                       |
| LVM Agent                        | 厂商待核实                                     | `com.lvmagent`                     | `core.plist`、`filemonitor.plist`、`manageproxy.plist`、`screen.plist`                                                    |   3 / 1 | 中；具备文件监控、代理和屏幕组件，归属需通过代码签名确认               |
| Zuler DaaS                       | 厂商待核实                                     | `com.zuler.daas`                   | `com.zuler.daas.launchhelper.plist`                                                                                       |   2 / 2 | 低到中；名称显示为 DaaS 启动组件，公开归属不足                         |
| Custodian                        | 厂商待核实                                     | 精确匹配 `custodian.plist`         | `custodian.plist`                                                                                                         |   5 / 2 | 低；名称过于通用，不能直接加入模糊关键词                               |
| AMSDStat                         | 厂商待核实                                     | 精确匹配 `amsdstat.plist`          | `amsdstat.plist`                                                                                                          | 51 / 40 | 低；样本较多但缺少 bundle ID，需读取 plist、签名或安装收据确认         |
| Amazon 内部管理代理              | Amazon                                         | 暂无已核实关键词                   | 本次样本无可确认记录                                                                                                      |   0 / 0 | 不应把未经证实的 `orthus` 直接归到 Amazon；发现真实 bundle ID 后再记录 |
| Tesla 内部管理代理               | Tesla                                          | 暂无已核实关键词                   | 本次样本无可确认记录                                                                                                      |   0 / 0 | `teslad` 是 Apple 系统进程，不是 Tesla 企业代理                        |

## 企业 VPN、零信任与网络准入

这类代理经常由公司强制安装，能够实施身份验证、网络准入、流量检查或终端合规策略，但通常不是 Apple MDM 本身。

| 产品或服务            | 所属公司            | 建议关键词                     | 已观察到的文件                                                                        |    样本 | 说明                                                   |
|-----------------------|---------------------|--------------------------------|---------------------------------------------------------------------------------------|--------:|--------------------------------------------------------|
| 阿里云 SDP            | 阿里云              | `com.aliyun.security.sdp`      | `com.aliyun.security.sdpagentse.plist`                                                |   1 / 1 | 企业零信任/软件定义边界代理                            |
| Cisco Secure Client   | Cisco               | `com.cisco.secureclient`       | `gui.plist`、`notification.plist`、`vpnagentd.plist`、`iseposture.plist`              |   4 / 3 | VPN、网络准入和 posture 检查                           |
| Cisco AnyConnect      | Cisco               | `com.cisco.anyconnect`         | `com.cisco.anyconnect.ciscod64.plist`                                                 |   1 / 1 | Secure Client 的旧产品线                               |
| FortiClient           | Fortinet            | `com.fortinet.forticlient`     | `fortiagent.plist`、`epctrl.plist`、`ztagent.plist`、`ztnafw.plist`、`vpn.plist`      |   3 / 2 | VPN、ZTNA、终端合规和安全组件                          |
| GlobalProtect         | Palo Alto Networks  | `com.paloaltonetworks`         | `com.paloaltonetworks.gp.pangpa.plist`、`pangps.plist`、`pangpsd.plist`               |   7 / 4 | 企业 VPN 与零信任访问                                  |
| Pulse Secure          | Pulse Secure/Ivanti | `pulsesecure`                  | `net.pulsesecure.AccessService.plist`、`PulseOpswatServiceAgentbased.plist`           |   4 / 2 | 企业 VPN/网络准入                                      |
| EasyConnect、aTrust   | 深信服 Sangfor      | `sangfor`                      | `ECAgentProxy.plist`、`EasyMonitor.plist`、`aTrustDaemon.plist`、`aTrustTunnel.plist` | 39 / 22 | 企业 VPN、零信任和终端准入                             |
| UniAccess             | 联软科技            | `leagsoft`                     | `com.leagsoft.uniaccess.plist`                                                        |   3 / 1 | 企业网络准入/终端安全                                  |
| Trust/LegendSec Agent | 网御星云/LeadSec    | `legendsec`                    | `trustdservice.plist`、`trustuninstall.plist`、`jobblesshelper.plist`                 |  11 / 1 | 企业信任与终端准入组件                                 |
| Lightspeed Signal     | Lightspeed Systems  | `lightspeedsystems`            | `com.lightspeedsystems.lssignal.plist`、`signal.plist`                                |   4 / 1 | 教育/企业设备可见性与安全代理                          |
| TigerSec ZRA          | TigerSec            | `tigersec`                     | `cn.tigersec.client-zra.plist`、`client.uninstall.plist`                              |   1 / 1 | 零信任远程访问客户端                                   |
| DataCloak             | DataCloak           | `datacloak`                    | `com.datacloak.softwareupdate.plist`                                                  |   3 / 1 | 企业数据安全客户端；样本仅暴露更新组件                 |
| Cloudflare WARP       | Cloudflare          | `com.cloudflare.1dot1dot1dot1` | `com.cloudflare.1dot1dot1dot1.macos.warp.daemon.plist`                                |   4 / 3 | 可能是个人 VPN 或企业 Zero Trust，不能单凭存在判定监管 |
| BIG-IP Edge Client    | F5                  | `com.f5.edgeclient`            | `com.f5.edgeclientagent.uninstallhelper.plist`、`BIG-IP Edge Client.app`              |   2 / 1 | 企业 VPN/访问代理                                      |
| 度管家                | 百度                | `com.baidu.duguanjia`          | `com.baidu.DuGuanJia.MainSvc.plist`、`VPNSvc.plist`                                   |   8 / 1 | 企业管理/VPN 组件                                      |
| 堡垒机本地代理        | 启明星辰 Venustech  | `venustech`                    | `com.venustech.bastion.local.plist`                                                   |   2 / 1 | 堡垒机与企业访问控制                                   |
| uSmartView QoE Agent  | ZTE                 | `com.zte.cn.usmartview`        | `com.zte.cn.uSmartView.QoEAgent.plist`                                                |   1 / 1 | 企业网络质量代理，不是 MDM                             |
| 未识别安全客户端      | 厂商待核实          | 精确匹配 `sec-client.plist`    | `sec-client.plist`                                                                    |   1 / 1 | 名称通用，必须读取 plist 或签名后再归属                |

## EDR、安全与合规代理

只有在目标明确包含企业安全软件时，才把这些关键词加入清理范围。它们的存在不能单独证明设备注册了 MDM。

| 产品                            | 所属公司                 | 建议关键词           | 已观察到的文件或进程                                                               |     样本 | 说明                                          |
|---------------------------------|--------------------------|----------------------|------------------------------------------------------------------------------------|---------:|-----------------------------------------------|
| SentinelOne                     | SentinelOne              | `sentinelone`        | `com.sentinelone.agent.plist`、`com.sentinelone.agent-helper.plist`                |    3 / 1 | EDR，不是 MDM                                 |
| Falcon                          | CrowdStrike              | `crowdstrike`        | `com.crowdstrike.falcon.UserAgent.plist`、`Falcon.app`                             |    2 / 2 | EDR；`crowdstrike` 比宽泛的 `falcon` 更准确   |
| Microsoft Defender for Endpoint | Microsoft                | `com.microsoft.wdav` | `Microsoft Defender.app`、`wdavdaemon`、`wdavdaemon_enterprise`                    |    5 / 2 | EDR，不是 Intune 本身                         |
| McAfee Endpoint Security        | McAfee/Trellix           | `mcafee`             | `com.mcafee.agent.ma.plist`、`com.mcafee.ssm.*`、`/Library/McAfee/`                |    3 / 1 | EDR/防病毒                                    |
| Qualys Cloud Agent              | Qualys                   | `qualys`             | `com.qualys.cloud-agent.plist`、`QualysCloudAgent.app`                             |    1 / 1 | 漏洞与合规代理                                |
| Cyberhaven                      | Cyberhaven               | `cyberhaven`         | `io.cyberhaven.lightbeam.*.plist`                                                  |    1 / 1 | DLP/数据安全代理                              |
| Endpoint Security for Mac       | 无法仅凭样本确认具体厂商 | `epsecurity`         | `com.epsecurity.EndpointSecurityforMac.plist`、`com.epsecurity.bdcoreissues.plist` |    1 / 1 | 安全软件；厂商归属需读取签名或安装包收据确认  |
| Avira Security                  | Avira                    | `com.avira.security` | `UIAgent.plist`、`statusmenu.plist`、`privd.plist`、`installd.plist`               |    2 / 2 | 防病毒/终端安全，不是 MDM                     |
| Avast                           | Avast                    | `com.avast.av`       | `com.avast.av.uninstaller.xpc.plist`                                               |    1 / 1 | 防病毒；样本只观察到卸载组件                  |
| Kaspersky                       | Kaspersky                | `com.kaspersky.kav`  | `agent.plist`、`app.plist`、`kavd.plist`                                           |    1 / 1 | 防病毒/终端安全                               |
| Lemon                           | 腾讯                     | `com.tencent.lemon`  | `LemonMonitor.plist`、`Lemon.plist`、`Lemon.listen.plist`                          | 145 / 25 | 通常是个人清理/安全软件，不应默认视为企业监管 |

## 远程协助与远程控制代理

远程控制工具可能由企业 IT 部署，也可能由用户自行安装。它们应记录为风险或运维线索，但不应自动等同于 MDM。

| 产品                 | 所属公司            | 建议关键词              | 已观察到的文件                                                                |     样本 | 判断                               |
|----------------------|---------------------|-------------------------|-------------------------------------------------------------------------------|---------:|------------------------------------|
| ToDesk               | ToDesk/海南有趣科技 | `com.youqu.todesk`      | `session.plist`、`startup.plist`、`service.plist`、`camsession.plist`         | 190 / 52 | 远程控制；样本量大，可能是个人安装 |
| 向日葵               | 贝锐 Oray           | `com.oray.sunlogin`     | `agent.plist`、`startup.plist`、`helper.plist`、`sunlogin.plist`              | 140 / 25 | 远程控制；不能单独判断企业监管     |
| AweSun               | 贝锐 Oray           | `com.oray.awesun`       | `agent.plist`、`startup.plist`、`helper.plist`                                |  17 / 10 | 远程控制                           |
| UU 远程              | 网易                | `com.netease.uuremote`  | `agent.plist`、`daemon.plist`                                                 | 117 / 16 | 远程控制/远程游戏                  |
| RayLink              | 瑞云科技            | `com.ruiwing.raylink`   | `capture1.plist`、`capture2.plist`、`file.service.plist`、`service.plist`     |   99 / 4 | 远程控制                           |
| TeamViewer           | TeamViewer          | `teamviewer`            | `com.teamviewer.teamviewer.plist`、`teamviewer_service.plist`、`Helper.plist` |   11 / 6 | 远程协助；企业和个人场景均存在     |
| AnyDesk              | AnyDesk             | `com.philandro.anydesk` | `Frontend.plist`、`Helper.plist`、`service.plist`                             |    4 / 2 | 远程控制                           |
| Splashtop            | Splashtop           | `splashtop`             | `com.splashtop.streamer.plist`、`streamer-daemon.plist`                       |    2 / 2 | 远程支持/Streamer                  |
| RustDesk             | RustDesk            | `com.carriez.rustdesk`  | `RustDesk_server.plist`、`RustDesk_service.plist`                             |    1 / 1 | 远程控制                           |
| NoMachine            | NoMachine           | `nomachine`             | `com.nomachine.server.plist`、`localnxserver.plist`                           |    1 / 1 | 远程桌面                           |
| Jump Desktop Connect | Phase Five Systems  | `com.p5sys.jump`        | `com.p5sys.jump.connect.agent.plist`、`connect.service.plist`                 |    1 / 1 | 远程桌面服务端                     |
| O+ Connect           | OPPO                | `com.oplus`             | `com.oplus.dsp.core.plist`、`dsp.rc.plist`、`oplus_remote_service`            |    9 / 7 | 手机互联/远程组件，不是 MDM        |

## 待核实或不应直接使用的关键词

| 原关键词  |    样本 | 结论                                                                              |
|-----------|--------:|-----------------------------------------------------------------------------------|
| `osquery` |   1 / 1 | 是查询与端点遥测引擎，不等于 MDM；样本来自 Fleet，清理 Fleet 应匹配 `fleetdm`     |
| `falcon`  |   2 / 2 | 样本确实是 CrowdStrike，但建议改用更具体的 `crowdstrike`                          |
| `us.zoom` | 14 / 14 | 命中的都是普通 Zoom 客户端、更新器和 `ZoomDaemon`；不应作为监管或 MDM 关键词      |
| `tinyapp` |   0 / 0 | 无日志证据，也无足够公开资料确认其为企业监管代理，暂不启用                        |
| `orthus`  |   0 / 0 | 无日志证据，无法可靠归属到 Amazon 或其他公司，暂不启用                            |
| `teslad`  |   0 / 0 | 已确认属于 Apple 的 `com.apple.devicemanagementclient`，应移到 Apple 系统组件分类 |

## 建议的核心关键词

严格用于 MDM/UEM/RMM 和终端管理时，可从以下集合开始：

```bash
MDM_KEYWORDS="com.apple.mdmclient com.apple.managedclient com.apple.devicemanagementclient addigy ivanti jamf kandji mobileiron mosyle rippling airwatch intune workspaceone jumpcloud com.ws1 com.vmware.deem fleetdm ninjarmm automox tanium corplink"
```

`Company Portal.app` 等包含空格的名称应通过精确文件名列表处理，不能加入当前以空格分隔的 `MDM_KEYWORDS`
。EDR、DLP、资产盘点和企业内部注册代理应维护为独立列表，避免把“发现企业软件”和“确认 Apple MDM 注册”混为一谈。

## 核验资料

- [Apple：声明式设备管理的托管后台任务](https://support.apple.com/en-eg/guide/deployment/dep931381403/web)
- [Microsoft：使用 Company Portal 注册 macOS 到 Intune](https://learn.microsoft.com/en-us/intune/user-help/enrollment/enroll-company-portal-macos)
- [Fleet：macOS 上的 fleetd/Orbit 官方卸载路径](https://fleetdm.com/scripts/macos-uninstall-fleetd)
- [Ivanti：MobileIron 历史及收购关系](https://www.ivanti.com/company/history/mobileiron)
- [Freshservice：Discovery Agent 架构和 macOS LaunchDaemon](https://support.freshservice.com/support/solutions/articles/50000009811-discovery-agent-architecture-and-working)
