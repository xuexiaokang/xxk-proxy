# xxk-proxy OpenWrt 快速部署指南

## 你需要上传的 4 个文件

| # | 本地路径 | 上传到 OpenWrt | 用途 |
|---|---------|---------------|------|
| 1 | `output/xxk-proxy-mipsle-upx` | `/usr/bin/xxk-proxy` | 代理二进制 (1.9MB) |
| 2 | `package/xxk-proxy/files/xxk-proxy.init` | `/etc/init.d/xxk-proxy` | 服务启动脚本 |
| 3 | `package/xxk-proxy/files/xxk-proxy.config` | `/etc/config/xxk-proxy` | UCI 配置 |
| 4 | `package/xxk-proxy/files/xxk-proxy.hotplug` | `/etc/hotplug.d/iface/60-xxk-proxy` | 网口热插拔 |

## 部署步骤

### 第一步：从本地电脑上传文件

在**本地电脑**（PowerShell）上执行：

```bash
scp E:\QClaw\xxk-proxy\openwrt-package\output\xxk-proxy-mipsle-upx \
    E:\QClaw\xxk-proxy\openwrt-package\package\xxk-proxy\files\xxk-proxy.init \
    E:\QClaw\xxk-proxy\openwrt-package\package\xxk-proxy\files\xxk-proxy.config \
    E:\QClaw\xxk-proxy\openwrt-package\package\xxk-proxy\files\xxk-proxy.hotplug \
    root@你的OpenWrtIP:/tmp/
```

### 第二步：在 OpenWrt 上安装

```bash
# 安装文件到正确位置
cp /tmp/xxk-proxy-mipsle-upx /usr/bin/xxk-proxy
cp /tmp/xxk-proxy.init /etc/init.d/xxk-proxy
cp /tmp/xxk-proxy.config /etc/config/xxk-proxy
cp /tmp/xxk-proxy.hotplug /etc/hotplug.d/iface/60-xxk-proxy

# 设置可执行权限
chmod +x /usr/bin/xxk-proxy /etc/init.d/xxk-proxy /etc/hotplug.d/iface/60-xxk-proxy

# 创建日志目录
mkdir -p /var/log/proxy
```

### 第三步：启动服务

```bash
# 启动
/etc/init.d/xxk-proxy start

# 开机自启
/etc/init.d/xxk-proxy enable
```

### 第四步：验证

```bash
# 检查进程
ps | grep xxk-proxy

# 检查端口
netstat -tln | grep 5858

# 检查日志
tail -5 /var/log/proxy/proxy.log
```

## 常见问题

### Q: 上传后 start 报 "disabled, skipping start"
A: 配置文件里 `enabled` 不是 `1`。检查 `/etc/config/xxk-proxy`：
```bash
cat /etc/config/xxk-proxy
# 确保有: option enabled '1'
```

### Q: 报 "file: not found" 或 "Permission denied"
A: 架构不匹配。确认 `uname -m` 输出和二进制对应：
| uname -m | 用哪个二进制 |
|----------|-------------|
| mips | xxk-proxy-mipsle-upx |
| armv7l | xxk-proxy-arm-upx |
| x86_64 | xxk-proxy-x86_64-upx |

### Q: 想改端口
```bash
uci set xxk-proxy.global.port=9090
uci commit xxk-proxy
/etc/init.d/xxk-proxy restart
```