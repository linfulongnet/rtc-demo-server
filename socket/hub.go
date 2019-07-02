package socket

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"reflect"
	"sort"
	"time"

	"github.com/gorilla/websocket"

	"../utils"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline  = []byte{'\n'}
	space    = []byte{' '}
	upGrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	hub *Hub
)

type broadcastMessage struct {
	source  int
	message []byte
}
type Hub struct {
	// Registered clients.
	clients map[*Client]bool
	// Inbound messages from the clients.
	broadcast chan *broadcastMessage
	// Register requests from the clients.
	register chan *Client
	// Unregister requests from clients.
	unregister chan *Client
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			go func() {
				client.requestInfo()
				client.online()
			}()
		case client := <-h.unregister:
			go func() {
				if _, ok := h.clients[client]; ok {
					client.offline()
				}
			}()
		case broadMsg := <-h.broadcast:
			for client := range h.clients {
				// 过滤发广播消息的客户端
				if broadMsg.source == client.info.Id {
					continue
				}

				select {
				case client.send <- broadMsg.message:
				default:
					log.Printf("***** client panic ******")
					client.hub.unregister <- client
				}
			}
		}
	}
}
func (h *Hub) getClient(id int) *Client {
	for cli := range h.clients {
		if cli.info.Id == id {
			return cli
		}
	}

	return nil
}

// Client is a middleman between the websocket connection and the hub.
type ClientInfo struct {
	Id         int    `json:"id"`
	OnlineTime string `json:"onlineTime"`
}
type Client struct {
	hub *Hub
	// The websocket connection.
	conn *websocket.Conn
	// Buffered channel of outbound messages.
	send chan []byte
	info *ClientInfo
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		go processSingnal(c, message)
	}
}
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			_, _ = w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write(newline)
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
func (c *Client) sendMsg(slMsg *Signaling) {
	message, err := json.Marshal(slMsg)
	if err != nil {
		log.Println("sendMsg Marshal Signaling error: ", err)
		return
	}

	log.Printf("[sendMsg] command:%d, data:%v", slMsg.Command, slMsg.Data)

	c.send <- message
}
func (c *Client) sendTo(target *Client, slMsg *Signaling) {
	slMsg.Target = target.info.Id
	slMsg.Source = c.info.Id
	message, err := json.Marshal(slMsg)
	if err != nil {
		log.Println("sendTo Marshal Signaling error: ", err)
		return
	}

	log.Printf("[sendTo] target:%d, source:%d, command:%d, data:%v", slMsg.Target, slMsg.Source, slMsg.Command, slMsg.Data)

	target.send <- message
}
func (c *Client) broadcast(slMsg *Signaling) {
	slMsg.Source = c.info.Id
	message, err := json.Marshal(slMsg)
	if err != nil {
		log.Println("broadcast Marshal Signaling error: ", err)
		return
	}

	log.Printf("[broadcast] source:%d, command:%d, data:%v", slMsg.Source, slMsg.Command, slMsg.Data)

	broadMsg := &broadcastMessage{
		source:  c.info.Id,
		message: message,
	}
	c.hub.broadcast <- broadMsg
}

// 定义客户端列表接口，并实现接口排序的三个方法
type clientInfoList []*ClientInfo

func (c clientInfoList) Len() int {
	return len(c)
}
func (c clientInfoList) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}
func (c clientInfoList) Less(i, j int) bool {
	return c[i].Id < c[j].Id
}

func (c *Client) infoToMap() SignalingData {
	info := *c.info
	t := reflect.TypeOf(info)
	v := reflect.ValueOf(info)

	infoMap := make(SignalingData)
	for i := 0; i < t.NumField(); i++ {
		infoMap[utils.FirstLetterToLower(t.Field(i).Name)] = v.Field(i).Interface()
	}

	return infoMap
}
func (c *Client) online() {
	slMsg := &Signaling{
		Command: Online,
		Data:    c.infoToMap(),
	}
	c.broadcast(slMsg)
}
func (c *Client) offline() {
	defer func() {
		// 关闭客户端连接
		// time.Sleep(1 * time.Second)
		close(c.send)
		delete(c.hub.clients, c)
	}()

	slMsg := &Signaling{
		Command: Offline,
		Data:    c.infoToMap(),
	}
	c.broadcast(slMsg)
}
func (c *Client) requestInfo() {
	slMsg := &Signaling{
		Command: UserInfo,
		Data:    c.infoToMap(),
	}
	c.sendMsg(slMsg)
}
func (c *Client) clientList() {
	slData := make(map[string]interface{})
	clients := make(clientInfoList, 0, len(c.hub.clients)-1)
	for cli := range c.hub.clients {
		if cli.info.Id == c.info.Id {
			continue
		}
		clients = append(clients, cli.info)
	}
	if !sort.IsSorted(clients) {
		sort.Sort(clients)
	}
	slData["list"] = clients

	slMsg := &Signaling{
		Command: UserList,
		Data:    slData,
	}
	c.sendMsg(slMsg)
}

func (c *Client) offer(slMsg *Signaling) {
	targetClient := c.hub.getClient(slMsg.Target)
	if targetClient == nil {
		return
	}

	c.sendTo(targetClient, slMsg)
}
func (c *Client) answer(slMsg *Signaling) {
	targetClient := c.hub.getClient(slMsg.Target)
	if targetClient == nil {
		return
	}

	c.sendTo(targetClient, slMsg)
}
func (c *Client) hangup(slMsg *Signaling) {
	targetClient := c.hub.getClient(slMsg.Target)
	if targetClient == nil {
		return
	}

	c.sendTo(targetClient, slMsg)
}
func (c *Client) transfer(slMsg *Signaling) {
	c.broadcast(slMsg)
}

type Id struct {
	value int
}

func (id *Id) get() int {
	id.value += 1

	return id.value
}

// webSocket 客户端 ID
var uId = &Id{value: 0}

// serveWs handles websocket requests from the peer.
func Serve(w http.ResponseWriter, r *http.Request) {
	conn, err := upGrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	client := &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 256),
		info: &ClientInfo{
			Id:         uId.get(),
			OnlineTime: utils.NowUtcString(),
		},
	}

	go client.writePump()
	go client.readPump()

	client.hub.register <- client
}

func init() {
	hub = &Hub{
		broadcast:  make(chan *broadcastMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}

	go hub.run()
}
