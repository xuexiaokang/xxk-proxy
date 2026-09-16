# xxk-proxy for OpenWrt

轻量级 HTTP/HTTPS 代理服务器，专为 OpenWrt 路由器优化。

## 功能特性

- **HTTP/HTTPS 代理**：支持标准 HTTP 代理和 HTTPS 隧道代理
- **YAML 配置**：支持配置文件和命令行参数两种方式
- **日志轮转**：支持按文件大小、备份数量、保留天数自动轮转
- **热重载**：配置文件修改后自动重新加载
- **信号处理**：支持 SIGINT/SIGTERM 优雅关闭
- **低内存占用**：专为嵌入式设备优化，静态编译，无外部依赖
- **OpenWrt 集成**：支持 UCI 配置、init.d 服务管理、hotplug 事件

## 支持的架构

| 架构       | 文件名                  | 原始大小  | UPX压缩后  | 适用设备                     |
|------------|------------------------|----------|-----------|------------------------------|
| MIPS LE    | `xxk-proxy-mipsle`     | ~7.6 MB  | ~1.9 MB   | 大多数 MIPS 路由器 (TP-Link, Netgear) |
| ARMv7     | `xxk-proxy-arm`        | ~6.7 MB  | ~1.9 MB   | ARM 路由器 (Banana Pi, Raspberry Pi) |
| x86_64    | `xxk-proxy-x86_64`     | ~6.9 MB  | ~2.2 MB   | x86 软路由 (OpenWrt on x86) |

## 快速安装

### 方法一：使用预编译二进制

```bash
# 1. 下载对应架构的二进制文件（已 UPX 压缩）
#    从 output/ 目录复制到 OpenWrt 设备
scp output/xxk-proxy-mipsle root@openwrt:/usr/bin/xxk-proxy

# 2. 复制配置文件和启动脚本
scp package/xxk-proxy/files/xxk-proxy.init root@openwrt:/etc/init.d/xxk-proxy
scp package/xxk-proxy/files/xxk-proxy.config root@openwrt:/etc/config/xxk-proxy
scp package/xxk-proxy/files/xxk-proxy.hotplug root@openwrt:/etc/hotplug.d/iface/60-xxk-proxy
scp package/xxk-proxy/files/xxk-proxy.sh root@openwrt:/lib/functions/xxk-proxy.sh

# 3. 设置权限并启动
chmod +x /usr/bin/xxk-proxy /etc/init.d/xxk-proxy
/etc/init.d/xxk-proxy enable
/etc/init.d/xxk-proxy start
```

### 方法二：使用 OpenWrt 构建系统

```bash
# 1. 将 xxk-proxy 添加到 OpenWrt feeds
cd openwrt
ln -s /path/to/xxk-proxy package/xxk-proxy
./scripts/feeds update xxk-proxy
./scripts/feeds install -a

# 2. 编译
make package/xxk-proxy/compile V=s

# 3. 安装生成的 ipk 包
cp bin/packages/packages/xxk-proxy_*.ipk /path/to/openwrt/
opkg install xxk-proxy_*.ipk
```

### 方法三：使用 Docker 构建

```bash
docker build -t xxk-proxy-openwrt -f openwrt-package/Dockerfile .
docker run --rm xxk-proxy-openwrt cat /usr/bin/xxk-proxy > output/xxk-proxy-docker
```

## 配置说明

### UCI 配置（推荐）

```bash
# 编辑配置文件
uci edit xxk-proxy
# 或直接编辑 /etc/config/xxk-proxy

config xxk-proxy 'global'
    option enabled '1'          # 是否启用
    option port '5858'          # 监听端口
    option log_enabled '1'      # 是否启用日志
    option log_file '/var/log/proxy/proxy.log'  # 日志文件路径
    option log_maxsize '10'     # 日志文件大小限制 (MB)
    option log_maxbackups '3'   # 保留备份文件数
    option log_maxage '28'      # 保留日志文件最大天数
    option log_compress '1'     # 是否压缩旧日志
    option reload '30'          # 配置重载间隔 (秒)

# 应用配置
uci commit xxk-proxy
/etc/init.d/xxk-proxy restart
```

### 命令行参数

```bash
xxk-proxy -port=5858 -log=true -logfile=/var/log/proxy/proxy.log
```

| 参数            | 默认值   | 说明                         |
|-----------------|---------|------------------------------|
| `-port`         | 5858    | 监听端口号                   |
| `-log`          | true    | 是否启用日志记录             |
| `-logfile`      | (空)    | 日志文件路径，空则输出到stdout |
| `-log.maxsize`  | 10      | 日志文件大小限制 (MB)        |
| `-log.maxbackups` | 3     | 保留的最大备份文件数         |
| `-log.maxage`   | 28      | 保留日志文件的最大天数       |
| `-log.compress` | true    | 是否压缩日志文件            |
| `-config`       | (空)    | YAML 配置文件路径           |
| `-reload`       | 30      | 配置文件重新加载间隔 (秒)    |

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

## 使用说明

代理服务器监听在 `http://0.0.0.0:5858`，客户端可以通过以下方式使用：

```bash
# HTTP 代理
curl -x http://openwrt:5858 http://example.com

# HTTPS 代理（通过 HTTP CONNECT 隧道）
curl -x http://openwrt:5858 https://example.com
```

## 日志查看

```bash
# 查看实时日志
logread -f -e xxk-proxy

# 或直接查看日志文件
tail -f /var/log/proxy/proxy.log
```

## 服务管理

```bash
# 启动 / 停止 / 重启
/etc/init.d/xxk-proxy start
/etc/init.d/xxk-proxy stop
/etc/init.d/xxk-proxy restart

# 开机自启 / 取消自启
/etc/init.d/xxk-proxy enable
/etc/init.d/xxk-proxy disable

# 查看状态
/etc/init.d/xxk-proxy status
```

## 构建自定义版本

```bash
# 构建特定架构
./build.sh mipsle   # MIPS LE (最常用)
./build.sh arm      # ARMv7
./build.sh x86_64   # x86_64

# 自定义编译参数
cd package/xxk-proxy/src
CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat \
    go build -ldflags="-s -w" -o xxk-proxy .
```

## 体积优化

Go 静态编译的二进制默认体积较大（6-8 MB），这对嵌入式设备不友好。
我们使用 **UPX** 进行压缩，体积减少约 75%：

| 架构       | 原始大小  | UPX 压缩后 | 节省比例 |
|------------|----------|-----------|---------|
| MIPS LE    | ~7.6 MB  | ~1.9 MB   | 75%     |
| ARMv7     | ~6.7 MB  | ~1.9 MB   | 71%     |
| x86_64    | ~6.9 MB  | ~2.2 MB   | 68%     |

UPX 压缩后的二进制在运行时会自解压到内存，对性能影响极小（< 1ms）。

## 技术细节

- **语言**：Go 1.21
- **依赖**：仅 `gopkg.in/yaml.v3`（YAML 解析）
- **静态编译**：无动态链接依赖，`CGO_ENABLED=0`
- **内存占用**：约 5-10 MB（典型运行时）
- **二进制大小**：约 1.9-2.2 MB（UPX 压缩后）

## 原始项目

本项目基于 `E:\QClaw\xxk-proxy\1.0.1` 进行 OpenWrt 适配，保留了原始代理功能，并增加了：

1. 信号处理（SIGINT/SIGTERM 优雅关闭）
2. 去除 `lumberjack` 依赖，改用标准库日志
3. OpenWrt init.d 服务脚本
4. UCI 配置支持
5. Hotplug 事件支持
6. 多架构交叉编译支持