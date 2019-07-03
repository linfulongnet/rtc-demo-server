package socket

import (
	"encoding/json"
	"log"
)

type SignalingType int

const (
	Online SignalingType = iota + 100
	Offline
	UserInfo
	UserList
	Offer
	Answer
	Candidate
	HangUp
	Transfer
)

type SignalingData map[string]interface{}
type Signaling struct {
	Command SignalingType `json:"command"`
	Data    SignalingData `json:"data"`
	Target  int           `json:"target"`
	Source  int           `json:"source"`
}

func processSingnal(c *Client, message []byte) {
	sl := &Signaling{}
	err := json.Unmarshal(message, sl)
	if err != nil {
		log.Fatal("[processSingnal] Unmarshal message error: ", err)
	}
	log.Printf("[receive] id:%d addr:%s, msg:%v", c.info.Id, c.conn.RemoteAddr().String(), sl)
	switch sl.Command {
	case UserInfo:
		c.requestInfo()
		break
	case UserList:
		c.clientList()
		break
	case Offer:
		fallthrough
	case Answer:
		fallthrough
	case Candidate:
		fallthrough
	case HangUp:
		c.signal(sl)
		break
	case Transfer:
		log.Println("transfer:", sl.Command)
		break
	default:
		c.broadcast(sl)
	}
}
