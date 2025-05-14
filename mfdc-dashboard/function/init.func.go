package function

import (
	"log"
	"os"
)

var (
	OutLog *log.Logger
	ErrLog *log.Logger
)

func init() {
	// Настройка логгера для stdout
	OutLog = log.New(os.Stdout, "", log.LstdFlags)

	// Настройка логгера для stderr
	ErrLog = log.New(os.Stderr, "", log.LstdFlags)
}
