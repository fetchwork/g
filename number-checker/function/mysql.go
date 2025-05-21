package function

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// MySQLConnect создаёт пул соединений с MySQL
func MySQLConnect() (*sqlx.DB, error) {
	cfg := GetConfig()

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		cfg.MySQL.User,
		cfg.MySQL.Password,
		cfg.MySQL.Host,
		cfg.MySQL.Port,
		cfg.MySQL.DBName,
	)

	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		ErrLog.Printf("Failed to connect to MySQL: %v", err)
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		ErrLog.Printf("Failed to ping MySQL: %v", err)
		return nil, err
	}

	return db, nil
}

// MySQLMiddleware передаёт готовое подключение к MySQL в контекст
func MySQLMiddleware(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	}
}

// CheckMySQL извлекает подключение из контекста
func CheckMySQL(c *gin.Context) (any, error) {
	db, exists := c.Get("db")
	if !exists {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "database not found"})
		return nil, fmt.Errorf("database not found")
	}
	return db, nil
}
