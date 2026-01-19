package k8s

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func RestartDeployment(name string) {
	go func(name string) {
		url := fmt.Sprintf("http://deactivate/kubernetes/deployment/%s", name)
		req, err := http.NewRequest("PUT", url, nil)
		if err != nil {
			log.Printf("Failed to create restart request for %s: %v", name, err)
			return
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Failed to restart %s: %v", name, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			log.Printf("Successfully requested restart for %s", name)
		} else {
			log.Printf("Restart request for %s returned status: %d", name, resp.StatusCode)
		}
	}(name)
}
