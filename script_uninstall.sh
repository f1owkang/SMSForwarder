#!/bin/bash
set -e

sudo systemctl stop smsforwarder 2>/dev/null || true
sudo systemctl disable smsforwarder 2>/dev/null || true
sudo rm -f /etc/systemd/system/smsforwarder.service
sudo systemctl daemon-reload
sudo rm -f /usr/local/bin/smsforwarder

read -p "是否同时删除配置与日志（/etc/smsforwarder /var/log/smsforwarder）？(y/n): " choice
if [ "$choice" = "y" ]; then
  sudo rm -rf /etc/smsforwarder /var/log/smsforwarder
  sudo userdel smsforwarder 2>/dev/null || true
  echo "配置与日志已删除，smsforwarder 用户已移除！"
else
  echo "已保留 /etc/smsforwarder 与 /var/log/smsforwarder。"
fi
echo "卸载完成！"
