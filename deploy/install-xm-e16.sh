#!/bin/bash
# ============================================================
# DCIM 一键安装脚本 — XM-E16 数据采集网关（Ubuntu 18.04 arm64）
# 用法: sudo bash install-xm-e16.sh [安装包目录]
# 前提: 交付包已解压（含 backend/dcim-api-arm64、frontend/dist、edge/dcim-edge-arm64、edge/edge.example.json）
# 幂等: 重复执行会覆盖程序与配置模板，但不覆盖已存在的 /opt/dcim/edge.json 与数据库
# ============================================================
set -e

PKG="${1:-$(cd "$(dirname "$0")" && pwd)}"
DCIM_HOME=/opt/dcim
EDGE_USER=dcim
LOG=/var/log/dcim-install.log
exec > >(tee -a "$LOG") 2>&1

echo "=== [1/9] 环境检查 ==="
ARCH=$(dpkg --print-architecture 2>/dev/null || uname -m)
echo "架构: $ARCH  内核: $(uname -r)  系统: $(lsb_release -ds 2>/dev/null || cat /etc/os-release | grep PRETTY | cut -d= -f2)"
if [ ! -f "$PKG/backend/dcim-api-arm64" ]; then
  echo "错误: 找不到 $PKG/backend/dcim-api-arm64（请先解压交付包并把本脚本放到包内）"; exit 1
fi

echo "=== [2/9] 安全加固提醒（必须人工确认已完成） ==="
if passwd -S root 2>/dev/null | grep -q "123456" || [ "$(cat /etc/machine-id 2>/dev/null | head -c 4)" = "" ]; then :; fi
cat <<'SEC'
  [!] XM-E16 出厂 root 默认口令为公开信息（手册 3.3 节），请立即执行:
      passwd root            # 修改 root 口令
      useradd -m dcim && passwd dcim   # 本脚本会创建 dcim 运行账号
      sed -i 's/#\?PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config  # 建议禁 SSH 密码
  [!] Ubuntu 18.04 已 EOL: apt 源需切 old-releases.ubuntu.com，且设备必须部署在内网，
      管理口不要暴露公网。
SEC

echo "=== [3/9] 创建运行账号与目录 ==="
id -u $EDGE_USER >/dev/null 2>&1 || useradd -r -m -s /bin/bash $EDGE_USER
mkdir -p $DCIM_HOME/bin $DCIM_HOME/web $DCIM_HOME/logs /var/lib/dcim-edge
chown -R $EDGE_USER:$EDGE_USER /var/lib/dcim-edge $DCIM_HOME/logs

echo "=== [4/9] 安装基础软件 (postgresql/nginx) ==="
if ! command -v psql >/dev/null; then
  # Ubuntu 18.04 EOL 源修复
  if grep -rq "archive.ubuntu.com" /etc/apt/sources.list 2>/dev/null; then
    sed -i 's|http://archive.ubuntu.com|http://old-releases.ubuntu.com|g; s|http://security.ubuntu.com|http://old-releases.ubuntu.com|g' /etc/apt/sources.list
  fi
  apt-get update -y
  DEBIAN_FRONTEND=noninteractive apt-get install -y postgresql nginx
fi
systemctl enable postgresql nginx

echo "=== [5/9] 初始化数据库 ==="
DB_USER=dcim; DB_NAME=dcim; DB_PASS_FILE=$DCIM_HOME/.db_pass
if [ ! -f "$DB_PASS_FILE" ]; then
  DB_PASS=$(head -c 24 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 24)
  echo "$DB_PASS" > "$DB_PASS_FILE"; chmod 600 "$DB_PASS_FILE"
  sudo -u postgres psql -tc "SELECT 1 FROM pg_roles WHERE rolname='$DB_USER'" | grep -q 1 || \
    sudo -u postgres psql -c "CREATE USER $DB_USER WITH PASSWORD '$DB_PASS';"
  sudo -u postgres psql -tc "SELECT 1 FROM pg_database WHERE datname='$DB_NAME'" | grep -q 1 || \
    sudo -u postgres psql -c "CREATE DATABASE $DB_NAME OWNER $DB_USER;"
  echo "  数据库已创建（密码存于 $DB_PASS_FILE，chmod 600）"
else
  DB_PASS=$(cat "$DB_PASS_FILE")
  echo "  数据库已存在，跳过创建"
fi

echo "=== [6/9] 挂载 TF/SATA 数据盘（可选，保护 eMMC） ==="
DATA_DEV=""
for d in /dev/mmcblk1p1 /dev/sda1; do
  if [ -b "$d" ]; then DATA_DEV=$d; break; fi
done
if [ -n "$DATA_DEV" ] && ! mountpoint -q /mnt/data; then
  mkdir -p /mnt/data
  blkid "$DATA_DEV" | grep -q ext4 || mkfs.ext4 -F "$DATA_DEV"
  mount "$DATA_DEV" /mnt/data
  grep -q "/mnt/data" /etc/fstab || echo "$DATA_DEV /mnt/data ext4 defaults,nofail 0 2" >> /etc/fstab
  echo "  数据盘 $DATA_DEV 已挂载到 /mnt/data（建议后续把 PostgreSQL 数据目录迁移至此）"
else
  echo "  未检测到 TF/SATA 数据盘或已挂载，跳过"
fi

