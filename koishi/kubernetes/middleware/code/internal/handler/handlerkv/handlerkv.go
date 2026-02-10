package handlerkv

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	headerRequestHost = "X-Forwarded-Host"
)

type HandlerKV struct {
	headerKey    string
	allowedValue string
}

func New(headerKey string, allowedValue string) (*HandlerKV, error) {
	log.Printf("HandlerKV Using header: %s", headerKey)

	return &HandlerKV{
		headerKey:    headerKey,
		allowedValue: allowedValue,
	}, nil

}

func (i *HandlerKV) KVFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		headerValue := c.Request.Header.Get(i.headerKey)
		requestHost := c.Request.Header.Get(headerRequestHost)
		log.Printf("KVFilter incoming request. reqeust host: %s, header value: %s", requestHost, headerValue)

		if headerValue == "" {
			// local access directly (not via cloudflare)
			c.Next()
			return
		}

		if headerValue != i.allowedValue {
			log.Printf("KVFilter Blocking request %s with value: %s, Return 403 Forbidden", requestHost, headerValue)
			c.String(http.StatusNotFound, "Not Found")
			c.Abort()
			return
		}

		c.Next()
	}
}
