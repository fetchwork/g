package main

import (
	"caf/function"
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	docs "caf/docs"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title MFDC Call analytics and filters
// @version 1.0
// @description Swagger API for Golang Project MFDC CAF
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-MFDC-Key
func main() {

	config := function.GetConfig()

	var router *gin.Engine

	if !config.API.DebugMode {
		gin.SetMode(gin.ReleaseMode)
		router = gin.New()
	} else {
		router = gin.Default()
	}

	docs.SwaggerInfo.BasePath = function.SwaggerAPIpath

	// Middleware для CORS
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, PUT, PATCH")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Accept, X-MFDC-Key, Range, Connection")

		// Обработка предварительных запросов (preflight)
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	router.Use(function.DBMiddleware()) // Подключаем middleware для базы данных

	teams := router.Group("/teams")
	{
		teams.POST("/add", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.AddTeam(db.(*sqlx.DB), c)
		})
		teams.GET("/list", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.TeamsList(db.(*sqlx.DB), c)
		})
		teams.PATCH("/edit/:id", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.TeamEdit(db.(*sqlx.DB), c)
		})
		teams.DELETE("/delete/:id", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.TeamDelete(db.(*sqlx.DB), c)
		})
	}

	blacklist := router.Group("/blacklist")
	{
		blacklist.POST("/view", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.GetBlacklist(db.(*sqlx.DB), c)
		})
		blacklist.DELETE("/delete/:id", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.BLDelete(db.(*sqlx.DB), c)
		})
		blacklist.POST("/add", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.AddBL(db.(*sqlx.DB), c)
		})
	}

	router.POST("/:id/members", function.CheckLoop(), func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		function.ReceiveMembers(db.(*sqlx.DB), c)
	})

	router.GET("/webhooks/:type/:number", function.CheckUserAuth(), func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		function.CallHook(db.(*sqlx.DB), c)
	})

	router.GET("/webhooks/recheck/:number", function.CheckUserAuth(), func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		function.RecheckNumberHook(db.(*sqlx.DB), c)
	})

	router.POST("/logs", function.CheckUserAuth(), func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		function.GetLogs(db.(*sqlx.DB), c)
	})

	router.GET("/config/reload", function.CheckUserAuth(), function.UpdateConfig)

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Обработчик для несуществующих маршрутов
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "404 Not Found"})
	})

	srv := &http.Server{
		Addr:    config.API.Bind,
		Handler: router.Handler(),
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			function.ErrLog.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Подключение к базе данных
	db, err := function.PGConnect("")
	if err != nil {
		function.ErrLog.Printf("Failed to connect to PostgreSQL: %s", err)
		return
	}
	defer db.Close()

	// Создаём структуру БД если нет
	err = function.CreateTables(db)
	if err != nil {
		function.ErrLog.Printf("Error creating structure tables: %v", err)
	}

	// Запускаем мониторинг в отдельной горутине
	go function.MonitorConfigReload(ctx)

	router.GET("/runmethod/stat", function.CheckUserAuth(), func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		function.RunCompareNumberToStat(db.(*sqlx.DB), c, ctx)
	})

	// Если это не дополнительная нода сервиса
	if !config.API.SlaveNode {

		// Запуск уведомления на email об отфильтрованных номерах
		go function.StartFilteredNotify(db, ctx)

		// Запуск функции проверки статистики номеров
		go function.StartCompareNumberToStat(db, ctx, "")

		// Запуск функции проверки номеров для блокирования по стратегии unsuccessful
		go function.StartCheckNumberForBlockByUnsuccessful(db, ctx)

		// Запуск функции проверки номеров для блокирования по стратегии cause
		go function.StartCheckNumberForBlockByCause(db, ctx)

		// Запуск функции проверки заблокированных номеров если при изменений данных владельца номера была запрошена дополнительная проверка на сутки
		go function.StartRecheckNumberForBlockByUnsuccessful(db, ctx)

		// Ежедневная очистка успешных звонков за сутки
		go function.JOBClearTodaySuccessCall(db, ctx)

	}

	// Ожидание сигнала завершения
	<-quit

	// Отменяем контекст для завершения периодических задач
	cancel()

	// Установка таймаута для завершения сервера
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()

	// Завершение работы сервера
	if err := srv.Shutdown(shutdownCtx); err != nil {
		function.ErrLog.Fatal("MFDC CAА server Shutdown:", err)
	}

	select {
	case <-shutdownCtx.Done():
		function.OutLog.Println("MFDC CAА server halted")
	}
}
