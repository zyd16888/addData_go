package logger

import (
	"fmt"
	"log"
	"os"
)

type Logger struct {
	Info  *log.Logger
	Error *log.Logger
	Debug *log.Logger
}

func NewLogger(infoFileName, errorFileName, debugFileName string) (*Logger, error) {
	infoFile, err := os.OpenFile(infoFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	errorFile, err := os.OpenFile(errorFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	debugFile, err := os.OpenFile(debugFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error opening file: %v", err)
		return nil, err
	}

	infoLogger := log.New(infoFile, "INFO: ", log.LstdFlags)
	errorLogger := log.New(errorFile, "ERROR: ", log.LstdFlags)
	debugLogger := log.New(debugFile, "DEBUG: ", log.LstdFlags)

	return &Logger{
		Info:  infoLogger,
		Error: errorLogger,
		Debug: debugLogger,
	}, nil
}
