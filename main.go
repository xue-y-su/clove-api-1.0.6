package main

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"net/netip"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/router"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	_ "github.com/QuantumNous/new-api/setting/performance_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

//go:embed web/default/dist
var buildFS embed.FS

//go:embed web/default/dist/index.html
var indexPage []byte

//go:embed web/classic/dist
var classicBuildFS embed.FS

//go:embed web/classic/dist/index.html
var classicIndexPage []byte

//go:embed web/snow/dist
var snowBuildFS embed.FS

//go:embed web/snow/dist/index.html
var snowIndexPage []byte

func main() {
	startTime := time.Now()

	err := InitResources()
	if err != nil {
		common.FatalLog("failed to initialize resources: " + err.Error())
		return
	}

	common.SysLog("New API " + common.Version + " started")
	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	if common.DebugEnabled {
		common.SysLog("running in debug mode")
	}

	defer func() {
		err := model.CloseDB()
		if err != nil {
			common.FatalLog("failed to close database: " + err.Error())
		}
	}()
	ipSecurityProjectionWorker := service.StartIPSecurityProjectionWorker()
	defer func() {
		stopContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := ipSecurityProjectionWorker.Stop(stopContext); err != nil {
			common.SysError(fmt.Sprintf("failed to stop IP security projection worker: %v", err))
		}
	}()
	auditOutboxWorker := service.StartAuditOutboxWorker()
	defer func() {
		stopContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := auditOutboxWorker.Stop(stopContext); err != nil {
			common.SysError(fmt.Sprintf("failed to stop audit outbox worker: %v", err))
		}
	}()
	tokenSecretMigrationWorker := service.StartTokenSecretMigrationWorker()
	defer func() {
		stopContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tokenSecretMigrationWorker.Stop(stopContext); err != nil {
			common.SysError(fmt.Sprintf("failed to stop token secret migration worker: %v", err))
		}
	}()
	invitationRewardWorker := service.StartInvitationRewardWorker()
	defer func() {
		stopContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := invitationRewardWorker.Stop(stopContext); err != nil {
			common.SysError(fmt.Sprintf("failed to stop invitation reward worker: %v", err))
		}
	}()

	if common.RedisEnabled {
		// for compatibility with old versions
		common.MemoryCacheEnabled = true
	}
	if common.MemoryCacheEnabled {
		common.SysLog("memory cache enabled")
		common.SysLog(fmt.Sprintf("sync frequency: %d seconds", common.SyncFrequency))

		// Add panic recovery and retry for InitChannelCache
		func() {
			defer func() {
				if r := recover(); r != nil {
					common.SysLog(fmt.Sprintf("InitChannelCache panic: %v, retrying once", r))
					// Retry once
					_, _, fixErr := model.FixAbility()
					if fixErr != nil {
						common.FatalLog(fmt.Sprintf("InitChannelCache failed: %s", fixErr.Error()))
					}
				}
			}()
			model.InitChannelCache()
		}()

		go model.SyncChannelCache(common.SyncFrequency)
	}

	// Warm pricing after channel cache initialization so Advanced Custom
	// endpoint inference can read cached route settings on first request.
	model.GetPricing()

	// 热更新配置
	go model.SyncOptions(common.SyncFrequency)

	// 周期性重载授权策略，保证多节点/多 master 部署下权限变更能传播到每个实例
	go authz.StartPolicySync(common.SyncFrequency)

	// 数据看板
	go model.UpdateQuotaData()

	if os.Getenv("CHANNEL_UPDATE_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_UPDATE_FREQUENCY"))
		if err != nil {
			common.FatalLog("failed to parse CHANNEL_UPDATE_FREQUENCY: " + err.Error())
		}
		go controller.AutomaticallyUpdateChannels(frequency)
	}

	// Codex credential auto-refresh check every 10 minutes, refresh when expires within 1 day
	service.StartCodexCredentialAutoRefreshTask()

	// Subscription quota reset task (daily/weekly/monthly/custom)
	service.StartSubscriptionQuotaResetTask()

	// Expire unused portions of opted-in daily check-in rewards.
	service.StartCheckinExpirationTask()

	// Report this process as a system instance so the System Info page can show
	// all currently alive nodes in multi-instance deployments.
	service.StartSystemInstanceReporter()

	// Wire task polling adaptor factory (breaks service -> relay import cycle).
	// Must run before the system task runner starts: the async_task_poll handler
	// calls service.RunTaskPollingOnce, which needs this factory set.
	service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor {
		a := relay.GetTaskAdaptor(platform)
		if a == nil {
			return nil
		}
		return a
	}

	// Register the periodic channel test, upstream model update, and async task
	// polling (Midjourney / Suno / video) jobs as scheduled system tasks
	// (DB-lease dedup across masters + run history), then start the runner that
	// schedules and executes them. Master-only execution and the UpdateTask
	// switch are enforced inside the runner and each handler's Enabled().
	controller.RegisterScheduledSystemTasks()
	service.StartSystemTaskRunner()

	if os.Getenv("BATCH_UPDATE_ENABLED") == "true" {
		common.BatchUpdateEnabled = true
		common.SysLog("batch update enabled with interval " + strconv.Itoa(common.BatchUpdateInterval) + "s")
		model.InitBatchUpdater()
	}

	if os.Getenv("ENABLE_PPROF") == "true" {
		gopool.Go(func() {
			log.Println(newPprofServer().ListenAndServe())
		})
		go common.Monitor()
		common.SysLog("pprof enabled")
	}

	err = common.StartPyroScope()
	if err != nil {
		common.SysError(fmt.Sprintf("start pyroscope error : %v", err))
	}

	// Initialize HTTP server
	server := gin.New()
	if err := configureTrustedProxies(server); err != nil {
		log.Fatalf("invalid TRUSTED_PROXIES: %v", err)
	}
	server.Use(gin.CustomRecovery(func(c *gin.Context, err any) {
		common.SysLog(fmt.Sprintf("panic detected: %v", err))
		respondWithPanicError(c)
	}))
	// This will cause SSE not to work!!!
	//server.Use(gzip.Gzip(gzip.DefaultCompression))
	server.Use(middleware.RequestId())
	server.Use(middleware.Version())
	server.Use(middleware.SecurityHeaders())
	server.Use(middleware.I18n())
	middleware.SetUpLogger(server)
	// Initialize session store
	store := cookie.NewStore([]byte(common.SessionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   2592000, // 30 days
		HttpOnly: true,
		Secure:   common.SessionCookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	server.Use(sessions.Sessions("session", store))

	InjectUmamiAnalytics()
	InjectGoogleAnalytics()

	// 设置路由
	router.SetRouter(server, router.ThemeAssets{
		DefaultBuildFS:   buildFS,
		DefaultIndexPage: indexPage,
		ClassicBuildFS:   classicBuildFS,
		ClassicIndexPage: classicIndexPage,
		SnowBuildFS:      snowBuildFS,
		SnowIndexPage:    snowIndexPage,
	})
	var port = os.Getenv("PORT")
	if port == "" {
		port = strconv.Itoa(*common.Port)
	}

	srv := newHTTPServer(serverListenAddress(port), server)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			common.FatalLog("failed to start HTTP server: " + err.Error())
		}
	}()

	time.Sleep(100 * time.Millisecond)

	common.LogStartupSuccess(startTime, port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	common.SysLog(fmt.Sprintf("received signal: %v, shutting down...", sig))

	// SSE streams may run for minutes; give them time to finish before forced exit
	shutdownTimeout := time.Duration(common.GetEnvOrDefault("SHUTDOWN_TIMEOUT_SECONDS", 120)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		common.SysError(fmt.Sprintf("server forced to shutdown: %v", err))
	}
	auditStopContext, auditStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := auditOutboxWorker.Stop(auditStopContext); err != nil {
		common.SysError(fmt.Sprintf("failed to stop audit outbox worker: %v", err))
	}
	auditStopCancel()
	invitationStopContext, invitationStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := invitationRewardWorker.Stop(invitationStopContext); err != nil {
		common.SysError(fmt.Sprintf("failed to stop invitation reward worker: %v", err))
	}
	invitationStopCancel()
	// 内存中的看板数据保存入库，避免重启丢失未落库数据 (issue #5679)
	if common.DataExportEnabled {
		model.SaveQuotaDataCache()
	}
	common.SysLog("server exited")
}

