# SKIP macOS MDM / DEP

[简体中文](README.zh-CN.md) | [English](README.md)

> [!CAUTION]
> **For lawful, expressly authorized security research, technical learning, and device maintenance only. Commercial use is prohibited.**
>
> This project contains tools that may modify system configuration, device-management state, network settings, system services, files, and disk encryption. These changes may cause data loss, system failure, reduced security, or an unbootable device. Use it only on devices you legally own or are explicitly authorized by both the owner and the relevant management authority to maintain. Never use it on an unauthorized device, to evade lawful school or enterprise management, or for any unlawful, infringing, contractual, or policy violation. Back up important data and accept all risks before use.

Before using this project, read the complete [Scope, Risk Notice, and Legal Terms](LEGAL_NOTICE.en.md). By running or using this project, you confirm that you understand the risks and have the legal right and explicit authorization required to operate on the target device.

## Overview

The current macOS client is written in Shell, while the Web service uses Node.js. The client may modify system state. Verify the target device and system volume, back up important data, and test in a safe environment first.

The source code and companion service are available free of charge for lawful, authorized, non-commercial use.

## Documentation

- [Scope, Risk Notice, and Legal Terms](LEGAL_NOTICE.en.md)
- [Client Technical Notes (Chinese)](TECHNICAL.md)
- [MDM and Enterprise Management Component Notes (Chinese)](MDM_COMPONENTS.md)
