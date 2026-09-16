#!/bin/bash
# 在 OpenWrt 设备上执行此脚本完成部署
# 用法: wget -O - http://你的IP/deploy.sh | sh
#    或: curl http://你的IP/deploy.sh | sh

set -e

echo "============================================"
echo "xxk-proxy OpenWrt 部署脚本"
echo "============================================"

# 1. 创建配置文件
echo "[1/4] 创建 UCI 配置..."
cat > /etc/config/xxk-proxy << 'CONFEOF'
config xxk-proxy 'global'
    option enabled '1'
    option port '5858'
    option log_enabled '1'
    option log_file '/var/log/proxy/proxy.log'
    option log_maxsize '10'
    option log_maxbackups '3'
    option log_maxage '28'
    option log_compress '1'
    option reload '30'
CONFEOF
echo "      配置已创建"

# 2. 创建 init 脚本
echo "[2/4] 创建 init 脚本..."
cat > /etc/init.d/xxk-proxy << 'INITEOF'
#!/bin/sh /etc/rc.common

START=99
STOP=10

PROG=/usr/bin/xxk-proxy
CONF=/etc/config/xxk-proxy
LOG_DIR=/var/log/proxy

. /lib/functions.sh

get_proxy_enabled() {
    uci get xxk-proxy.global.enabled 2>/dev/null || echo "0"
}

get_proxy_port() {
    uci get xxk-proxy.global.port 2>/dev/null || echo "5858"
}

get_proxy_log() {
    uci get xxk-proxy.global.log_enabled 2>/dev/null || echo "1"
}

get_proxy_logfile() {
    uci get xxk-proxy.global.log_file 2>/dev/null || echo ""
}

get_proxy_reload() {
    uci get xxk-proxy.global.reload 2>/dev/null || echo "30"
}

start() {
    local enabled port log logfile reload args

    enabled=$(get_proxy_enabled)
    if [ "$enabled" != "1" ]; then
        echo "xxk-proxy is disabled, skipping start"
        return 0
    fi

    port=$(get_proxy_port)
    log=$(get_proxy_log)
    logfile=$(get_proxy_logfile)
    reload=$(get_proxy_reload)

    mkdir -p "$LOG_DIR"

    args="-port=$port"
    [ "$log" = "1" ] && args="$args -log=true" || args="$args -log=false"
    [ -n "$logfile" ] && args="$args -logfile=$logfile"
    [ -n "$reload" ] && args="$args -reload=$reload"

    echo "Starting xxk-proxy on port $port..."
    $PROG $args &
}

stop() {
    echo "Stopping xxk-proxy..."
    killall -TERM $PROG 2>/dev/null
    sleep 1
    killall -KILL $PROG 2>/dev/null || true
}

restart() {
    stop
    sleep 1
    start
}

reload() {
    restart
}
INITEOF
chmod +x /etc/init.d/xxk-proxy
echo "      init 脚本已创建"

# 3. 创建 hotplug 脚本
echo "[3/4] 创建 hotplug 脚本..."
cat > /etc/hotplug.d/iface/60-xxk-proxy << 'HOTPLUGEOF'
#!/bin/sh
case "$ACTION" in
    "ifup")
        WAN_IFACE=$(uci get xxk-proxy.global.wan_iface 2>/dev/null || echo "")
        if [ -n "$WAN_IFACE" ] && [ "$INTERFACE" = "$WAN_IFACE" ]; then
            /etc/init.d/xxk-proxy restart
        fi
        ;;
esac
HOTPLUGEOF
chmod +x /etc/hotplug.d/iface/60-xxk-proxy
echo "      hotplug 脚本已创建"

# 4. 创建库函数
echo "[4/4] 创建库函数..."
mkdir -p /lib/functions
cat > /lib/functions/xxk-proxy.sh << 'LIBEOF'
xxk-proxy_get_config() {
    local section="$1"
    local option="$2"
    local default="$3"
    local value
    value=$(uci get "xxk-proxy.$section.$option" 2>/dev/null)
    if [ -n "$value" ]; then
        echo "$value"
    else
        echo "$default"
    fi
}

xxk-proxy_enabled() {
    local enabled
    enabled=$(xxk-proxy_get_config "global" "enabled" "0")
    [ "$enabled" = "1" ]
}

xxk-proxy_port() {
    xxk-proxy_get_config "global" "port" "5858"
}

xxk-proxy_logfile() {
    xxk-proxy_get_config "global" "log_file" "/var/log/proxy/proxy.log"
}

xxk-proxy_reload() {
    xxk-proxy_get_config "global" "reload" "30"
}

xxk-proxy_gen_args() {
    local args=""
    local port logfile log_enabled reload
    port=$(xxk-proxy_port)
    log_enabled=$(xxk-proxy_get_config "global" "log_enabled" "1")
    logfile=$(xxk-proxy_logfile)
    reload=$(xxk-proxy_reload)
    args="-port=$port"
    [ "$log_enabled" = "1" ] && args="$args -log=true" || args="$args -log=false"
    [ -n "$logfile" ] && args="$args -logfile=$logfile"
    [ -n "$reload" ] && args="$args -reload=$reload"
    echo "$args"
}
LIBEOF
chmod +x /lib/functions/xxk-proxy.sh
echo "      库函数已创建"

echo ""
echo "============================================"
echo "部署完成！"
echo "============================================"
echo ""
echo "请确保 /usr/bin/xxk-proxy 二进制已就位，然后启动："
echo "  /etc/init.d/xxk-proxy start"
echo ""
echo "或开机自启："
echo "  /etc/init.d/xxk-proxy enable"