func respondWithPanicError(c *gin.Context) {
	middleware.RespondWithPanicError(c)
}

func trustedProxiesFromEnv() ([]string, error) {
	raw := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES"))
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	proxies := make([]string, 0, len(parts))
	for _, part := range parts {
		proxy := strings.TrimSpace(part)
		if proxy == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(proxy); err == nil && prefix.Bits() == 0 {
			return nil, fmt.Errorf("unsafe open trusted proxy range %q", proxy)
		}
		proxies = append(proxies, proxy)
	}
	return proxies, nil
}

func serverListenAddress(port string) string {
	host := strings.TrimSpace(os.Getenv("BIND_ADDRESS"))
	if host == "" {
		return ":" + port
	}
	return net.JoinHostPort(host, port)
}

func configureTrustedProxies(server *gin.Engine) error {
	proxies, err := trustedProxiesFromEnv()
	if err != nil {
		return err
	}
	if err := server.SetTrustedProxies(proxies); err != nil {
		return err
	}
	if len(proxies) == 0 {
		common.SysLog("trusted proxies disabled; forwarded headers will be ignored")
	} else {
		common.SysLog("trusted proxies configured: " + strings.Join(proxies, ","))
	}
	return nil
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}
}

func newPprofServer() *http.Server {
	addr := strings.TrimSpace(os.Getenv("PPROF_ADDR"))
	if addr == "" {
		addr = "127.0.0.1:8005"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}
}

var analyticsIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func validatedAnalyticsID(value string) (string, bool) {
	value = strings.TrimSpace(value)
	return value, analyticsIDPattern.MatchString(value)
}

func validatedAnalyticsScriptURL(value string) (string, bool) {
	value = strings.TrimSpace(value)
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	return parsed.String(), true
}

func InjectUmamiAnalytics() {
	analyticsInjectBuilder := &strings.Builder{}
	if os.Getenv("UMAMI_WEBSITE_ID") != "" {
		umamiSiteID, validID := validatedAnalyticsID(os.Getenv("UMAMI_WEBSITE_ID"))
		umamiScriptURL := os.Getenv("UMAMI_SCRIPT_URL")
		if umamiScriptURL == "" {
			umamiScriptURL = "https://analytics.umami.is/script.js"
		}
		umamiScriptURL, validURL := validatedAnalyticsScriptURL(umamiScriptURL)
		if validID && validURL {
			analyticsInjectBuilder.WriteString("<script defer src=\"")
			analyticsInjectBuilder.WriteString(html.EscapeString(umamiScriptURL))
			analyticsInjectBuilder.WriteString("\" data-website-id=\"")
			analyticsInjectBuilder.WriteString(html.EscapeString(umamiSiteID))
			analyticsInjectBuilder.WriteString("\"></script>")
		} else {
			common.SysError("invalid Umami analytics configuration; injection skipped")
		}
	}
	analyticsInjectBuilder.WriteString("<!--Umami QuantumNous-->\n")
	analyticsInject := []byte(analyticsInjectBuilder.String())
	placeholder := []byte("<!--umami-->\n")
	indexPage = bytes.ReplaceAll(indexPage, placeholder, analyticsInject)
	classicIndexPage = bytes.ReplaceAll(classicIndexPage, placeholder, analyticsInject)
}

