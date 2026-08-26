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

if [ ! -x /usr/local/bin/smsforwarder ]; then
  echo "尚未安装，请先执行 script_install_online.sh"
  exit 1
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

TAG=$(curl -fsSL -o /dev/null -w '%{url_effective}' -L "https://github.com/${REPO}/releases/latest" | sed 's#.*/##')
if [ -z "$TAG" ]; then
  echo "无法获取最新版本号，请检查网络后重试"
  exit 1
fi

echo "正在下载 ${ARCH} 最新版本（${TAG}）..."
curl -fsSL "https://github.com/${REPO}/releases/download/${TAG}/smsforwarder-${TAG}-linux-${ARCH}.tar.gz" | tar xz -C "$TMP"

# 先校验下载的二进制可运行（-v 不依赖任何环境）
if ! "$TMP/smsforwarder" -v >/dev/null 2>&1; then
  echo "下载的二进制无法运行，已中止升级"
  exit 1
fi

sudo cp /usr/local/bin/smsforwarder /usr/local/bin/smsforwarder.bak
sudo cp "$TMP/stopwords.txt" "$TMP/userwords.txt" /etc/smsforwarder/
sudo install -m 755 "$TMP/smsforwarder" /usr/local/bin/smsforwarder

# 替换后用新二进制校验现有配置，失败自动回滚
if ! sudo /usr/local/bin/smsforwarder -check >/dev/null 2>&1; then
  echo "新版本配置自检失败，回滚到上一个版本"
  sudo install -m 755 /usr/local/bin/smsforwarder.bak /usr/local/bin/smsforwarder
  sudo systemctl restart smsforwarder
  exit 1
fi

sudo id -u smsforwarder >/dev/null 2>&1 || sudo useradd --system --no-create-home --shell /usr/sbin/nologin smsforwarder
sudo chown -R smsforwarder:smsforwarder /var/log/smsforwarder
sudo systemctl restart smsforwarder

echo "升级完成，原配置已保留（回滚备份：/usr/local/bin/smsforwarder.bak）。"
