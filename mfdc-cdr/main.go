package main

import (
	"cdr-api/function"
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	docs "cdr-api/docs"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title MFDC CDR API
// @version 1.0
// @description Swagger API for Golang Project MFDC CDR
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

	docs.SwaggerInfo.BasePath = function.APIPath

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

	router.POST("/list", function.CheckUserAuth(), func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		function.GetList(db.(*sqlx.DB), c)
	})

	router.POST("/csc", function.CheckUserAuth(), func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		function.CheckSuccessCallByNumber(db.(*sqlx.DB), c)
	})

	router.GET("/call/:id", function.CheckUserAuth(), func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		function.GetCall(db.(*sqlx.DB), c)
	})

	router.GET("/file/:id", function.CheckDownloadAuth(), func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		function.GetFile(db.(*sqlx.DB), c)
	})

	tags := router.Group("/tags")
	{
		tags.POST("/add", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.AddTag(db.(*sqlx.DB), c)
		})
		tags.DELETE("/:id", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.DeleteTag(db.(*sqlx.DB), c)
		})
		tags.GET("/list", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.ListTags(db.(*sqlx.DB), c)
		})
		tags.PUT("/push", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.PushTag(db.(*sqlx.DB), c)
		})
	}

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

	if config.API_Webitel.Collect {
		// Запускаем периодическую задачу в отдельной горутине
		go function.StartAPI2DBTask(ctx)
	}

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

	// Запуск горутины для проверки первого числа месяца
	go function.CheckAndCreatePartition(ctx, db)

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
		function.ErrLog.Fatal("MFDC CDR API server Shutdown:", err)
	}

	select {
	case <-shutdownCtx.Done():
		function.OutLog.Println("MFDC CDR API server halted")
	}
}
