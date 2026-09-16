#!/bin/sh
# xxk-proxy library functions for OpenWrt

# Get proxy config value
# Usage: xxk-proxy_get_config <section> <option> [default]
xxk-proxy_get_config() {
    local section="$1"
    local option="$2"
    local default="$3"
    local value

    value=$(uci_get_config_value "xxk-proxy" "$section.$option" 2>/dev/null)
    if [ -n "$value" ]; then
        echo "$value"
    else
        echo "$default"
    fi
}

# Check if proxy is enabled
xxk-proxy_enabled() {
    local enabled
    enabled=$(xxk-proxy_get_config "global" "enabled" "0")
    [ "$enabled" = "1" ]
}

# Get proxy port
xxk-proxy_port() {
    xxk-proxy_get_config "global" "port" "5858"
}

# Get proxy log file path
xxk-proxy_logfile() {
    xxk-proxy_get_config "global" "log_file" "/var/log/proxy/proxy.log"
}

# Get proxy reload interval
xxk-proxy_reload() {
    xxk-proxy_get_config "global" "reload" "30"
}

# Generate command line arguments from UCI config
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