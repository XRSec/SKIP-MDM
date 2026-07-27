# Scope of Use, Risk Disclosure, and Legal Notice

## Important Notice

This project is provided solely for computer-security research, technical education, and system maintenance explicitly authorized by the device owner and any relevant management authority. Commercial use is prohibited.

The tools in this project may modify system configuration, device-management status, network configuration, system services, files, and disk-encryption settings. They may cause data loss, system malfunction, reduced security, or failure to boot. Before use, read this notice in full, verify the target device and system volume, and back up all important data.

The statements “for research and education only” and “commercial use prohibited” do not permit unlawful activity and do not replace owner authorization, organizational approval, contractual obligations, or professional legal advice.

## Permitted Uses

This project may be used only for:

- devices lawfully owned by the user;
- devices for which the user has explicit authorization from the owner and any relevant management authority;
- lawful security research, educational experiments, troubleshooting, or system maintenance; and
- activities consistent with applicable law, contractual obligations, software licenses, and organizational security policies.

## Prohibited Uses

This project must not be used to:

- access, control, modify, remove, or dispose of any device, system configuration, or data without authorization;
- circumvent lawful device management, security controls, or auditing imposed by a school, business, government body, or other organization;
- conceal device status, evade auditing, commit fraud, invade privacy, or harm another party’s lawful interests;
- sell commercial services, charge for unlocking or operation, incorporate the project into a commercial offering, or obtain direct or indirect profit; or
- engage in any activity that violates applicable law, contracts, software licenses, device-ownership terms, or organizational policy.

## Technical and Data Risks

This project may modify or remove system configuration, Hosts entries, configuration profiles, services, startup items, vendor files, and disk-encryption settings. This can result in:

- loss of files or data;
- malfunction of the system, network, login, or software;
- inability to boot the system or access the target system volume;
- reduced device security;
- loss of access to school, business, or organizational services;
- loss of compliance with applicable security, regulatory, warranty, or support requirements; and
- changes that cannot be reversed automatically.

Before making any change, confirm that the correct device and system volume are selected, important data is reliably backed up, and a working recovery plan is available. Stop immediately and consult a device administrator or qualified professional if you do not understand an operation.

## Device Ownership and Authorization

The project authors and contributors cannot determine device ownership, management relationships, or whether a user has sufficient authorization. By running or using this project, the user confirms that:

1. the user has lawful authority over the target device or explicit authorization from the owner and relevant management authority;
2. the user understands the purpose, scope, risks, and possible consequences of the intended operations;
3. necessary backups have been completed and a recovery plan is available;
4. the purpose and method of use comply with applicable law, contractual obligations, and organizational policies; and
5. the project will not be used for any unauthorized, unlawful, infringing, or commercial purpose.

Do not run or use this project if any of these statements cannot be confirmed.

## FileVault-Specific Risk

Disabling FileVault reduces protection for data stored on disk and may start a lengthy background decryption process. Unencrypted data may be easier to expose if the device is lost, stolen, or accessed by another person.

Only the device owner or an explicitly authorized administrator may perform this operation. Before proceeding, connect the device to reliable power, back up important data, and confirm that the operation complies with applicable organizational security policies. FileVault must not be disabled offline from Recovery.

## Privacy and Credentials

After the user confirms this notice in the terminal, the client sends the target device serial number to the configured service. The service also records the request IP address and time and may derive an approximate location from the IP address. This information is used only to record confirmation of this legal and risk notice, not to determine device authorization. A user who does not accept this processing must choose `N` at the confirmation prompt and exit.

Except for that confirmation record and a report separately initiated by the user, the project must not collect or upload system inventories, process lists, user-directory contents, diagnostic logs, or unrelated data. When a user separately initiates a report and confirms its upload, the report may include MDM/ADE enrollment status, the presence of ADE/DEP markers, the enrollment-service hostname, Apple domains overridden in `/etc/hosts`, system-level management-component metadata, and running executable names or paths; it excludes Hosts IP addresses and non-Apple entries, the complete enrollment URL, process arguments, and user-directory or temporary-directory paths. Passwords, tokens, private keys, and other credentials must not be written to command-line arguments, environment variables, logs, language packs, or temporary files on disk.

Users must not publish terminal output, reports, or screenshots containing device identifiers, access credentials, personal information, or internal organizational information.

## Limitation of Responsibility

This project is provided “as is,” without any guarantee of fitness for a particular purpose, continuous availability, successful execution, complete or safe results, or reversibility. Users must independently assess the legality, necessity, and risk of their actions and accept responsibility for consequences arising from use, modification, distribution, or misuse.

To the maximum extent permitted by applicable law, the authors and contributors are not liable for direct or indirect loss arising from use, misuse, or inability to use this project, including data loss, system damage, reduced security, business interruption, or third-party claims. This limitation does not apply where exclusion or limitation is prohibited by law.

This notice is not legal advice and cannot eliminate every legal risk. If you cannot determine whether an operation is lawful, whether authorization is sufficient, or whether these terms apply in your jurisdiction, do not use the project; consult a qualified legal professional or device administrator.

## Suggested Interactive Confirmation

Before first use and before modifying the system, the interface should display:

```text
Important Notice

This tool modifies system configuration on this computer or the selected system
volume. Some operations may reduce security and may cause data loss, system
malfunction, or failure to boot.

Continue only if all of the following are true:

1. You lawfully own the device or have explicit authorization from its owner and
   relevant management authority.
2. The operation is for security research, technical education, or lawful device
   maintenance.
3. The operation is not intended to circumvent lawful management imposed by a
   school, business, institution, or other organization.
4. Important data has been backed up and you understand that some changes may be
   irreversible.
5. You accept responsibility for the risks and consequences.

Commercial, unlawful, infringing, and unauthorized use is prohibited.

After confirmation, the client sends the device serial number to the configured
service. The service records the request IP address and time and may derive an
approximate location, solely to record confirmation of this notice.
```

The confirmation prompt must default to rejection:

```text
Do you accept this data transfer and confirm authorization, understanding of the risks, and the prohibition on commercial use? [y/N]
```

Only `y`, `Y`, `yes`, or `YES` may continue. Every other input must exit. Confirmation must not be replaced with a default selection, countdown, or silent execution.

The Web entry point may additionally require the user to read the complete notice and select an authorization checkbox. Web confirmation does not replace the terminal’s `y/N` confirmation covering data processing and risk.
