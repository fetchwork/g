package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"vc-api/function"

	docs "vc-api/docs"

	"github.com/gin-gonic/gin"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title MFDC VC API
// @version 1.0
// @description Swagger API for Golang Project MFDC VC
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-MFDC-Key
func main() {
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()
	//docs.SwaggerInfo.BasePath = "/"
	docs.SwaggerInfo.BasePath = "/api/vc"

	// Middleware для CORS
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, PUT, PATCH")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Accept, X-MFDC-Key")

		// Обработка предварительных запросов (preflight)
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	router.GET("/list", function.CheckUserAuth(), function.GetList)
	router.PATCH("/edit/:id", function.CheckUserAuth(), function.ChangeVendor)

	router.GET("/config/reload", function.CheckUserAuth(), function.UpdateConfig)

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Обработчик для несуществующих маршрутов
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "404 Not Found"})
	})

	config := function.GetConfig()

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

	// Запуск синхронизации с Webitel
	go function.StartCheckActualSub(ctx, db)

	// Ожидание сигнала завершения
	<-quit

	// Отменяем контекст для завершения периодических задач
	cancel()

	// Установка таймаута для завершения сервера
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()

	// Завершение работы сервера
	if err := srv.Shutdown(shutdownCtx); err != nil {
		function.ErrLog.Fatal("MFDC VC server Shutdown:", err)
	}

	select {
	case <-shutdownCtx.Done():
		function.OutLog.Println("MFDC VC server halted")
	}
}
