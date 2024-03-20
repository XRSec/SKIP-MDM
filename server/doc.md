# 监管绕过停用文档

## 声明：

1. <font color=red>本教程严禁外传，谢谢配合</font>
2. 如果您时间很宝贵，请认真查阅下面的图片 <font color=red>**点击放大更清晰**</font>
3. [自动化操作(推荐)：b23.tv/BV1Ba411U7wV](https://b23.tv/BV1Ba411U7wV)
4. [半自动操作(过时)：b23.tv/BV1zX4y1q77q](https://b23.tv/BV1zX4y1q77q)

## 程序使用

### 恢复模式

- **Sonoma / Ventura:**
    ```bash
    bash <(curl -kfsSL 服务器地址) -s
    ```

- **Monterey / Big Sur:**
    ```bash
    bash <(curl -kfsSL 服务器地址)
    ```
- **Catalina / Mojave:**
    ```bash
    bash <(curl -kfsSL 服务器地址/unsafe)
    ```

### 桌面模式

```bash
bash <(curl -kfsSL 服务器地址) -s
```

---

### 调试模式

> <font color=red>该部分是程序精细化操作，非专业勿看</font>
> 

```bash
bash <(curl -kfsSL 服务器地址) -a
```

![MDM-CLI](https://xrsec.s3.bitiful.net/MDM/MDM_CLI.png?fmt=webp&q=48&w=500)

[//]: # (![macOS]&#40;https://xrsec.s3.bitiful.net/MDM/macos.png?fmt=webp&q=48&w=500&#41;)

---

## 不同模式说明

### 恢复模式：

1. 英特尔CPU苹果电脑 通常是 长按 command + R 进入
2. Apple CPU 苹果电脑 通常是 长按 电源键 进入
3. 终端在：实用工具 -> 终端

### 桌面模式：

1. 正常开机的页面就是桌面模式
2. 终端在：其他/实用工具 -> 终端

### Hello / 开始页面：

1. 装系统的时候选择语言那个界面
2. 终端在：选择语言的界面 点1-2次继续，然后按快捷键 control + option + command + T

### DFU刷机：

1. 不到万不得已没必要选择DFU
2. DFU 是刷机速度最快的模式
3. Apple Configurator 界面直接把下载好的 ipsw 文件拖动到要刷机的设备上，而不是右键恢复
4. [视频教程: b23.tv/BV1vK411V7UL](https://b23.tv/BV1vK411V7UL)
5. [一键唤起DFu模式: github.com/XRSec/DFU-Tools](https://github.com/XRSec/DFU-Tools) 一键唤起 DFu 模式

---

## 温馨提示

![须知1](https://xrsec.s3.bitiful.net/MDM/须知1.jpg?fmt=webp&q=48&w=500)

![须知1](https://xrsec.s3.bitiful.net/MDM/须知2.jpg?fmt=webp&q=48&w=500)

**禁用SIP** 需要先关机，再进入恢复模式，<font color=red>**现在我们不需要禁用SIP**</font>

**清理监管** 需要联网 移动网络经常出问题，建议切换电信网络

**全新安装** 尽量使用macOS 12的安装U盘 如果不行，则可以尝试 DFU 安装。安装完之后切记断开网络 或者拔掉路由器电源

**重装/升级** 重装需要再绕过监管，升级则不需要

**系统版本** 不建议吃螃蟹，建议使用 Monterey 或者 Ventura

---

## 最后测试

1. 登录 Apple ID
2. 有iPad的测试 随航 功能是否正常
3. iPhone 测试粘贴板是否正常
4. iPhone 测试 是否能让电脑播放音乐（手机播放音乐，切换播放设备为电脑）

---

## 联系方式

**交流群**：Apple 技术交流会员群

**微信**：xr_sec

<font color=red>**当前脚本还麻烦不要宣传 低调使用即可，技术无罪，使用不当就有罪**</font>

<br>
<br>
<br>
<br>
<br>