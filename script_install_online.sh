#!/bin/bash
set -e

REPO="f1owkang/SMSForwarder"

case "$(uname -m)" in
  x86_64)  ARCH=amd64 ;;
  aarch64) ARCH=arm64 ;;
  armv7*|armhf) ARCH=arm ;;
  armv6*|armv5te) ARCH=armv6 ;;
  *) echo "不支持的架构: $(uname -m)"; exit 1 ;;
esac

URL_BASE="https://github.com/${REPO}/releases/download"
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep -o '"tag_name": *"[^"]*"' | cut -d'"' -f4)
if [ -z "$TAG" ]; then
  echo "无法获取最新版本号，请检查网络后重试"
  exit 1
fi
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "正在下载 ${ARCH} 版本（${TAG}）..."
curl -fsSL "${URL_BASE}/${TAG}/smsforwarder-${TAG}-linux-${ARCH}.tar.gz" | tar xz -C "$TMP"

sudo install -m 755 "$TMP/smsforwarder" /usr/local/bin/smsforwarder
sudo mkdir -p /etc/smsforwarder /var/log/smsforwarder
sudo cp "$TMP/stopwords.txt" "$TMP/userwords.txt" /etc/smsforwarder/
if [ ! -f /etc/smsforwarder/config.yml ]; then
  sudo cp "$TMP/config.example.yml" /etc/smsforwarder/config.yml
fi
sudo cp "$TMP/smsforwarder.service" /etc/systemd/system/
sudo systemctl daemon-reload

if [ -f /home/forward/config.json ]; then
  echo "检测到旧版 Python 版配置 /home/forward/config.json，请参照新示例改写 /etc/smsforwarder/config.yml"
fi

echo "安装完成！接下来："
echo "  1. 编辑 /etc/smsforwarder/config.yml"
echo "  2. 运行 smsforwarder -check 校验"
echo "  3. sudo systemctl enable --now smsforwarder"
