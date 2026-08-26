#!/bin/bash
set -e

BIN=""
for cand in ./smsforwarder ./dist/smsforwarder-linux-arm64/smsforwarder ./dist/smsforwarder-linux-amd64/smsforwarder ./dist/smsforwarder-linux-arm/smsforwarder ./dist/smsforwarder-linux-armv6/smsforwarder; do
  if [ -f "$cand" ]; then BIN="$cand"; break; fi
done
if [ -z "$BIN" ]; then
  echo "未找到预编译二进制，请先在本机执行 make build-all 或放置 ./smsforwarder"
  exit 1
fi

sudo install -m 755 "$BIN" /usr/local/bin/smsforwarder
sudo mkdir -p /etc/smsforwarder /var/log/smsforwarder
sudo cp stopwords.txt userwords.txt /etc/smsforwarder/
[ -f /etc/smsforwarder/config.yml ] || sudo cp config.example.yml /etc/smsforwarder/config.yml
sudo cp smsforwarder.service /etc/systemd/system/
sudo systemctl daemon-reload

echo "本地安装完成！请编辑 /etc/smsforwarder/config.yml 后："
echo "  smsforwarder -check && sudo systemctl enable --now smsforwarder"
