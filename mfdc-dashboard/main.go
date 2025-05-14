package main

import (
	"context"
	"dashboard/function"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	docs "dashboard/docs"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title MFDC Dashboard
// @version 1.0
// @description Swagger API for Golang Project MFDC Dashboard
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

	router.GET("/agents", function.CheckUserAuth(), func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		function.DashAgents(db.(*sqlx.DB), c)
	})

	router.GET("/calls", function.CheckUserAuth(), func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		dbWebitel, _ := function.CheckDBWebitel(c)
		function.DashCalls(db.(*sqlx.DB), dbWebitel.(*sqlx.DB), c)
	})

	router.GET("/spins", function.CheckUserAuth(), func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		dbWebitel, _ := function.CheckDBWebitel(c)
		function.DashSpins(db.(*sqlx.DB), dbWebitel.(*sqlx.DB), c)
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

	// Ожидание сигнала завершения
	<-quit

	// Отменяем контекст для завершения периодических задач
	cancel()

	// Установка таймаута для завершения сервера
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()

	// Завершение работы сервера
	if err := srv.Shutdown(shutdownCtx); err != nil {
		function.ErrLog.Fatal("MFDC Dashboard server Shutdown:", err)
	}

	select {
	case <-shutdownCtx.Done():
		function.OutLog.Println("MFDC Dashboard server halted")
	}
}
