package main

import (
	"billing-api/function"
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	docs "billing-api/docs"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title MFDC Billing API
// @version 1.0
// @description Swagger API for Golang Project MFDC
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

	//docs.SwaggerInfo.BasePath = "/"
	docs.SwaggerInfo.BasePath = "/api/billing"

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

	providers := router.Group("/providers")
	{
		providers.GET("/all", function.CheckUserAuth(), function.GetProviders)
		providers.GET("/:id", function.CheckUserAuth(), function.GetProvidersByID)
		providers.DELETE("/:id", function.CheckUserAuth(), function.CheckAdminLevel(), function.DeleteProviderByID)
		providers.DELETE("/:id/address", function.CheckUserAuth(), function.CheckAdminLevel(), function.DeleteAddressByAddress)
		providers.POST("/add", function.CheckUserAuth(), function.CheckAdminLevel(), function.AddProvider)
		providers.POST(":id/add", function.CheckUserAuth(), function.CheckAdminLevel(), function.AddProviderIP)
		providers.PUT(":id/edit", function.CheckUserAuth(), function.CheckAdminLevel(), function.EditProvider)
	}
	routes := router.Group("/routes")
	{
		routes.GET("/all", function.CheckUserAuth(), function.CheckAdminLevel(), function.GetRoutes)
		routes.POST("/add", function.CheckUserAuth(), function.CheckAdminLevel(), function.AddRoute)
		routes.PATCH(":id/edit", function.CheckUserAuth(), function.CheckAdminLevel(), function.EditRoute)
		routes.DELETE(":id", function.CheckUserAuth(), function.CheckAdminLevel(), function.DeleteRouteByID)
	}
	cdr := router.Group("/cdr")
	{
		cdr.POST("/list", func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.GetCDR(db.(*sqlx.DB), c)
		})
		cdr.POST(":id/report", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.GetSumm(db.(*sqlx.DB), c)
		})
	}

	export := router.Group("/export")
	{
		export.GET("/list", func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.GetExports(db.(*sqlx.DB), c)
		})
		export.GET("/download/:id", func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.DownloadCSVHandler(db.(*sqlx.DB), c)
		})
		export.DELETE("/:id", func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.DeleteExportTaskByID(db.(*sqlx.DB), c)
		})
	}

	router.POST("/csc", function.CheckUserAuth(), func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		function.CheckSuccessCallByNumber(db.(*sqlx.DB), c)
	})

	router.GET("/config/reload", function.CheckUserAuth(), function.CheckAdminLevel(), function.UpdateConfig)

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Обработчик для несуществующих маршрутов
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "404 Not Found"})
	})

	db, err := function.PGConnect()
	if err != nil {
		function.ErrLog.Printf("Failed to connect to PostgreSQL: %s", err)
		return
	}
	defer db.Close()

	srv := &http.Server{
		Addr:    config.API.Bind,
		Handler: router.Handler(),
	}

	// Запускаем сервер в отдельной горутине
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			function.ErrLog.Fatalf("listen: %s\n", err)
		}
	}()

	// Ожидание сигналов завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	/*
		// Загружаем локацию Europe/Moscow
		loc, err := time.LoadLocation("Europe/Moscow")
		if err != nil {
			function.ErrLog.Println("Failed to load time locations", err)
			return
		}
		startDate := time.Date(2024, 11, 30, 0, 0, 0, 0, loc) // Задаем начальную дату
		go function.StartAggregation(startDate, ctx, *loc)

	*/
	// Запускаем мониторинг в отдельной горутине
	go function.MonitorConfigReload(ctx)

	// Функция ежесуточного суммирования данных за предыдущие сутки
	go function.TaskAgregateSum(ctx)

	// Функция запуска проверки задач и выполнения экспорта данных в CSV
	go function.StartCSVCheker(db, ctx)

	function.OutLog.Println("MFDC Billing API running")

	// Ожидание сигнала завершения
	<-quit

	// Отменяем контекст для завершения периодических задач
	cancel()

	// Завершение работы сервера
	if err := srv.Shutdown(ctx); err != nil {
		function.ErrLog.Fatal("MFDC Billing API exiting:", err)
	}

	// Ждем завершения контекста
	select {
	case <-ctx.Done():
		time.Sleep(3 * time.Second)
	}

	function.OutLog.Println("MFDC Billing API halted")

}
