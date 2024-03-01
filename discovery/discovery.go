package discovery

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"
)

// Server represents a server's host and port.
type Server struct {
	Host        string `json:"host"`
	Port        string `json:"port"`
	ServiceType string `json:"service_type"`
}

// Discovery discovers a server with the minimum request time.
func Discovery(host string, port int, serviceType string, timeout time.Duration) (string, int) {
	client := &http.Client{Timeout: timeout * time.Second}
	serviceURL := "http://" + host + ":" + strconv.Itoa(port) + "/api/v1.0/services"
	log.Printf("Requesting server list from: %s", serviceURL)

	// Request to get list of servers
	req, err := http.NewRequest("GET", serviceURL, nil)
	if err != nil {
		log.Printf("Server discovery error: can't create request for %s\n", serviceType)
		return "", 0
	}

	q := req.URL.Query()
	q.Add("service_type", serviceType)
	req.URL.RawQuery = q.Encode()

	log.Printf("Making request to URL: %s", req.URL.String())
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Server discovery error: can't get list of servers for %s\n", serviceType)
		return "", 0
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %s\n", err)
		return "", 0
	}

	var servers []Server
	if err := json.Unmarshal(body, &servers); err != nil {
		log.Printf("Error decoding server list for %s: %s\n", serviceType, err)
		return "", 0
	}

	var (
		serverHost      string
		serverPortInt   int
		minResponseTime = time.Duration(math.MaxInt64)
	)

	for _, server := range servers {
		portInt, err := strconv.Atoi(server.Port)
		if err != nil {
			log.Printf("Error converting port from string to int for host %s: %s\n", server.Host, err)
			continue
		}

		serverURL := "http://" + server.Host + ":" + server.Port + "/api/v1.0/health"
		start := time.Now()
		response, err := client.Get(serverURL)
		if err != nil {
			continue
		}
		response.Body.Close()

		responseTime := time.Since(start)
		if responseTime < minResponseTime {
			serverHost = server.Host
			serverPortInt = portInt
			minResponseTime = responseTime
			log.Printf("Current best: %s, time: %s\n", serverHost, minResponseTime)
		}
	}

	if serverHost == "" {
		log.Printf("Can't reach any %s or list of servers are empty\n", serviceType)
	} else {
		log.Printf("Nearest %s: %s, time: %s\n", serviceType, serverHost, minResponseTime)
	}

	return serverHost, serverPortInt
}
