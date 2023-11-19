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

| 接口           | 参数                                              | 说明           |
|--------------|-------------------------------------------------|--------------|
| /            |                                                 | 执行程序         |
| /getLatestID | ?serial_number=序列号                              | 获取最新版本程序的哈希值 |
| /getLatest   | ?serial_number=序列号                              | 获取最新版本程序     |
| /getLogs     | ?ps=管理员密码&q=字符串                                 | 获取日志         |
| /add         | ?serial_number=序列号&card_id=卡号&password=密码       | 添加授权用户       |
| /add         | ?serial_number=序列号&ps=管理员密码                     | 添加授权用户       |
| /getCard     | ?card_id=card_id&ps=管理员密码                       | 获取卡密信息       |
| /cardUpdate  | ?card_id=card_id&password=new_password&ps=管理员密码 | 更新卡密清空绑定     |
| /cardDel     | ?card_id=card_id&ps=管理员密码                       | 删除卡密绑定       |
| /del         | ?serial_number=序列号&ps=管理员密码                     | 删除授权用户       |
| /auth        | ?serial_number=序列号&ps=管理员密码                     | 验证授权用户       |
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

# CDN

```ini
/add;/auth;/del;/cli;/getLatestID;/getLatest;/getLogs;/getCard;/cardDel;/cardUpdate
/favicon.ico;/apple-touch-icon*.png;/*.js
```