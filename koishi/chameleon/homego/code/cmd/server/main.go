package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"koishi/chameleon/homego/internal/auth"
	"koishi/chameleon/homego/internal/config"
	"koishi/chameleon/homego/internal/handlers/handleradmin"
	"koishi/chameleon/homego/internal/handlers/handlerbase"
	"koishi/chameleon/homego/internal/handlers/handlerlinux"
	"koishi/chameleon/homego/internal/handlers/handlerps1"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/handsomecheung/mb64"
)

func main() {
	mberr := mb64.SetEncoding(os.Getenv("MB64_KEY"))
	if mberr != nil {
		fmt.Println(mberr)
		os.Exit(1)
	}

	rand.New(rand.NewSource(time.Now().UnixNano()))

	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	authService := auth.NewAuthService(cfg)

	handlerbase := handlerbase.NewHandler(cfg, authService)

	r := gin.Default()

	// Load templates
	r.LoadHTMLGlob("templates/*")

	// Serve static files
	r.Use(static.Serve("/static", static.LocalFile("static", false)))
	r.Use(static.Serve("/wasm", static.LocalFile("wasm", false)))
	r.StaticFile("/favicon.ico", "static/favicon.ico")

	r.Static("/timages", "/tmp/images")

	r.GET("/", handlerbase.HomePage)
	r.GET("/s", handlerbase.ShowKeyPage)
	r.GET("/auth", handlerbase.Auth)

	hps1 := handlerps1.NewHandler(handlerbase)
	ps1 := r.Group(handlerps1.PathPrefix)
	{
		ps1.GET("/", hps1.IndexPage)
		ps1.GET("/article/:encname", hps1.ShowArticlePage)
		ps1.GET("/article-ranks", hps1.RanksPage)

		ps1.GET("/try/*name", hps1.Try)
		ps1.GET("/queue", hps1.QueuePage)
		ps1.GET("/queue/status", hps1.QueueStatus)
		ps1.DELETE("/queue", hps1.QueueLeave)

	}

	hlinux := handlerlinux.NewHandler(handlerbase)
	linux := r.Group(handlerlinux.PathPrefix)
	{
		linux.GET("/", hlinux.IndexPage)
		linux.GET("/article/:encname", hlinux.ShowArticlePage)
		linux.GET("/article-ranks", hlinux.RanksPage)

		linux.GET("/try/*name", hlinux.Try)
		linux.GET("/queue", hlinux.QueuePage)
		linux.GET("/queue/status", hlinux.QueueStatus)
		linux.DELETE("/queue", hlinux.QueueLeave)
	}

	// private API, can not be accessed from internet. used by deactivate
	hadmin := handleradmin.NewHandler(handlerbase, hps1, hlinux)
	admin := r.Group(handleradmin.PathPrefix)
	{
		admin.GET("/links", hadmin.LinksPage)
		admin.GET("/queues", hadmin.QueuePage)
		admin.DELETE("/session/all", hadmin.DeleteSessions)
	}

	// Start server
	if err := r.Run(cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
