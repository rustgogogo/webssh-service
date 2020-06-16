package main

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"net/http"
	"os"
)

var logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

func init() {
	logger.Level(zerolog.DebugLevel)
}

func GetLogLevel(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"level": logger.GetLevel().String(),
	})
}

/**
设置为debug级别
*/
func SetLogDebugLevel(c *gin.Context) {
	logger.Level(zerolog.DebugLevel)
	c.String(http.StatusOK, "OK")
}

/**
设置为info级别
*/
func SetLogInfoLevel(c *gin.Context) {
	logger.Level(zerolog.InfoLevel)
	c.String(http.StatusOK, "OK")
}
