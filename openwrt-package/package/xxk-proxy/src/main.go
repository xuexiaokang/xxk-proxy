package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 是YAML配置文件的结构体
type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`
	Log struct {
		Enabled    bool   `yaml:"enabled"`
		File       string `yaml:"file"`
		MaxSizeMB  int    `yaml:"maxsize"`
		MaxBackups int    `yaml:"maxbackups"`
		MaxAgeDays int    `yaml:"maxage"`
		Compress   bool   `yaml:"compress"`
	} `yaml:"log"`
	Reload int `yaml:"reload"`
}

var (
	port              = flag.Int("port", 5858, "监听端口号")
	logEnabled        = flag.Bool("log", true, "是否启用日志记录")
	logFile           = flag.String("logfile", "", "日志文件路径")
	configFilePath    = flag.String("config", "", "YAML配置文件路径")
	Reload            = flag.Int("reload", 30, "配置文件重新加载间隔（秒")
	serverCtx, cancel = context.WithCancel(context.Background())
	logConfigMutex    sync.Mutex
	cfg               Config
)

var ErrBrokenPipe = errors.New("broken pipe")

var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, defaultBufferSize) // 默认缓冲区大小
	},
}

const defaultBufferSize = 128 * 1024 // 默认缓冲区大小

func main() {
	flag.Parse()

	if *configFilePath != "" {
		err := loadConfig(*configFilePath)
		if err != nil {
			log.Fatalf("读取YAML配置文件失败: %v", err)
		}
	} else {
		cfg.Server.Port = *port
		cfg.Log.Enabled = *logEnabled
		cfg.Log.File = *logFile
		cfg.Reload = *Reload
	}

	setupLogger()
	// 启动配置文件自动加载
	go watchConfigFile(*configFilePath)

	http.HandleFunc("/", handler)

	go func() {
		if err := startHTTPServer(serverCtx); err != nil {
			log.Fatalf("启动HTTP服务器失败: %v", err)
		}
	}()

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	select {
	case <-serverCtx.Done():
	case sig := <-sigCh:
		log.Printf("收到信号 %v，正在关闭...", sig)
		cancel()
	}

	log.Println("代理服务已停止")
}

func loadConfig(configPath string) error {
	yamlData, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var newCfg Config
	err = yaml.Unmarshal(yamlData, &newCfg)
	if err != nil {
		return err
	}

	logConfigMutex.Lock()
	cfg = newCfg
	logConfigMutex.Unlock()

	// 重新设置日志记录器
	setupLogger()

	return nil
}

func watchConfigFile(configPath string) {
	if configPath == "" {
		return
	}

	ticker := time.NewTicker(time.Duration(cfg.Reload) * time.Second)
	defer ticker.Stop()

	lastModifiedTime := time.Now()

	for range ticker.C {
		fileInfo, err := os.Stat(configPath)
		if err != nil {
			log.Printf("无法获取配置文件信息: %v", err)
			continue
		}

		if fileInfo.ModTime().After(lastModifiedTime) {
			lastModifiedTime = fileInfo.ModTime()
			log.Println("检测到配置文件修改，重新加载配置...")
			err := loadConfig(configPath)
			if err != nil {
				log.Printf("重新加载配置文件失败: %v", err)
			} else {
				log.Println("配置文件重新加载完成")

				// 重新设置 HTTP 服务器的监听端口
				cancel()
				serverCtx, cancel = context.WithCancel(context.Background())
				go func() {
					if err := startHTTPServer(serverCtx); err != nil {
						log.Fatalf("启动HTTP服务器失败: %v", err)
					}
				}()
			}
		}
	}
}

func setupLogger() {
	logConfigMutex.Lock()
	defer logConfigMutex.Unlock()

	if !cfg.Log.Enabled {
		log.SetOutput(io.Discard)
		log.Println("日志记录已禁用")
		return
	}

	if cfg.Log.File == "" {
		log.SetOutput(os.Stdout)
		log.Println("日志记录已启用: 输出到标准输出")
		return
	}

	// 确保日志目录存在
	logDir := cfg.Log.File
	if idx := strings.LastIndex(logDir, "/"); idx >= 0 {
		os.MkdirAll(logDir[:idx], 0755)
	}

	f, err := os.OpenFile(cfg.Log.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("无法打开日志文件 %s: %v，回退到标准输出", cfg.Log.File, err)
		log.SetOutput(os.Stdout)
		return
	}
	log.SetOutput(f)
	log.Printf("日志记录已启用: 输出到文件 %s", cfg.Log.File)
}

func startHTTPServer(ctx context.Context) error {
	logConfigMutex.Lock()
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	logConfigMutex.Unlock()

	log.Printf("监听端口: %d...", cfg.Server.Port)

	srv := &http.Server{Addr: addr}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP服务器启动失败: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP服务器关闭失败: %v", err)
	}
	return nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	// 设置连接关闭标头
	r.Header.Set("Connection", "close")
	w.Header().Set("Connection", "close")

	// 获取目标路径和 URL
	targetPath := getTargetPath(r)
	targetURL := getTargetURL(r, targetPath)

	// 创建代理请求
	proxyReq, err := createProxyRequest(r, targetURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 发送代理请求
	proxyResp, err := sendProxyRequest(proxyReq, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer proxyResp.Body.Close()

	// 处理重定向
	if proxyResp.StatusCode == http.StatusMovedPermanently || proxyResp.StatusCode == http.StatusFound {
		scheme := r.URL.Scheme
		if scheme == "" {
			if r.TLS != nil {
				scheme = "https"
			} else {
				scheme = "http"
			}
		}
		host := r.URL.Host
		if host == "" {
			host = r.Host
		}
		baseURL := fmt.Sprintf("%s://%s", scheme, host)
		location := proxyResp.Header.Get("Location")
		if location != "" {
			location = baseURL + "/" + location
			log.Printf("正在重定向到: %s", location)
			w.Header().Set("Location", location)
		}
		w.WriteHeader(proxyResp.StatusCode)
		return
	}

	// 记录请求和响应
	logRequestAndResponse(r, targetURL, proxyResp)

	// 将代理响应复制到客户端
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	copyResponseToClientWithContext(ctx, w, proxyResp)
}

var client = &http.Client{
	Timeout: 0,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		ResponseHeaderTimeout: 10 * time.Second,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       100,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		const maxRedirects = 10
		redirectCount := len(via)
		if redirectCount >= maxRedirects {
			return fmt.Errorf("超出最大重定向次数 (%d 次)", redirectCount)
		}
		previousURL := via[redirectCount-1].URL
		redirectURL, err := url.Parse(req.Response.Header.Get("Location"))
		if err != nil {
			return fmt.Errorf("无效的重定向 URL: %w", err)
		}
		targetURL := previousURL.ResolveReference(redirectURL)
		req.URL = targetURL
		log.Printf("从 %s 重定向到 %s", previousURL, targetURL)
		return http.ErrUseLastResponse
	},
}

func sendProxyRequest(proxyReq *http.Request, r *http.Request) (*http.Response, error) {
	resp, err := client.Do(proxyReq)
	if err != nil {
		log.Printf("发送请求失败: %v", err)
		return nil, err
	}
	return resp, nil
}

func getTargetPath(r *http.Request) string {
	targetPath := r.URL.Path[1:]
	query := r.URL.RawQuery
	if query != "" {
		targetPath += "?" + query
	}
	return targetPath
}

func getTargetURL(r *http.Request, targetPath string) string {
	if strings.HasPrefix(targetPath, "http:/") {
		targetPath = strings.TrimPrefix(targetPath, "http:/")
	} else if strings.HasPrefix(targetPath, "https:/") {
		targetPath = strings.TrimPrefix(targetPath, "https:/")
	}
	useHTTPS := strings.HasPrefix(r.URL.Path, "/https:/")

	var targetURL string
	if useHTTPS {
		targetURL = "https://" + r.URL.Host + targetPath
	} else {
		targetURL = "http://" + r.URL.Host + targetPath
	}
	return targetURL
}

func createProxyRequest(r *http.Request, targetURL string) (*http.Request, error) {
	proxyReq, err := http.NewRequest(r.Method, targetURL, nil)
	if err != nil {
		return nil, err
	}

	if r.Body != nil {
		bodyCopy, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyCopy))
		proxyReq.Body = io.NopCloser(bytes.NewBuffer(bodyCopy))
	}

	proxyReq.Header = make(http.Header)
	copyHeader(proxyReq.Header, r.Header)

	clientIP := strings.Split(r.RemoteAddr, ":")[0]
	if prior, ok := proxyReq.Header["X-Forwarded-For"]; ok {
		clientIP = prior[0] + ", " + clientIP
	}
	proxyReq.Header.Set("X-Forwarded-For", clientIP)

	return proxyReq, nil
}

func logRequestAndResponse(r *http.Request, targetURL string, proxyResp *http.Response) {
	remoteAddr := r.RemoteAddr
	forwardedFor := r.Header.Get("X-Forwarded-For")
	userAgent := r.Header.Get("User-Agent")

	if forwardedFor != "" {
		log.Printf("[X-Forwarded-For: %s] [%s] %s %s %s %d [User-Agent: %s]",
			forwardedFor, remoteAddr, r.Method, targetURL, r.Proto, proxyResp.StatusCode, userAgent)
	} else {
		log.Printf("[%s] %s %s %s %d [User-Agent: %s]",
			remoteAddr, r.Method, targetURL, r.Proto, proxyResp.StatusCode, userAgent)
	}
}

func copyResponseToClientWithContext(ctx context.Context, w http.ResponseWriter, proxyResp *http.Response) {
	defer func() {
		if err := proxyResp.Body.Close(); err != nil {
			log.Printf("关闭响应体错误: %v", err)
		}
	}()

	copyHeader(w.Header(), proxyResp.Header)
	w.WriteHeader(proxyResp.StatusCode)

	buf := bufferPool.Get().([]byte)
	defer bufferPool.Put(buf)

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := copyWithContext(ctx, w, proxyResp.Body, buf); err != nil {
			handleCopyError(w, err, proxyResp)
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("客户端断开连接")
	case <-done:
		log.Println("响应成功完成")
	}
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader, buf []byte) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			n, readErr := src.Read(buf)
			if n > 0 {
				written := 0
				for written < n {
					wn, writeErr := dst.Write(buf[written:n])
					if writeErr != nil {
						return fmt.Errorf("写入错误: %w", writeErr)
					}
					written += wn
				}
			}
			if readErr != nil {
				if errors.Is(readErr, io.EOF) {
					return nil
				}
				return fmt.Errorf("读取错误: %w", readErr)
			}
		}
	}
}

func handleCopyError(w http.ResponseWriter, err error, proxyResp *http.Response) {
	if errors.Is(err, context.Canceled) {
		log.Printf("传输被取消: %v, URL: %s", err, proxyResp.Request.URL.String())
	} else if errors.Is(err, context.DeadlineExceeded) {
		log.Printf("传输超时: %v, URL: %s", err, proxyResp.Request.URL.String())
	} else {
		log.Printf("传输错误: %v, URL: %s", err, proxyResp.Request.URL.String())
	}
	if w.Header().Get("Content-Length") == "" {
		http.Error(w, "服务器错误", http.StatusInternalServerError)
	}
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}