echo "=== [7/9] 部署程序 ==="
install -m 755 "$PKG/backend/dcim-api-arm64" $DCIM_HOME/bin/dcim-api
install -m 755 "$PKG/edge/dcim-edge-arm64" $DCIM_HOME/bin/dcim-edge
rm -rf $DCIM_HOME/web.new && cp -r "$PKG/frontend/dist" $DCIM_HOME/web.new && rm -rf $DCIM_HOME/web.bak
[ -d $DCIM_HOME/web ] && mv $DCIM_HOME/web $DCIM_HOME/web.bak
mv $DCIM_HOME/web.new $DCIM_HOME/web

# 生产配置（首次生成，之后不覆盖）
CFG=$DCIM_HOME/configs/production.yaml
mkdir -p $DCIM_HOME/configs
if [ ! -f "$CFG" ]; then
  cat > "$CFG" <<EOF
server:
  host: 127.0.0.1
  port: 8095
database:
  host: 127.0.0.1
  port: 5432
  user: $DB_USER
  password: $DB_PASS
  dbname: $DB_NAME
  sslmode: disable
jwt:
  secret: $(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 48)
  expire_hours: 72
simulate:
  enabled: false
wechat:
  appid: ""
  app_secret: ""
EOF
  chmod 600 "$CFG"
  echo "  已生成 $CFG（微信小程序 appid/secret 请按需填写）"
fi

# edge 配置（首次生成，之后不覆盖——secret 由平台"网关管理"页注册后回填）
EDGE_CFG=$DCIM_HOME/edge.json
if [ ! -f "$EDGE_CFG" ]; then
  cat > "$EDGE_CFG" <<EOF
{
  "platform_url": "http://127.0.0.1:8095/api/v1",
  "code": "",
  "secret": "",
  "cache_dir": "/var/lib/dcim-edge",
  "simulate": false,
  "hb_interval": 15,
  "pull_interval": 3,
  "sync_interval": 300
}
EOF
  chmod 600 "$EDGE_CFG"
  echo "  已生成 $EDGE_CFG —— 请在 Web「网关管理」注册网关后，把 code/secret 填入"
  echo "  （code 可用设备 Serial: cat /proc/cpuinfo | grep Serial）"
fi

echo "=== [8/9] systemd 服务 ==="
cat > /etc/systemd/system/dcim-api.service <<EOF
[Unit]
Description=DCIM Platform API
After=network.target postgresql.service

[Service]
Type=simple
User=$EDGE_USER
WorkingDirectory=$DCIM_HOME
ExecStart=$DCIM_HOME/bin/dcim-api -c $CFG
Restart=always
RestartSec=3
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
cat > /etc/systemd/system/dcim-edge.service <<EOF
[Unit]
Description=DCIM Edge Collector (XM-E16)
After=network.target dcim-api.service
Wants=dcim-api.service

[Service]
Type=simple
User=root
WorkingDirectory=$DCIM_HOME
ExecStart=$DCIM_HOME/bin/dcim-edge $EDGE_CFG
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
# nginx 站点
cat > /etc/nginx/sites-available/dcim <<'EOF'
server {
    listen 3000;
    root /opt/dcim/web;
    index index.html;
    location /api/ {
        proxy_pass http://127.0.0.1:8095;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
    location /ws/ {
        proxy_pass http://127.0.0.1:8095;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 3600s;
    }
    location / { try_files $uri $uri/ /index.html; }
}
EOF
ln -sf /etc/nginx/sites-available/dcim /etc/nginx/sites-enabled/dcim
rm -f /etc/nginx/sites-enabled/default
systemctl daemon-reload
systemctl enable dcim-api dcim-edge
systemctl restart dcim-api
systemctl restart nginx
systemctl restart dcim-edge || echo "  dcim-edge 启动失败（可能未配置 code/secret，配置后 systemctl restart dcim-edge）"

echo "=== [9/9] HDMI kiosk 大屏（可选） ==="
VIEW=/opt/scripts/boot/view.sh
if [ -f "$VIEW" ]; then
  cp "$VIEW" "$VIEW.bak.$(date +%s)"
  sed -i 's|--app="http://127.0.0.1:80"|--app="http://127.0.0.1:3000/#/bigscreen"|' "$VIEW"
  echo "  view.sh 已指向 DCIM 大屏（备份保留）"
else
  echo "  未找到 $VIEW（非 kiosk 出厂镜像），跳过"
fi

sleep 2
echo ""
echo "=== 自检 ==="
curl -sf -m 5 http://127.0.0.1:8095/api/v1/health && echo " <- API OK" || echo "API 未就绪: journalctl -u dcim-api -n 50"
curl -sf -m 5 -o /dev/null -w "前端: HTTP %{http_code}\n" http://127.0.0.1:3000/
IP=$(ip -4 addr show eth2 2>/dev/null | grep -oP '(?<=inet\s)\d+(\.\d+){3}' || hostname -I | awk '{print $1}')
echo ""
echo "部署完成。访问: http://$IP:3000"
echo "下一步: 1) Web 登录(admin) → 网关管理 → 注册网关取 code/secret"
echo "        2) 填入 /opt/dcim/edge.json 后 systemctl restart dcim-edge"
echo "        3) 测点管理 → 批量导入（CSV 模板含 XM-E16 串口/DI 填法）"
