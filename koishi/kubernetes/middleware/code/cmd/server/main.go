package main

import (
	"log"
	"net/http"
	"os"

	"koishi/kubernetes/middleware/common/internal/handler/handlerip"
	"koishi/kubernetes/middleware/common/internal/handler/handlerkv"

	"github.com/gin-gonic/gin"
)

func main() {
	ipFilter1Text := os.Getenv("IP_FILTER1_TEXT")
	if ipFilter1Text == "" {
		log.Fatal("IP_FILTER1_TEXT is not set")
	}
	ipFilter1File := os.Getenv("IP_FILTER1_FILE")
	if ipFilter1File == "" {
		log.Fatal("IP_FILTER1_FILE is not set")
	}

	hip1, err := handlerip.New(ipFilter1File, ipFilter1Text)
	if err != nil {
		log.Fatalf("Failed to create IP handler: %v", err)
	}

	kvFilter1Key := os.Getenv("KV_FILTER1_KEY")
	if kvFilter1Key == "" {
		log.Fatal("KV_FILTER1_KEY is not set")
	}
	kvFilter1Value := os.Getenv("KV_FILTER1_VALUE")
	if kvFilter1Value == "" {
		log.Fatal("KV_FILTER1_VALUE is not set")
	}

	hkv1, err := handlerkv.New(kvFilter1Key, kvFilter1Value)
	if err != nil {
		log.Fatalf("Failed to create KV handler: %v", err)
	}

	r := gin.Default()

	r.GET("/ipfilter1", hip1.IPFilter(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	r.GET("/kvfilter1", hkv1.KVFilter(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	addr := ":8080"
	if err := r.Run(addr); err != nil {
		log.Fatalf("Could not listen on %s: %v", addr, err)
	}
}
