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

// Структура для ответа сервера
type ServerResponse struct {
	SubscriberUID string `json:"subscriber_uid"`
}

// Структура для входящего сообщения
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

// Функция для получения MAC-адреса устройства
func getMACAddress() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, interf := range interfaces {
		if interf.Name == "br-lan" { // Проверка на соответствие имени интерфейса
			if len(interf.HardwareAddr) > 0 {
				return interf.HardwareAddr.String(), nil
			}
			return "", errors.New("MAC адрес для интерфейса br-lan не найден")
		}
	}

	return "", errors.New("Интерфейс br-lan не найден")
}

// Функция для выполнения HTTP-запроса и получения subscriber_uid
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

// Функция для подключения и поддержания соединения с WebSocket
func connectWebSocket(subscriberUID string) {
	for {
		err := connectAndListen(subscriberUID)
		if err != nil {
			log.Println("Ошибка при подключении к WebSocket:", err)
			log.Println("Попытка переподключения через 5 секунд...")
			time.Sleep(5 * time.Second)
		}
	}
}

// Функция для отправки запроса на подписку
func subscribeToTopics(c *websocket.Conn, subscriberUID string) error {
	subscriptionMessage := map[string]interface{}{
		"request": map[string]interface{}{
			"method": "subscribe",
			"arguments": map[string]interface{}{
				"topics": []string{subscriberUID},
			},
			"call_id": "4140dd17a18c45db8a98ba155cbfat21",
		},
		"response": nil,
	}
	message, err := json.Marshal(subscriptionMessage)
	if err != nil {
		return err
	}
	return c.WriteMessage(websocket.TextMessage, message)
}

// Функция для обработки и отправки ответа
func processAndRespond(c *websocket.Conn, message []byte, subscriberUID string) error {
	var incomingMsg IncomingMessage
	err := json.Unmarshal(message, &incomingMsg)
	if err != nil {
		fmt.Println("err: ", err)
		return err
	}

	fmt.Println("incomingMsg: ", incomingMsg)
	fmt.Println("incomingMsg.Request.Arguments.Subscription.Topic : ", incomingMsg.Request.Arguments.Subscription.Topic)
	fmt.Println("subscriberUID: ", subscriberUID)

	// Проверка соответствия topic и Subscriber UID
	if incomingMsg.Request.Arguments.Subscription.Topic != subscriberUID {
		return nil // Ничего не делаем, если topic не соответствует
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

// Функция для подключения к WebSocket и прослушивания сообщений
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
			log.Println("Ошибка при обработке и отправке ответа:", err)
		}
	}
}

func main() {
	// Получение MAC-адреса
	mac, err := getMACAddress()
	if err != nil {
		fmt.Println("Ошибка при получении MAC-адреса:", err)
		return
	}
	fmt.Println("MAC-адрес:", mac)

	// Выполнение HTTP-запроса
	subscriberUID, err := makeRequest(mac)
	if err != nil {
		fmt.Println("Ошибка при выполнении HTTP-запроса:", err)
		return
	}

	fmt.Println("Subscriber UID:", subscriberUID)

	// Подключение к WebSocket и поддержание соединения
	connectWebSocket(subscriberUID)
}
