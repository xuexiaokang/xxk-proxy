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
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
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
	logMaxSizeMB      = flag.Int("log.maxsize", 10, "日志文件大小限制（MB）")
	logMaxBackups     = flag.Int("log.maxbackups", 3, "保留的最大备份文件数")
	logMaxAgeDays     = flag.Int("log.maxage", 28, "保留日志文件的最大天数")
	logCompress       = flag.Bool("log.compress", true, "是否压缩日志文件")
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
		cfg.Log.MaxSizeMB = *logMaxSizeMB
		cfg.Log.MaxBackups = *logMaxBackups
		cfg.Log.MaxAgeDays = *logMaxAgeDays
		cfg.Log.Compress = *logCompress
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

	<-serverCtx.Done()
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

	ticker := time.NewTicker(time.Duration(cfg.Reload) * time.Second) // 定时器间隔可以根据需要调整
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
		// 如果日志记录被禁用，则将日志输出设置为丢弃
		log.SetOutput(io.Discard)
		log.Println("日志记录已禁用")
		return
	}

	if cfg.Log.File == "" {
		// 如果日志文件路径为空，则默认将日志输出到标准输出
		log.SetOutput(os.Stdout)
		log.Println("日志记录已启用: 输出到标准输出")
		return
	}

	// 否则，根据配置设置日志记录到指定文件
	logger := &lumberjack.Logger{
		Filename:   cfg.Log.File,
		MaxSize:    cfg.Log.MaxSizeMB,
		MaxBackups: cfg.Log.MaxBackups,
		MaxAge:     cfg.Log.MaxAgeDays,
		Compress:   cfg.Log.Compress,
	}
	log.SetOutput(logger)
	log.Printf("日志记录已启用: 输出到文件 %s", cfg.Log.File)
}

func startHTTPServer(ctx context.Context) error {
	logConfigMutex.Lock()
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	logConfigMutex.Unlock()

	fmt.Printf("监听端口: %d...\n", cfg.Server.Port)

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
	// 设置连接关闭标头，告知客户端和代理服务器都关闭连接
	r.Header.Set("Connection", "close")
	w.Header().Set("Connection", "close")

	// 保留原始请求的路径，获取目标路径
	targetPath := getTargetPath(r)
	// 根据目标路径获取目标 URL
	targetURL := getTargetURL(r, targetPath)

	// 创建代理请求
	proxyReq, err := createProxyRequest(r, targetURL)
	if err != nil {
		// 请求创建失败，返回 500 错误
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 发送代理请求
	proxyResp, err := sendProxyRequest(proxyReq, r)
	if err != nil {
		// 发送请求失败，返回 502 错误
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	// 确保代理响应体在函数结束时关闭
	defer proxyResp.Body.Close()

	// 如果响应是 301（永久移动）或 302（临时移动）重定向
	if proxyResp.StatusCode == http.StatusMovedPermanently || proxyResp.StatusCode == http.StatusFound {
		// 获取原始请求的协议（Scheme），如果没有则使用默认的 http 协议
		scheme := r.URL.Scheme
		if scheme == "" {
			if r.TLS != nil {
				scheme = "https"
			} else {
				scheme = "http"
			}
		}
		// 获取原始请求的主机（Host），如果没有则使用 r.Host
		host := r.URL.Host
		if host == "" {
			host = r.Host
		}
		// 构造基础 URL，包含协议和主机
		baseURL := fmt.Sprintf("%s://%s", scheme, host)

		// 获取 Location 标头（重定向地址）
		location := proxyResp.Header.Get("Location")
		if location != "" {
			// 如果 Location 不完整，拼接基础 URL 和 Location
			location = baseURL + "/" + location

			// 输出重定向的目标 URL
			log.Printf("正在重定向到: %s", location)
			// 设置响应的 Location 标头，进行重定向
			w.Header().Set("Location", location)
		}

		// 设置响应状态码并返回
		w.WriteHeader(proxyResp.StatusCode)
		return
	}

	// 记录请求和响应信息（可选，日志记录）
	logRequestAndResponse(r, targetURL, proxyResp)
	// 将代理响应复制到客户端
	ctx, cancel := context.WithCancel(context.Background()) // 没有超时限制
	defer cancel()

	copyResponseToClientWithContext(ctx, w, proxyResp)
}

var client = &http.Client{
	Timeout: 0, // 整个请求超时
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second, // 连接超时
			KeepAlive: 30 * time.Second, // 长连接保持时间
			DualStack: true,
		}).DialContext,
		ResponseHeaderTimeout: 10 * time.Second, // 接收响应头超时
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       100, // 可选：限制每个主机的最大连接数
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		const maxRedirects = 10 // 最大重定向次数
		redirectCount := len(via)

		// 超过最大重定向次数，直接返回错误
		if redirectCount >= maxRedirects {
			return fmt.Errorf("超出最大重定向次数 (%d 次)", maxRedirects)
		}

		// 获取上一个请求的 URL
		previousURL := via[redirectCount-1].URL

		// 根据响应头解析目标重定向 URL
		redirectURL, err := url.Parse(req.Response.Header.Get("Location"))
		if err != nil {
			return fmt.Errorf("无效的重定向 URL: %w", err)
		}

		// 计算目标 URL，支持相对路径
		targetURL := previousURL.ResolveReference(redirectURL)
		req.URL = targetURL

		log.Printf("从 %s 重定向到 %s", previousURL, targetURL)

		// 不跟随重定向
		return http.ErrUseLastResponse
	},
}

func sendProxyRequest(proxyReq *http.Request, r *http.Request) (*http.Response, error) {
	// 执行代理请求
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
	// 创建新的请求，将原始请求的方法和URL传入
	proxyReq, err := http.NewRequest(r.Method, targetURL, nil)
	if err != nil {
		return nil, err
	}

	// 处理请求体
	if r.Body != nil {
		bodyCopy, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyCopy))           // 还原原始请求的 Body
		proxyReq.Body = io.NopCloser(bytes.NewBuffer(bodyCopy))    // 设置代理请求的 Body
	}

	// 复制头部信息
	proxyReq.Header = make(http.Header)
	copyHeader(proxyReq.Header, r.Header)

	// 添加 X-Forwarded-For 头部，包含客户端的 IP
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

	// 复制响应头
	copyHeader(w.Header(), proxyResp.Header)
	w.WriteHeader(proxyResp.StatusCode)

	buf := bufferPool.Get().([]byte)
	defer bufferPool.Put(buf) // 将缓冲区放回池中

	done := make(chan struct{}) // 通知完成的通道
	go func() {
		defer close(done)
		if err := copyWithContext(ctx, w, proxyResp.Body, buf); err != nil {
			handleCopyError(w, err, proxyResp)
		}
	}()

	select {
	case <-ctx.Done(): // 客户端断开连接
		log.Println("客户端断开连接")
	case <-done:
		log.Println("响应成功完成")
	}
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader, buf []byte) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err() // 客户端取消或断开连接
		default:
			// 从源读取数据
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
					return nil // 流结束
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

	// 可选：向客户端返回错误
	if w.Header().Get("Content-Length") == "" {
		http.Error(w, "服务器错误", http.StatusInternalServerError)
	}
}

// copyHeader 负责复制请求头
func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}