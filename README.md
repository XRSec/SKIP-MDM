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

| 接口           | 参数                                        | 说明           |
|--------------|-------------------------------------------|--------------|
| /            |                                           | 执行程序         |
| /cli         |                                           | 验证程序         |
| /getLatestID | ?serial_number=序列号                        | 获取最新版本程序的哈希值 |
| /getLatest   | ?serial_number=序列号                        | 获取最新版本程序     |
| /add         | ?serial_number=序列号&card_id=卡号&password=密码 | 添加授权用户       |
| /add         | ?serial_number=序列号&ps=管理员密码               | 添加授权用户       |
| /del         | ?serial_number=序列号&ps=管理员密码               | 删除授权用户       |
| /auth        | ?serial_number=序列号&ps=管理员密码               | 验证授权用户       |
| /ps          | ?ps=管理员密码                                 | 验证授权用户       |

## 试验功能

```bash
sudo rm /var/db/ConfigurationProfiles/Settings/.cloudConfigHasActivationRecord
sudo rm /var/db/ConfigurationProfiles/Settings/.cloudConfigRecordFound
sudo touch /var/db/ConfigurationProfiles/Settings/.cloudConfigProfileInstalled
sudo touch /var/db/ConfigurationProfiles/Settings/.cloudConfigRecordNotFound
```

## Build

```bash
make build
```

# 卡密生成

- http://www.txttool.com/kami/