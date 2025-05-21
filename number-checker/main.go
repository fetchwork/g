package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	docs "number-checker/docs"
	function "number-checker/function"
)

// @title MFDC Number Checker API
// @version 1.0
// @description Swagger API for Golang Project MFDC VC
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-MFDC-Key
func main() {
	// Загружаем конфигурацию
	config := function.GetConfig()

	// Режим Gin
	var router *gin.Engine

	if !config.API.DebugMode {
		gin.SetMode(gin.ReleaseMode)
		router = gin.New()
	} else {
		router = gin.Default()
	}

	docs.SwaggerInfo.BasePath = "/api/nchecker" //"/api/nchecker"

	// Роуты

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

	db, _ := function.MySQLConnect()
	router.Use(function.MySQLMiddleware(db))

	router.POST("/check-numbers", function.CheckUserAuth(), func(c *gin.Context) {
		db, _ := function.CheckMySQL(c)
		function.CheckNumbers(db.(*sqlx.DB), c)
	})

	// Swagger
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 404
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "404 Not Found"})
	})

	// Сервер
	srv := &http.Server{
		Addr:    config.API.Bind,
		Handler: router.Handler(),
	}

	// Запуск сервера в отдельной горутине
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			function.ErrLog.Fatalf("listen: %s\n", err)
		}
	}()

	// Ожидание сигнала завершения
	// Ожидание сигналов завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go function.MonitorConfigReload(ctx)

	function.OutLog.Println("MFDC NumberCheker API running")

	<-quit
	cancel()

	if err := srv.Shutdown(ctx); err != nil {
		function.ErrLog.Fatal("MFDC NumberCheker API exiting:", err)
	}

	// Ждем завершения контекста
	select {
	case <-ctx.Done():
		time.Sleep(3 * time.Second)
	}

	function.OutLog.Println("MFDC NumberCheker API halted")
}
