// Command server 是 HeartEcho 的 Go 后端入口：
// 装配配置、日志、数据库与路由，然后启动 HTTP 服务并等待退出信号。
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/config"
	"github.com/nanirise/heart-echo/backend/internal/handler"
	"github.com/nanirise/heart-echo/backend/internal/service"
	"github.com/nanirise/heart-echo/backend/pkg/logger"
)

// shutdownTimeout 是收到退出信号后等待在途请求收尾的时限。
// Docker 默认给容器 10 秒再 SIGKILL，这里留一半余量。
const shutdownTimeout = 5 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		// 日志器还没建起来，配置阶段的错误只能直接写 stderr。
		fmt.Fprintf(os.Stderr, "server: 加载配置失败: %v\n", err)
		os.Exit(1)
	}

	appLogger := logger.New()
	defer func() { _ = appLogger.Sync() }()
	// 判定链路的日志走 zap.L()，不在这里设成全局就是 no-op，debug 全丢。
	zap.ReplaceGlobals(appLogger)

	db, err := connectDatabase(cfg)
	if err != nil {
		appLogger.Fatal("初始化数据库连接失败", zap.Error(err))
	}

	// 主动消息定时扫描（AGENTS §4.8：定时任务一律在 Go 侧）；
	// 与手动端点共用同一个 TriggerNow（spec §4）。
	jobCtx, cancelJob := context.WithCancel(context.Background())
	// [假设] 这里与 router.go 各建一个 ChatService / AIClient 实例：两条链路共享 db、
	// 不共享状态，各自演化不影响。代价是两套到 ai-service 的连接池（AIClient 用默认
	// http.Client，无连接数上限，可接受）。若将来 Week 3 的 emotion / memory 要复用
	// 同一个实例，再把 AIClient 提到这里建一次、由 NewRouter 传参。
	proactiveSvc := service.NewProactiveService(db, service.NewChatService(db, service.NewAIClient(cfg.AI)))
	proactiveJob := service.NewProactiveJob(proactiveSvc, cfg.ProactiveJobInterval, appLogger)
	go proactiveJob.Run(jobCtx)

	srv := &http.Server{
		// 不写 host 就是监听 0.0.0.0。容器里必须如此，
		// 绑成 127.0.0.1 会让同网络的其他容器连不上（AGENTS §4.10）。
		Addr:    ":" + cfg.Server.Port,
		Handler: handler.NewRouter(cfg, appLogger, db),
	}

	// ListenAndServe 会阻塞，放进单独 goroutine，
	// 主 goroutine 留着等退出信号。
	go func() {
		appLogger.Info("HTTP 服务启动", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// 端口被占用之类的启动期故障，必须让进程退出，
			// 否则表现为"容器在跑但没在监听"。
			appLogger.Fatal("HTTP 服务异常退出", zap.Error(err))
		}
	}()

	// SIGINT(Ctrl+C) / SIGTERM(docker stop)。
	// 等信号而不是直接 ListenAndServe，是为了让在途请求有机会收尾，
	// 否则每次重启都会留下一批被硬生生掐断的连接。
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	appLogger.Info("收到退出信号，开始关闭")

	// 先停后台定时任务再收 HTTP：Run 循环监听 jobCtx，cancel 后退出
	cancelJob()

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		appLogger.Error("优雅关闭超时，强制退出", zap.Error(err))
	}
}

// connectDatabase 初始化 GORM 连接。
//
// DisableAutomaticPing：启动时不 ping 数据库，连不上也让服务起得来，
// 由 /health 报告 database 状态（API_CONTRACT §11）。
// 这与 cmd/migrate 刻意相反——迁移必须连不上就立刻失败，Web 服务不该：
// 数据库晚起几秒，不该导致后端容器反复重启。
//
// 不调 model.AutoMigrate：建表由成员 3 的 cmd/migrate 负责，两边同时建会互相干扰。
func connectDatabase(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN()), &gorm.Config{
		DisableAutomaticPing: true,
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}
