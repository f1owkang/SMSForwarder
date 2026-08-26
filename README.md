# SMS Forwarder 短信转发服务

Go 实现的 ModemManager 短信转发器：监听 D-Bus 事件，自动提取验证码/关键词，推送到 PushPlus、Server酱、Bark、Telegram、自定义 Webhook，或回落为短信转发。单二进制零依赖，比 Python 版更快、更省内存。

在随身WIFI（帝旭410，Debian 11）上完成测试。无需 cron，D-Bus 事件驱动，消息延迟更低。

## 特性

- 单一静态二进制，无 Python/jieba 依赖
- 验证码优先提取（`验证码【xxxxxx】`），其余短信用 gse 中文分词提取关键词
- 每个接收者一条有序渠道链：PushPlus → Server酱 → Bark → Telegram → Webhook → 短信回落，成功即停
- YAML 配置 + 启动校验 + `-check` 自检 + `-test` 测试推送
- 多调制解调器自动发现，ModemManager 重启自动重连
- 结构化 JSONL 日志（字段与旧版兼容），每日心跳日志证明存活

## 使用方法

```
curl -sSL https://raw.githubusercontent.com/f1owkang/SMSForwarder/main/script_install_online.sh | bash
```

安装后：

1. 编辑 `/etc/smsforwarder/config.yml`（完整示例见仓库 `config.example.yml`）
2. `smsforwarder -check` 校验配置
3. 启动服务：

```
sudo systemctl enable --now smsforwarder
```

验证推送是否通畅：

```
sudo /usr/local/bin/smsforwarder -test
```

## 渠道配置速查

| type | 必填字段 | 说明 |
|---|---|---|
| pushplus | token | 官方推送 |
| serverchan | send_key | Server酱 |
| bark | device_key | 可选 `server` 自建 |
| telegram | bot_token, chat_id | 可选 `proxy`（如 socks5://127.0.0.1:1080）|
| webhook | url | POST JSON `{number,text,keyword,timestamp}`，HTTP 2xx 即成功 |
| sms | phone | D-Bus 直接发送，作回落 |

## 手动构建

需要 Go 1.26+：

```
make test        # 运行测试
make package     # 三架构 tar.gz 到 dist/
```

本地开发机交叉编译示例（Windows pwsh 同理设置环境变量）：

```
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o smsforwarder ./cmd/smsforwarder
```

## CLI

```
smsforwarder [-c 配置路径]    # 守护运行
smsforwarder -check          # 校验配置后退出
smsforwarder -test           # 向所有接收者发测试消息
smsforwarder -v              # 版本号
```

## 从 Python 版迁移

- 配置从 `/home/forward/config.json` 改为 `/etc/smsforwarder/config.yml`，需按新格式重写一次
- 日志仍为 JSONL 且字段不变（`number/text/timestamp/forwarded_to/status`）
- 升级直接运行 `script_update.sh`，卸载运行 `script_uninstall.sh`

## 参考项目

- https://github.com/TeddyNight/sms_forwarder_mmcli
- https://github.com/lkiuyu/DbusSmsForward
- https://github.com/cyfrit/sms_forwarder
