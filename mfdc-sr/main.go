package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sr-api/function"
	"syscall"
	"time"

	docs "sr-api/docs"

	"github.com/gin-gonic/gin"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title MFDC Spin regulator API
// @version 1.0
// @description Swagger API for Golang Project MFDC SR
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-MFDC-Key
func main() {
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()
	//docs.SwaggerInfo.BasePath = "/"
	docs.SwaggerInfo.BasePath = "/api/sr"

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
	router.PATCH("/edit/:id", function.CheckUserAuth(), function.ChangeQueue)
	router.POST("/logs", function.CheckUserAuth(), function.GetLogs)

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
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			function.ErrLog.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall. SIGKILL but can"t be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	function.OutLog.Println("MFDC SR API shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		function.ErrLog.Fatal("MFDC SR API server Shutdown:", err)
	}
	// catching ctx.Done(). timeout of 5 seconds.
	select {
	case <-ctx.Done():
		function.OutLog.Println("timeout of 5 seconds.")
	}
	function.OutLog.Println("MFDC SR API server exiting")

	//router.Run(config.API.Bind)
}
