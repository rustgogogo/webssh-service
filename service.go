package main

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"
)

/**
websocket接口
*/
func WebsshApi(c *gin.Context) {
	conn, err := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		return true
	}}).Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Err(err).Send()
	} else {
		//客户端连接后获取参数
		deviceId := c.Query("deviceId")
		if len(deviceId) == 0 {
			logger.Warn().Msg("websocket设备id为空")
			conn.Close()
		} else {
			logger.Debug().Str("deviceId", deviceId).Str("ip", c.ClientIP()).Str("ua", c.Request.UserAgent()).Msg("websocket连接成功，开始建立ws<->ssh隧道")
			go Ws2ssh(conn, deviceId)
		}
	}
}

/**
根据设备id获取ssh连接配置
应该从数据库读取
*/
func getSshConfigByDeviceId(deviceId string) (string, *ssh.ClientConfig) {
	sshConfig := &ssh.ClientConfig{
		User: "pi",
		Auth: []ssh.AuthMethod{
			ssh.Password("123456"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		ClientVersion:   "",
		Timeout:         10 * time.Second,
	}
	return "192.168.1.8:22", sshConfig
}

/**
建立ssh连接
*/
func SSHConnect(deviceId string) (*ssh.Session, io.WriteCloser, error) {

	addr, sshConfig := getSshConfigByDeviceId(deviceId)

	//建立与SSH服务器的连接
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		logger.Err(err).Str("deviceId", deviceId).Str("ssh-addr", addr).Msg("ssh连接失败")
		return nil, nil, err
	}

	//https://tools.ietf.org/html/rfc4254#page-10
	session, err := sshClient.NewSession()
	if err != nil {
		sshClient.Close()
		logger.Err(err).Str("deviceId", deviceId).Str("ssh-addr", addr).Msg("ssh会话创建失败")
		return nil, nil, err
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     //打开回显
		ssh.TTY_OP_ISPEED: 14400, //输入速率 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, //输出速率 14.4kbaud
		ssh.VSTATUS:       1,
	}

	var termWidth, termHeight int

	if runtime.GOOS == "windows" {
		termWidth = 80
		termHeight = 30
	} else {
		//使用VT100终端来实现tab键提示，上下键查看历史命令，clear键清屏等操作
		//VT100 start
		//windows下不支持VT100
		fd := int(os.Stdin.Fd())
		oldState, err := terminal.MakeRaw(fd)
		if err != nil {
			logger.Err(err).Msg("VT100终端错误")
		}
		defer terminal.Restore(fd, oldState)
		//VT100 end
		termWidth, termHeight, err = terminal.GetSize(fd)
	}

	//打开伪终端
	//https://tools.ietf.org/html/rfc4254#page-11
	err = session.RequestPty("xterm", termHeight, termWidth, modes)
	if err != nil {
		session.Close()
		sshClient.Close()

		logger.Err(err).Str("deviceId", deviceId).Str("ssh-addr", addr).Msg("ssh伪终端创建失败")
		return nil, nil, err
	}

	pipeInput, err := session.StdinPipe()

	if err != nil {
		session.Close()
		sshClient.Close()

		logger.Err(err).Str("deviceId", deviceId).Str("ssh-addr", addr).Msg("ssh输入管道打开失败")
		return nil, nil, err
	}

	//启动一个远程shell
	//https://tools.ietf.org/html/rfc4254#page-13
	err = session.Shell()
	if err != nil {
		session.Close()
		sshClient.Close()

		logger.Err(err).Str("deviceId", deviceId).Str("ssh-addr", addr).Msg("ssh shell打开失败")
		return nil, nil, err
	}

	go func() {
		//等待远程命令结束或远程shell退出
		err = session.Wait()
		if err != nil {
			logger.Err(err).Str("deviceId", deviceId).Str("ssh-addr", addr).Msg("ssh会话断开")
		}
	}()

	return session, pipeInput, nil
}

/**
打通websocket 到 ssh之间的连接
*/
func Ws2ssh(wsConn *websocket.Conn, deviceId string) {
	session, pipeInput, err := SSHConnect(deviceId)

	if err != nil {
		wsConn.Close()
		return
	}

	session.Stdout = &MyOutput{WsConn: wsConn}

	go StreamBind(wsConn, deviceId, session, pipeInput)
}

/**
流绑定
*/
func StreamBind(wsConn *websocket.Conn, deviceId string, session *ssh.Session, pipeInput io.WriteCloser) {
	defer wsConn.Close()
	defer session.Close()
	for {
		msgType, msg, err := wsConn.ReadMessage()
		if err != nil {
			logger.Err(err).Str("deviceId", deviceId).Msg("读取websocket数据失败，断开流绑定")
			break
		} else {
			logger.Debug().Str("deviceId", deviceId).Int("msgType", msgType).Int("消息长度", len(msg)).Msg("websocket收到消息")
		}
		n, err := pipeInput.Write(msg)
		if err != nil {
			logger.Err(err).Str("deviceId", deviceId).Msg("数据发送ssh会话失败，断开流绑定")
			break
		} else {
			logger.Debug().Str("deviceId", deviceId).Int("sendLen", n).Msg("发送给ssh数据")
		}
	}

	logger.Info().Str("deviceId", deviceId).Msg("连接断开，关闭websocket和ssh会话")
}

func main() {
	//启动websocket服务
	h := gin.Default()

	//内部使用的api
	h.GET("/api/log/debug", SetLogDebugLevel)
	h.GET("/api/log/info", SetLogInfoLevel)

	//对外提供websocket api
	h.GET("/api/v1/webssh", WebsshApi)

	err := h.Run()
	if err != nil {
		logger.Err(err).Msg("服务启动失败")
	}
}
