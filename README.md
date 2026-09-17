# xxk-proxy

轻量级 HTTP/HTTPS 代理服务器，专为 OpenWrt 路由器优化。

[![Go](https://img.shields.io/badge/Go-1.21-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![OpenWrt](https://img.shields.io/badge/OpenWrt-21.02+-green.svg)](https://openwrt.org/)

## 特性

- **HTTP/HTTPS 代理** — 标准 HTTP 代理 + HTTPS CONNECT 隧道
- **YAML 配置** — 配置文件 + 命令行参数，双模式支持
- **日志轮转** — 按文件大小、备份数、保留天数自动轮转
- **热重载** — 配置修改后自动重新加载（默认 30s 间隔）
- **优雅关闭** — SIGINT/SIGTERM 信号处理，连接不中断
- **零外部依赖** — Go 静态编译，`CGO_ENABLED=0`，无动态链接
- **OpenWrt 原生集成** — UCI 配置、init.d 服务、hotplug 事件
- **多架构支持** — MIPS/ARM/x86_64 交叉编译，UPX 压缩至 ~2MB

## 快速开始

### 一键部署（推荐）

在 OpenWrt 设备上执行：

```bash
# 下载预编译二进制（根据你的架构选择）
# MIPS:   wget https://github.com/xuexiaokang/xxk-proxy/releases/download/v1.0.0/xxk-proxy-mips
# ARMv7:  wget https://github.com/xuexiaokang/xxk-proxy/releases/download/v1.0.0/xxk-proxy-arm
# x86_64: wget https://github.com/xuexiaokang/xxk-proxy/releases/download/v1.0.0/xxk-proxy-x86_64

# 安装
chmod +x xxk-proxy-* && mv xxk-proxy-* /usr/bin/xxk-proxy

# 创建配置
uci add_config xxk-proxy global
uci set xxk-proxy.global.enabled=1
uci set xxk-proxy.global.port=5858
uci commit xxk-proxy

# 启动 + 开机自启
/etc/init.d/xxk-proxy start
/etc/init.d/xxk-proxy enable
```

### 手动安装

```bash
# 1. 传二进制
scp output/xxk-proxy-mips root@openwrt:/usr/bin/xxk-proxy

# 2. 传 init 脚本和配置
scp package/xxk-proxy/files/xxk-proxy.init root@openwrt:/etc/init.d/xxk-proxy
scp package/xxk-proxy/files/xxk-proxy.config root@openwrt:/etc/config/xxk-proxy
scp package/xxk-proxy/files/xxk-proxy.hotplug root@openwrt:/etc/hotplug.d/iface/60-xxk-proxy

# 3. 启动
chmod +x /usr/bin/xxk-proxy /etc/init.d/xxk-proxy
/etc/init.d/xxk-proxy start
```

## 配置

### UCI 配置（推荐）

```bash
# 编辑配置
uci edit xxk-proxy
# 或直接编辑 /etc/config/xxk-proxy
```

```ini
config xxk-proxy 'global'
    option enabled '1'          # 是否启用
    option port '5858'          # 监听端口
    option log_enabled '1'      # 是否启用日志
    option log_file '/var/log/proxy/proxy.log'  # 日志路径
    option log_maxsize '10'     # 单文件大小限制 (MB)
    option log_maxbackups '3'   # 保留备份数
    option log_maxage '28'      # 保留天数
    option log_compress '1'     # 是否压缩旧日志
    option reload '30'          # 热重载间隔 (秒)
```

### 命令行参数

```bash
xxk-proxy -port=5858 -log=true -logfile=/var/log/proxy/proxy.log
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-port` | `5858` | 监听端口 |
| `-log` | `true` | 启用日志 |
| `-logfile` | `(空)` | 日志路径，空则输出到 stdout |
| `-log.maxsize` | `10` | 单文件大小限制 (MB) |
| `-log.maxbackups` | `3` | 保留备份数 |
| `-log.maxage` | `28` | 保留天数 |
| `-log.compress` | `true` | 压缩旧日志 |
| `-config` | `(空)` | YAML 配置文件路径 |
| `-reload` | `30` | 热重载间隔 (秒) |

### YAML 配置文件

```yaml
server:
  port: 5858

log:
  enabled: true
  file: /var/log/proxy/proxy.log
  maxsize: 10
  maxbackups: 3
  maxage: 28
  compress: true

reload: 30
```

## 使用

```bash
# HTTP 代理
curl -x http://openwrt:5858 http://example.com

# HTTPS 代理（CONNECT 隧道）
curl -x http://openwrt:5858 https://example.com
```

## 服务管理

```bash
/etc/init.d/xxk-proxy start     # 启动
/etc/init.d/xxk-proxy stop      # 停止
/etc/init.d/xxk-proxy restart   # 重启
/etc/init.d/xxk-proxy enable    # 开机自启
/etc/init.d/xxk-proxy disable   # 取消自启
```

## 构建

### 交叉编译

```bash
# 构建特定架构
./build.sh mips     # MIPS 大端（最常用）
./build.sh arm      # ARMv7
./build.sh x86_64   # x86_64

# 自定义参数
cd package/xxk-proxy/src
CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat \
    go build -ldflags="-s -w" -o xxk-proxy .
```

### Docker 构建

```bash
docker build -t xxk-proxy-openwrt -f openwrt-package/Dockerfile .
docker run --rm xxk-proxy-openwrt cat /usr/bin/xxk-proxy > output/xxk-proxy-docker
```

## 体积优化

Go 静态编译的二进制默认 6-8MB，对嵌入式设备不友好。使用 **UPX** 压缩后体积减少约 75%：

| 架构 | 原始大小 | UPX 压缩后 | 节省 |
|------|---------|-----------|------|
| MIPS | ~7.6 MB | ~1.9 MB | 75% |
| ARMv7 | ~6.7 MB | ~1.9 MB | 71% |
| x86_64 | ~6.9 MB | ~2.2 MB | 68% |

UPX 压缩后的二进制运行时自解压到内存，性能影响 < 1ms。

## 技术细节

- **语言**: Go 1.21
- **依赖**: 仅 `gopkg.in/yaml.v3`
- **编译**: `CGO_ENABLED=0`，完全静态链接
- **内存**: 5-10 MB（典型运行时）
- **二进制**: 1.9-2.2 MB（UPX 压缩后）

## 支持的架构

| 架构 | 文件名 | 适用设备 |
|------|--------|---------|
| MIPS | `xxk-proxy-mips` | TP-Link, Netgear 等 MIPS 路由器 |
| ARMv7 | `xxk-proxy-arm` | Banana Pi, Raspberry Pi 等 ARM 设备 |
| x86_64 | `xxk-proxy-x86_64` | x86 软路由 (OpenWrt on x86) |

## 许可证

MIT — 见 [LICENSE](LICENSE)