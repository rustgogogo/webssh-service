package main

import (
	"github.com/gorilla/websocket"
)

/**
设备信息
*/
type DeviceInfo struct {
	/**
	设备id
	*/
	DeviceId string
	/**
	ssh端口
	*/
	SshPort int
	/**
	设备ip
	*/
	Ip string
	/**
	ssh用户名
	*/
	SshUser string
	/**
	ssh密码
	*/
	SshPwd string
}

type MyOutput struct {
	WsConn *websocket.Conn
}

func (out *MyOutput) Write(p []byte) (n int, err error) {
	err = out.WsConn.WriteMessage(websocket.BinaryMessage, p)
	return len(p), err
}
