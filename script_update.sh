#!/bin/bash
set -e

REPO="f1owkang/SMSForwarder"

case "$(uname -m)" in
  x86_64)  ARCH=amd64 ;;
  aarch64) ARCH=arm64 ;;
  armv7*|armv6*) ARCH=arm ;;
  *) echo "不支持的架构: $(uname -m)"; exit 1 ;;
esac

if [ ! -x /usr/local/bin/smsforwarder ]; then
  echo "尚未安装，请先执行 script_install_online.sh"
  exit 1
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep -o '"tag_name": *"[^"]*"' | cut -d'"' -f4)
if [ -z "$TAG" ]; then
  echo "无法获取最新版本号，请检查网络后重试"
  exit 1
fi

echo "正在下载 ${ARCH} 最新版本（${TAG}）..."
curl -fsSL "https://github.com/${REPO}/releases/download/${TAG}/smsforwarder-${TAG}-linux-${ARCH}.tar.gz" | tar xz -C "$TMP"

sudo cp /usr/local/bin/smsforwarder /usr/local/bin/smsforwarder.bak
sudo cp "$TMP/stopwords.txt" "$TMP/userwords.txt" /etc/smsforwarder/
sudo install -m 755 "$TMP/smsforwarder" /usr/local/bin/smsforwarder
sudo systemctl restart smsforwarder

echo "升级完成，原配置已保留（回滚备份：/usr/local/bin/smsforwarder.bak）。"