func InjectGoogleAnalytics() {
	analyticsInjectBuilder := &strings.Builder{}
	if os.Getenv("GOOGLE_ANALYTICS_ID") != "" {
		gaID, validID := validatedAnalyticsID(os.Getenv("GOOGLE_ANALYTICS_ID"))
		if validID {
			// Google Analytics 4 (gtag.js)
			analyticsInjectBuilder.WriteString("<script async src=\"https://www.googletagmanager.com/gtag/js?id=")
			analyticsInjectBuilder.WriteString(url.QueryEscape(gaID))
			analyticsInjectBuilder.WriteString("\"></script>")
			analyticsInjectBuilder.WriteString("<script>")
			analyticsInjectBuilder.WriteString("window.dataLayer = window.dataLayer || [];")
			analyticsInjectBuilder.WriteString("function gtag(){dataLayer.push(arguments);}")
			analyticsInjectBuilder.WriteString("gtag('js', new Date());")
			analyticsInjectBuilder.WriteString("gtag('config', '")
			analyticsInjectBuilder.WriteString(gaID)
			analyticsInjectBuilder.WriteString("');")
			analyticsInjectBuilder.WriteString("</script>")
		} else {
			common.SysError("invalid Google Analytics configuration; injection skipped")
		}
	}
	analyticsInjectBuilder.WriteString("<!--Google Analytics QuantumNous-->\n")
	analyticsInject := []byte(analyticsInjectBuilder.String())
	placeholder := []byte("<!--Google Analytics-->\n")
	indexPage = bytes.ReplaceAll(indexPage, placeholder, analyticsInject)
	classicIndexPage = bytes.ReplaceAll(classicIndexPage, placeholder, analyticsInject)
}

func InitResources() error {
	// Initialize resources here if needed
	// This is a placeholder function for future resource initialization
	err := godotenv.Load(".env")
	if err != nil {
		if common.DebugEnabled {
			common.SysLog("No .env file found, using default environment variables. If needed, please create a .env file and set the relevant variables.")
		}
	}

	// 加载环境变量
	common.InitEnv()

	logger.SetupLogger()

	// Initialize model settings
	ratio_setting.InitRatioSettings()

	service.InitHttpClient()

	service.InitTokenEncoders()
	tokenSecretConfig, err := common.LoadTokenSecretConfig()
	if err != nil {
		return fmt.Errorf("load token secret configuration: %w", err)
	}
	if err := service.ConfigureTokenSecretManager(tokenSecretConfig); err != nil {
		return fmt.Errorf("initialize token secret manager: %w", err)
	}

	// Initialize SQL Database
	err = model.InitDB()
	if err != nil {
		common.FatalLog("failed to initialize database: " + err.Error())
		return err
	}
	if tokenSecretConfig.Phase == common.TokenSecretPhaseCipherOnly {
		manager, managerErr := service.CurrentTokenSecretManager()
		if managerErr != nil {
			return managerErr
		}
		prepareCipherOnly := service.ValidateCipherOnlyTokenSecrets
		if common.IsMasterNode {
			prepareCipherOnly = service.PrepareCipherOnlyTokenSecrets
		}
		if err := prepareCipherOnly(context.Background(), model.DB, manager, common.GetEnvOrDefault("TOKEN_SECRET_BACKFILL_BATCH_SIZE", 100)); err != nil {
			return fmt.Errorf("prepare cipher-only token storage: %w", err)
		}
	}
	if err = authz.Init(model.DB); err != nil {
		common.FatalLog("failed to initialize authorization: " + err.Error())
		return err
	}

	model.CheckSetup()

	// Initialize options, should after model.InitDB()
	model.InitOptionMap()
	if err := service.EnsureAnnouncementMediaDirectory(); err != nil {
		return fmt.Errorf("initialize announcement media directory: %w", err)
	}

	// 清理旧的磁盘缓存文件
	common.CleanupOldCacheFiles()

	// Initialize SQL Database
	err = model.InitLogDB()
	if err != nil {
		return err
	}

	// Initialize Redis
	err = common.InitRedisClient()
	if err != nil {
		return err
	}

	perfmetrics.Init()

	// 启动系统监控
	common.StartSystemMonitor()

	// Initialize i18n
	err = i18n.Init()
	if err != nil {
		common.SysError("failed to initialize i18n: " + err.Error())
		// Don't return error, i18n is not critical
	} else {
		common.SysLog("i18n initialized with languages: " + strings.Join(i18n.SupportedLanguages(), ", "))
	}
	// Register user language loader for lazy loading
	i18n.SetUserLangLoader(model.GetUserLanguage)

	// Load custom OAuth providers from database
	err = oauth.LoadCustomProviders()
	if err != nil {
		common.SysError("failed to load custom OAuth providers: " + err.Error())
		// Don't return error, custom OAuth is not critical
	}

	return nil
}
