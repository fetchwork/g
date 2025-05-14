package main

import (
	"auth-api/function"
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	docs "auth-api/docs"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title MFDC Auth API
// @version 1.0
// @description Swagger API for Golang Project MFDC
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-MFDC-Key
func main() {

	//gin.SetMode(gin.ReleaseMode)

	router := gin.Default()
	docs.SwaggerInfo.BasePath = "/"
	//docs.SwaggerInfo.BasePath = "/api/auth"

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

	router.Use(function.DBMiddleware()) // Подключаем middleware для базы данных

	users := router.Group("/users")
	{
		users.POST("/add", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.UserAdd(db.(*sqlx.DB), c)
		})

		users.GET("/list", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.UserList(db.(*sqlx.DB), c)
		})

		users.PATCH("/:id/edit", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.UserEdit(db.(*sqlx.DB), c)
		})

		users.GET("/info", function.CheckUserAuth(), func(c *gin.Context) {
			function.GetUserInfo(c)
		})

		users.DELETE("/:id/delete", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.UserDelete(db.(*sqlx.DB), c)
		})

		users.GET("/:id/info", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.GetUserNameByUID(db.(*sqlx.DB), c)
		})
	}

	router.GET("/teams/list", function.CheckUserAuth(), func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		function.TeamsList(db.(*sqlx.DB), c)
	})

	router.POST("/login", func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		function.AuthUser(db.(*sqlx.DB), c)
	})

	router.GET("/config/reload", function.CheckUserAuth(), function.UpdateConfig)

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Обработчик для несуществующих маршрутов
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"status": "failed", "error": "URL 404 Not Found"})
	})

	config := function.GetConfig()

	srv := &http.Server{
		Addr:    config.API.Auth_bind,
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
	function.OutLog.Println("MFDC Auth API shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		function.ErrLog.Fatal("MFDC Auth API server Shutdown:", err)
	}
	// catching ctx.Done(). timeout of 3 seconds.
	select {
	case <-ctx.Done():
		function.OutLog.Println("timeout of 3 seconds.")
	}
	function.OutLog.Println("MFDC Auth API server exiting")

}
