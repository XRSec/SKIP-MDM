## 解密 shell

```bash
ls *.sh\~
cp cleanShell.sh\~ cleanShell.sh
cp fixShell.sh\~ fixShell.sh
cp mainShell.sh\~ mainShell.sh
```

## 加密 shell

```bash
gzexe *.sh
```

## 接口

| 接口     | 参数                 | 说明                        |
|--------|--------------------|---------------------------|
| /      |                    | 主程序                       |
| /fix   |                    | 修复因屏蔽hosts导致苹果服务无法使用      |
| /clean | ?serial_number=序列号 | 清理 监管程序 通常在恢复模式下使用, 不推荐使用 |
| /add   | ?serial_number=序列号 | 添加授权用户                    |
| /del   | ?serial_number=序列号 | 删除授权用户                    |
| /auth  | ?serial_number=序列号 | 验证授权用户                    |

## 试验功能

```bash
sudo rm /var/db/ConfigurationProfiles/Settings/.cloudConfigHasActivationRecord
sudo rm /var/db/ConfigurationProfiles/Settings/.cloudConfigRecordFound
sudo touch /var/db/ConfigurationProfiles/Settings/.cloudConfigProfileInstalled
sudo touch /var/db/ConfigurationProfiles/Settings/.cloudConfigRecordNotFound
```