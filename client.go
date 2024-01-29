package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// Structure for the server response
type ServerResponse struct {
	SubscriberUID string `json:"subscriber_uid"`
}

// Structure for an incoming message
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

// Function for obtaining the MAC address of the device
func getMACAddress() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, interf := range interfaces {
		if interf.Name == "br-lan" { // Check if the interface name matches
			if len(interf.HardwareAddr) > 0 {
				return interf.HardwareAddr.String(), nil
			}
			return "", errors.New("MAC address for br-lan interface not found")
		}
	}

	return "", errors.New("br-lan interface not found")
}

// Function to make an HTTP request and get subscriber_uid
func makeRequest(macAddress string) (string, error) {
	resp, err := http.Get("http://161.184.221.236:8887/api/v1.0/adapter/" + macAddress + "/udpu")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var respObj ServerResponse
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		return "", err
	}

	fmt.Println("uDPU:", respObj)
	return respObj.SubscriberUID, nil
}

// Function for connecting and maintaining a connection with WebSocket
func connectWebSocket(subscriberUID string) {
	for {
		err := connectAndListen(subscriberUID)
		if err != nil {
			log.Println("Error connecting to WebSocket:", err)
			log.Println("Attempting to reconnect after 5 seconds...")
			time.Sleep(5 * time.Second)
		}
	}
}

// Function for sending a subscription request
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

// Function for processing and sending a response
func processAndRespond(c *websocket.Conn, message []byte, subscriberUID string) error {
	var incomingMsg IncomingMessage
	err := json.Unmarshal(message, &incomingMsg)
	if err != nil {
		fmt.Println("err: ", err)
		return err
	}

	fmt.Println("incomingMsg.Request.Arguments.Subscription.Topic : ", incomingMsg.Request.Arguments.Subscription.Topic)
	fmt.Println("subscriberUID: ", subscriberUID)

	// Verifying topic and Subscriber UID matches
	if incomingMsg.Request.Arguments.Subscription.Topic != subscriberUID {
		return nil // Do nothing if topic does not match
	}

	responseObj := map[string]interface{}{
		"stdout":   incomingMsg.Request.Arguments.Data.Command,
		"stderr":   "",
		"datetime": time.Now().Format(time.RFC3339),
		"type":     incomingMsg.Request.Arguments.Data.ActionType,
	}

	responseMessage := map[string]interface{}{
		"request": map[string]interface{}{
			"method": "publish",
			"arguments": map[string]interface{}{
				"topics": []string{"server"},
				"data":   responseObj,
				"sync":   true,
			},
			"call_id": "86d65b4254f1436089bdbb43c96628b6",
		},
		"response": nil,
	}

	fmt.Println("responseMessage: ", responseMessage)

	responseMsg, err := json.Marshal(responseMessage)
	if err != nil {
		return err
	}

	return c.WriteMessage(websocket.TextMessage, responseMsg)
}

// Function to connect to WebSocket and listen to messages
func connectAndListen(subscriberUID string) error {
	u := "ws://161.184.221.236:8887/api/v1.0/pubsub"
	c, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		return err
	}
	defer c.Close()

	err = subscribeToTopics(c, subscriberUID)
	if err != nil {
		return err
	}

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			return err
		}

		err = processAndRespond(c, message, subscriberUID)
		if err != nil {
			log.Println("Error processing and sending response:", err)
		}
	}
}

func main() {
	// Obtaining a MAC Address
	mac, err := getMACAddress()
	if err != nil {
		fmt.Println("Error while obtaining MAC address:", err)
		return
	}
	fmt.Println("MAC-address:", mac)

	// Making an HTTP Request
	subscriberUID, err := makeRequest(mac)
	if err != nil {
		fmt.Println("Error while making HTTP request:", err)
		return
	}

	fmt.Println("Subscriber UID:", subscriberUID)

	// Connect to WebSocket and maintain connection
	connectWebSocket(subscriberUID)
}
