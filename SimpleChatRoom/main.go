/*
	Author		: Nancy Yang / maxbearwiz@gmail.com
	Date			: 2021-01-18
	Description : Websocket backend for Simple Chat Room application
*/

package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type WebsocketClient struct {
	Name string
	Conn *websocket.Conn
}

type ChatMessage struct {
	MessageType string `json:"MessageType"`
	Username    string `json:"Username"`
	Timestamp   string `json:"Timestamp"`
	Text        string `json:"Text"`
}

var clients = make(map[*WebsocketClient]bool)
var register = make(chan *WebsocketClient)
var unregister = make(chan *WebsocketClient)
var broadcaster = make(chan ChatMessage)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleMessages() {
	for {
		select {
		case client := <-register:
			clients[client] = true
			log.Print("A new websocket connection is registered")
		case client := <-unregister:
			delete(clients, client)
			log.Print("A new websocket connection is unregistered")
		case msg := <-broadcaster:
			for client := range clients {
				msg.Timestamp = time.Now().UTC().Format(time.RFC3339)
				log.Printf("%+v", msg)
				if client.Name != msg.Username {
					err := client.Conn.WriteJSON(msg)
					if err != nil && err != io.EOF && !websocket.IsCloseError(err, websocket.CloseGoingAway) {
						log.Printf("[%s] connection error: %v", client.Name, err)
						client.Conn.Close()
						delete(clients, client)
					}
				}
			}

		}
	}
}

func (c *WebsocketClient) Read() {
	defer func() {
		unregister <- c
		c.Conn.Close()
	}()
	for {
		var msg ChatMessage
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Println(err)
			}
			break
		}
		log.Printf("[%s] received msg: %+v", c.Name, msg)
		Username := strings.TrimSpace(msg.Username)
		if c.Name != Username {
			c.Name = Username
			log.Printf("Name websocket \"%s\"", c.Name)
			announce := ChatMessage{
				Username:    msg.Username,
				MessageType: "announce",
				Text:        fmt.Sprintf("\"%s\" has joined chat room", c.Name),
			}
			broadcaster <- announce
		}
		if len(strings.TrimSpace(msg.Text)) > 0 {
			msg.MessageType = "message"
			broadcaster <- msg
		}
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	client := &WebsocketClient{
		Conn: ws,
	}
	register <- client

	go client.Read()
}

func main() {
	http.Handle("/", http.FileServer(http.Dir("./frontend/public")))
	http.HandleFunc("/websocket", handleConnections)
	go handleMessages()

	err := http.ListenAndServe(":5000", nil)
	log.Print("Server starting at localhost:5000")
	if err != nil {
		log.Fatal(err)
	}
}
