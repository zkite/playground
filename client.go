package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"time"

	"hello/db"
	"hello/discovery"
	"hello/response"

	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
)

var addr = "161.184.221.236:8887"

func executeCommand(command string) (string, error) {
	cmd := exec.Command("bash", "-c", command)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stderr.String(), err
	}

	return stdout.String(), nil
}

type IncomingMessage struct {
	Request struct {
		Method    string `json:"method"`
		Arguments struct {
			Subscription struct {
				ID           string `json:"id"`
				SubscriberID string `json:"subscriber_id"`
				Topic        string `json:"topic"`
				NotifierID   string `json:"notifier_id"`
			} `json:"subscription"`
			Data struct {
				ActionType string `json:"action_type"`
				Command    string `json:"command"`
			} `json:"data"`
		} `json:"arguments"`
		CallID string `json:"call_id"`
	} `json:"request"`
	Response interface{} `json:"response"`
}

func getMACAddress() (string, error) {
	return "00:1B:44:11:3A:18", nil
}

func makeRequest(serverHost string, serverPort int, macAddress string) (response.ServerResponse, error) {
	var respObj response.ServerResponse

	u := url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", serverHost, serverPort),
		Path:   "/api/v1.0/adapter/" + macAddress + "/udpu",
	}

	resp, err := http.Get(u.String())
	if err != nil {
		return respObj, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return respObj, err
	}

	err = json.Unmarshal(body, &respObj)
	if err != nil {
		return respObj, err
	}

	jsonResponse, err := json.Marshal(respObj)
	if err != nil {
		log.Fatalf("Error occurred during marshalling. Error: %s", err.Error())
	}

	fmt.Println(string(jsonResponse))

	return respObj, nil
}

func subscribeToTopics(c *websocket.Conn, subscriberUID string) error {
	subscriptionMessage := map[string]interface{}{
		"request": map[string]interface{}{
			"method": "subscribe",
			"arguments": map[string]interface{}{
				"topics": []string{subscriberUID},
			},
			"call_id": "4140dd17a18c45db8a98ba155cbfa",
		},
		"response": nil,
	}
	message, err := json.Marshal(subscriptionMessage)
	if err != nil {
		return err
	}
	return c.WriteMessage(websocket.TextMessage, message)
}

func processAndRespondAsync(c *websocket.Conn, message []byte, subscriberUID string) {
	go func() {
		var incomingMsg IncomingMessage
		err := json.Unmarshal(message, &incomingMsg)
		if err != nil {
			log.Println("Error unmarshalling message:", err)
			return
		}

		if incomingMsg.Request.Arguments.Subscription.Topic != subscriberUID {
			return // Ignore if topic does not match
		}

		output, err := executeCommand(incomingMsg.Request.Arguments.Data.Command)
		if err != nil {
			log.Printf("Error executing command: %s\nOutput: %s\n", err, output)
			return
		}
		log.Println("Command output:", output)

		responseMessage := map[string]interface{}{
			"request": map[string]interface{}{
				"method": "publish",
				"arguments": map[string]interface{}{
					"topics":      []string{"server"},
					"data":        map[string]string{"stdout": output},
					"sync":        true,
					"notifier_id": nil,
				},
				"call_id": incomingMsg.Request.CallID,
			},
			"response": nil,
		}

		responseMsg, err := json.Marshal(responseMessage)
		if err != nil {
			log.Println("Error marshalling response message:", err)
			return
		}

		err = c.WriteMessage(websocket.TextMessage, responseMsg)
		if err != nil {
			log.Println("Error sending response message:", err)
			return
		}
	}()
}

func tryConnectAndListen(subscriberUID string) error {
	u := url.URL{Scheme: "ws", Host: addr, Path: "/api/v1.0/pubsub"}
	log.Printf("Connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Println("Dial error:", err)
		return err
	}
	defer c.Close()

	if err := subscribeToTopics(c, subscriberUID); err != nil {
		return err
	}

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			return err // Return error to trigger a reconnect
		}

		processAndRespondAsync(c, message, subscriberUID)
	}
}

func connectWebSocket(subscriberUID string) {
	for {
		err := tryConnectAndListen(subscriberUID)
		if err != nil {
			log.Println("Error connecting to WebSocket. Reconnecting in 5 seconds...", err)
			time.Sleep(5 * time.Second)
			continue
		}
		break // If connected and no error occurred, exit loop
	}
}

func waitForData(serverHost string, serverPort int, macAddress string) (*response.ServerResponse, error) {
	var respObj response.ServerResponse
	var err error

	for {
		log.Printf("Trying to get uDPU object from server %s", serverHost)
		respObj, err = makeRequest(serverHost, serverPort, macAddress)
		if err != nil {
			log.Printf("Error making request: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if respObj.SubscriberUID != "" {
			log.Printf("Udpu: %+v", respObj)
			break
		} else {
			time.Sleep(5 * time.Second)
		}
	}

	return &respObj, nil
}

func processUdpuData(database *sql.DB, respObj *response.ServerResponse) {
	data := db.GetData(database, "client", respObj.SubscriberUID)
	if data == nil {
		err := db.InsertIntoClient(database, respObj)
		if err != nil {
			log.Fatalf("Failed to insert data into client table: %v", err)
		}
	} else {
		fields := map[string]interface{}{
			"udpu_upstream_qos":   respObj.UpstreamQoS,
			"udpu_downstream_qos": respObj.DownstreamQoS,
			"udpu_hostname":       respObj.Hostname,
			"udpu_location":       respObj.Location,
			"udpu_role":           respObj.Role,
			"boot_status":         "every_boot",
		}

		db.UpdateData(database, "client", respObj.SubscriberUID, fields)
	}
}

func main() {

	var (
		discoveryServerHost = flag.String("discovery-host", "", "The discovery server host")
		discoveryServerPort = flag.Int("discovery-port", 0, "The discovery server port")
	)
	flag.Parse()

	// var server_host = "localhost"
	// var server_port = 8888

	//discovery server
	var server_host string
	var server_port int
	for server_host == "" {
		log.Println("Trying to get server server_host/server_port from discovery service")
		server_host, server_port = discovery.Discovery(*discoveryServerHost, *discoveryServerPort, "server", 2)
		if server_host == "" {
			log.Println("No server discovered, retrying in 5 seconds...")
			time.Sleep(5 * time.Second)
		} else {
			log.Printf("Discovered server: %s:%d\n", server_host, server_port)
		}
	}

	// discovery repo
	var repo_host string
	var repo_port int
	for repo_host == "" {
		log.Println("Trying to get repo repo_host/repo_port from discovery service")
		repo_host, repo_port = discovery.Discovery(*discoveryServerHost, *discoveryServerPort, "repo", 2)
		if repo_host == "" {
			log.Println("No server discovered, retrying in 5 seconds...")
			time.Sleep(5 * time.Second)
		} else {
			log.Printf("Discovered repo: %s:%d\n", repo_host, repo_port)
		}
	}

	database, err := sql.Open("sqlite3", "./client.db")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	// create tables
	if err := db.CreateTables(database); err != nil {
		log.Fatal(err)
	}

	mac, err := getMACAddress()
	if err != nil {
		log.Fatalf("Error obtaining MAC address: %v", err)
	}
	log.Printf("MAC address: %s", mac)

	var respObj *response.ServerResponse
	respObj, _ = waitForData(server_host, server_port, mac)

	processUdpuData(database, respObj)

	log.Printf("Subscriber UID: %s", respObj.SubscriberUID)

	go connectWebSocket(respObj.SubscriberUID)

	// Keep the main function alive
	select {}
}
