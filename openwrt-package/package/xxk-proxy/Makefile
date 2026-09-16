# OpenWrt Makefile for xxk-proxy
#
# This package builds a lightweight HTTP/HTTPS proxy server for OpenWrt.
#
# Usage:
#   1. Add to feeds: ln -s /path/to/xxk-proxy package/xxk-proxy
#   2. ./scripts/feeds update xxk-proxy
#   3. ./scripts/feeds install -a
#   4. make package/xxk-proxy/compile V=s
#
# Or build standalone:
#   make package/xxk-proxy/compile V=s

include $(TOPDIR)/rules.mk

PKG_NAME:=xxk-proxy
PKG_VERSION:=1.0.1
PKG_RELEASE:=1

# Source: local directory
PKG_SOURCE_DIR:=@$(CURDIR)/src
PKG_BUILD_DIR:=$(BUILD_DIR)/$(PKG_NAME)-$(PKG_VERSION)

PKG_FIXUP:=
PKG_INSTALL:=1
PKG_BUILD_PARALLEL:=1

include $(INCLUDE_DIR)/package.mk
include $(INCLUDE_DIR)/go.mk

# ============================================================
# Package definition
# ============================================================

define Package/xxk-proxy
  SECTION:=net
  CATEGORY:=Network
  TITLE:=XXK HTTP Proxy Server
  URL:=https://github.com/xxk/xxk-proxy
  DEPENDS:=+libc
  MENU:=1
endef

define Package/xxk-proxy/description
  A lightweight HTTP/HTTPS proxy server written in Go,
  optimized for OpenWrt routers with low memory footprint.
  Supports YAML configuration, log rotation, and hot-reload.
endef

define Package/xxk-proxy/conffiles
/etc/config/xxk-proxy
endef

# ============================================================
# Build instructions
# ============================================================

# Copy source to build directory
define Build/Prepare
	$(call Build/Prepare/Default)
	$(CP) $(PKG_SOURCE_DIR)/. $(PKG_BUILD_DIR)/
endef

# Build the Go binary
define Build/Compile
	$(call Go/Compile/Package,xxk-proxy)
endef

# Install files to the package staging directory
define Package/xxk-proxy/install
	# Binary
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/xxk-proxy $(1)/usr/bin/xxk-proxy

	# Init script
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/xxk-proxy.init $(1)/etc/init.d/xxk-proxy

	# UCI config
	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_DATA) ./files/xxk-proxy.config $(1)/etc/config/xxk-proxy

	# Hotplug script
	$(INSTALL_DIR) $(1)/etc/hotplug.d/iface
	$(INSTALL_BIN) ./files/xxk-proxy.hotplug $(1)/etc/hotplug.d/iface/60-xxk-proxy

	# Library functions
	$(INSTALL_DIR) $(1)/lib/functions
	$(INSTALL_DATA) ./files/xxk-proxy.sh $(1)/lib/functions/xxk-proxy.sh

	# Log directory
	$(INSTALL_DIR) $(1)/var/log/proxy

endef

$(eval $(call BuildPackage,xxk-proxy))