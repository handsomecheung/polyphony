package handlerip

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	headerClientIP   = "CF-Connecting-IP"
	headerRequstHost = "X-Forwarded-Host"
)

type HandlerIP struct {
	deniedNets []*net.IPNet
	deniedText string
}

func readIPs(filepath string) ([]string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	var ipFilter1IPs []string
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ipFilter1IPs = append(ipFilter1IPs, line)
	}

	return ipFilter1IPs, nil
}

func New(filepath string, returnText string) (*HandlerIP, error) {
	deniedIPs, err := readIPs(filepath)
	if err != nil {
		return nil, err
	}

	deniedNets := make([]*net.IPNet, 0, len(deniedIPs))
	for _, ipStr := range deniedIPs {
		_, ipNet, err := net.ParseCIDR(ipStr)
		if err != nil {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				return nil, fmt.Errorf("HandlerIP Invalid IP/CIDR address: %s", ipStr)
			}
			mask := net.CIDRMask(len(ip)*8, len(ip)*8)
			ipNet = &net.IPNet{IP: ip, Mask: mask}
		}

		deniedNets = append(deniedNets, ipNet)
	}

	log.Printf("HandlerIP Using header for client ip: %s", headerClientIP)
	log.Printf("HandlerIP Blocking IP Nets: %v", deniedNets)

	return &HandlerIP{
		deniedNets: deniedNets,
	}, nil
}

func (i *HandlerIP) IPFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIPStr := c.Request.Header.Get(headerClientIP)
		clientHostStr := c.Request.Header.Get(headerRequstHost)
		log.Printf("IPFilter incoming request. request host: %s, client ip: %s", clientHostStr, clientIPStr)

		if clientIPStr == "" {
			c.Next()
			return
		}

		clientIP := net.ParseIP(clientIPStr)
		if clientIP == nil {
			c.Next()
			return
		}

		for _, deniedNet := range i.deniedNets {
			if deniedNet.Contains(clientIP) {
				log.Printf("IPFilter Blocking request host %s from client IP: %s, Return 403 Forbidden", clientHostStr, clientIPStr)
				c.String(http.StatusForbidden, i.deniedText)
				c.Abort()
				return
			}
		}

		c.Next()
	}
}
