package main

import (
	"github.com/gorilla/websocket"
)

type MyOutput struct {
	WsConn *websocket.Conn
}

func (out *MyOutput) Write(p []byte) (n int, err error) {
	err = out.WsConn.WriteMessage(websocket.BinaryMessage, p)
	return len(p), err
}
