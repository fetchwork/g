package function

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func MySQLConnect() (*sqlx.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		config.MySQL.User,
		config.MySQL.Password,
		config.MySQL.Host,
		config.MySQL.Port,
		config.MySQL.DBName,
	)

	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		ErrLog.Printf("Failed to connect to MySQL: %s", err)
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		ErrLog.Printf("Failed to ping MySQL: %s", err)
		return nil, err
	}

	return db, nil
}

func MySQLMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		db, err := MySQLConnect()
		if err != nil {
			ErrLog.Printf("Failed to connect to MySQL: %s", err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
			c.Abort()
			return
		}
		defer db.Close()
		c.Set("db", db)
		c.Next()
	}
}

func CheckMySQL(c *gin.Context) (any, error) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database not found"})
		return nil, fmt.Errorf("database not found")
	}
	return db, nil
}
