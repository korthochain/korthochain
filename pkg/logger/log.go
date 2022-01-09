package logger

import (
	"log"
	"os"
)

var (
	WarnLogger  *log.Logger
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
)

func init() {
	InfoLogger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
	WarnLogger = log.New(os.Stdout, "WARN: ", log.Ldate|log.Ltime)
	ErrorLogger = log.New(os.Stdout, "EROR: ", log.Ldate|log.Ltime)
}
