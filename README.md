# SMS Forwarder

[![Release](https://img.shields.io/github/v/release/f1owkang/SMSForwarder)](https://github.com/f1owkang/SMSForwarder/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-amd64%20%7C%20arm64%20%7C%20armv7-blue)](#安装)

转发随身 WiFi 等 Linux 设备上 ModemManager 收到的短信：监听 D-Bus 事件，自动提取验证码或关键词，推送到 PushPlus、Server酱、Bark、Telegram、自定义 Webhook，失败可回落为短信转发。单一静态二进制，没有 Python 和 jieba 依赖，在帝旭410（Debian 11）上完成过整套验证。

比起 cron 轮询 mmcli 的方案，D-Bus 事件驱动没有轮询间隔，短信到达即处理。

## 工作原理

```
ModemManager ──D-Bus Added信号──> 监听器 ──> 提取验证码/关键词 ──> 按接收者渠道链依次推送
                                                                    │ 成功即停
                                                    失败继续下一渠道 <┘
```

- 启动时枚举所有调制解调器（不限定 Modem/0），订阅每个的 `Added` 信号
- ModemManager 重启后自动重新订阅
- 每日输出一条心跳日志，证明进程与 D-Bus 连接健在

## 安装

```bash
curl -sSL https://raw.githubusercontent.com/f1owkang/SMSForwarder/main/script_install_online.sh | bash
```

脚本自动识别架构（amd64 / arm64 / armv7），下载对应产物并安装：

- 二进制：`/usr/local/bin/smsforwarder`
- 配置与词库：`/etc/smsforwarder/`
- 日志目录：`/var/log/smsforwarder/`

然后三步：

```bash
vim /etc/smsforwarder/config.yml        # 填入你的推送 token
smsforwarder -check                     # 校验配置
sudo systemctl enable --now smsforwarder
```

发一条测试消息确认各渠道通畅（注意：`-test` 会真实发送，sms 渠道会产生短信资费）：

```bash
sudo /usr/local/bin/smsforwarder -test
```

## 配置

`config.yml` 完整结构如下，带注释版见仓库 [config.example.yml](config.example.yml)：

```yaml
log_file: /var/log/smsforwarder/sms_log.jsonl   # 留空则只写 stdout/journal
auto_delete: false                               # 转发成功后删除设备上的短信
heartbeat: 24h                                   # 心跳日志间隔，0 表示关闭

recipients:
  - name: main1
    channels:                  # 按顺序尝试，成功即停；sms 通常放最后作回落
      - type: pushplus
        token: "XXXXXXXXXXXXXXXXXXXXXXXXXX"
      - type: sms
        phone: "181XXXXXXXX"
```

可以配多个接收者，互不影响。渠道字段速查：

| type | 必填字段 | 可选字段 | 说明 |
|---|---|---|---|
| pushplus | token | | |
| serverchan | send_key | | Server酱 |
| bark | device_key | server | `server` 缺省为官方 `https://api.day.app` |
| telegram | bot_token, chat_id | proxy | 国内直连不通时填代理，如 `socks5://127.0.0.1:1080` |
| webhook | url | | POST JSON `{number,text,keyword,timestamp}`，返回 HTTP 2xx 即成功 |
| sms | phone | | 通过 D-Bus 直接发送，作回落 |

配置文件按以下顺序查找：`-c` 指定的路径 → `/etc/smsforwarder/config.yml` → 二进制同目录 `./config.yml`。启动时全量校验，缺字段会在报错里直接指出是哪个接收者的哪个渠道。

## 消息格式

PushPlus 保持与旧版一致：标题为发件号码，正文首行为提取结果。

其余渠道：标题为关键词（空则用号码），正文为「号码 / 正文 / 时间」三行。

## 提取规则

优先匹配验证码：「验证码 / 校验码 / 动态码」后面紧跟的 4~6 位数字，输出形如 `验证码【884825】`。

未命中时走 gse 中文分词做 TF-IDF 提取，过滤停用词和纯数字后取前 4 个词。词库复用原项目格式，放在 `/etc/smsforwarder/stopwords.txt` 和 `userwords.txt`，可自行增删。

## 日常运维

```bash
systemctl status smsforwarder          # 服务状态
journalctl -u smsforwarder -f          # 实时日志
tail -f /var/log/smsforwarder/sms_log.jsonl   # 结构化转发记录
sudo systemctl restart smsforwarder    # 改完配置后重启生效
```

升级和卸载：

```bash
sudo ./script_update.sh       # 下载最新版覆盖二进制，配置保留
sudo ./script_uninstall.sh    # 停服务、删文件，可选清理配置
```

## 从 Python 版迁移

- 配置需要从 `/home/forward/config.json` 按 `config.example.yml` 重写一份，旧文件不会被读取
- 日志仍是 JSONL，`number/text/timestamp/forwarded_to/status` 字段不变，已有的分析脚本不受影响
- 安装脚本检测到旧配置时会提示迁移

## 本地构建

Go 1.26+：

```bash
make test        # 测试
make build-all   # 三架构编译到 dist/
make package     # 打包 tar.gz（含配置示例、词库、service 文件）
```

交叉编译示例（其他平台同理设置环境变量）：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o smsforwarder ./cmd/smsforwarder
```

推送 `v*` 格式的 tag 会触发 GitHub Actions 自动构建并发布 Release。

## 参考项目

- https://github.com/TeddyNight/sms_forwarder_mmcli
- https://github.com/lkiuyu/DbusSmsForward
- https://github.com/cyfrit/sms_forwarder
