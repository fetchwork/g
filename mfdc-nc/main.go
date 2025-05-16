package main

import (
	"context"
	"nc/function"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	docs "nc/docs"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title MFDC Number changer
// @version 1.0
// @description Swagger API for Golang Project MFDC NC
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

	numbers := router.Group("/numbers")
	{
		numbers.POST("/upload", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.UploadNumbers(db.(*sqlx.DB), c)
		})
		numbers.GET("/info/:number", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.NumberInfo(db.(*sqlx.DB), c)
		})
		numbers.GET("/routing/:number", func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.NumberTeamInfo(db.(*sqlx.DB), c)
		})
		numbers.GET("/list/:pool_id", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.NumbersInPool(db.(*sqlx.DB), c)
		})
		numbers.PATCH("/exclusion", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.NumberExclusion(db.(*sqlx.DB), c)
		})
	}

	pools := router.Group("/pools")
	{
		pools.GET("/list", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.PoolsList(db.(*sqlx.DB), c)
		})
		pools.GET("/:id/activate", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.ActivatePoolManual(db.(*sqlx.DB), c)
		})
		pools.GET("/:id/deactivate", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.DeactivatePoolManual(db.(*sqlx.DB), c)
		})
		pools.DELETE("/delete/:id", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.PoolDelete(db.(*sqlx.DB), c)
		})
		pools.POST("/numsmove", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.RedistributionPools(db.(*sqlx.DB), c)
		})
	}

	schedule := router.Group("/schedule")
	{
		schedule.POST("/add", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.AddSchedule(db.(*sqlx.DB), c)
		})
		schedule.GET("/list", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.SchedulsList(db.(*sqlx.DB), c)
		})
		schedule.PATCH("/edit/:id", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.ScheduleEdit(db.(*sqlx.DB), c)
		})
		schedule.DELETE("/delete/:id", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.ScheduleDelete(db.(*sqlx.DB), c)
		})
	}

	teams := router.Group("/teams")
	{
		teams.GET("/list", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.TeamsList(db.(*sqlx.DB), c)
		})
		teams.GET("/:id/rotate", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.TeamNumberRotate(db.(*sqlx.DB), c)
		})
		teams.GET("/activenums", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.ActiveTeamsNumbers(db.(*sqlx.DB), c)
		})
		teams.POST("/daynums", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.TeamsDayNumbers(db.(*sqlx.DB), c)
		})
		teams.POST("/add", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.AddTeam(db.(*sqlx.DB), c)
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

	vendors := router.Group("/vendors")
	{
		vendors.GET("/list", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.VendorsList(db.(*sqlx.DB), c)
		})
		vendors.POST("/add", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.AddVendor(db.(*sqlx.DB), c)
		})
		vendors.PATCH("/edit/:id", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.VendorEdit(db.(*sqlx.DB), c)
		})
		vendors.DELETE("/delete/:id", function.CheckUserAuth(), func(c *gin.Context) {
			db, _ := function.CheckDB(c)
			function.VendorDelete(db.(*sqlx.DB), c)
		})
	}

	router.GET("/subpools/next", function.CheckUserAuth(), func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		function.ActivateSubPoolManual(db.(*sqlx.DB), c)
	})

	router.GET("/subpools/:pool_id/next", function.CheckUserAuth(), func(c *gin.Context) {
		db, _ := function.CheckDB(c)
		function.ActivateSubPoolManualForPool(db.(*sqlx.DB), c)
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

	// Синк с сервисом VC
	go function.StartVCSync(ctx, db)

	if config.Rotate.EnableRotation {
		// Запуск ротации расписаний
		go function.StartRotationSchedule(ctx, db)

		// Запуск ротации номеров пул > сабпул > номер
		go function.StartPeriodicRotation(ctx, db)

		// Ежедневная активация сабпулов
		go function.SubPoolActivate(ctx, db)

		//Тестовая функция активации сабпула
		//go function.TestStartSubPoolActivateTask(ctx, db)
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
		function.ErrLog.Fatal("MFDC NC server Shutdown:", err)
	}

	select {
	case <-shutdownCtx.Done():
		function.OutLog.Println("MFDC NC server halted")
	}
}
