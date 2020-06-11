package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"go-testing/mist"
	"go-testing/pb"
	"go-testing/pbutil"
	"go-testing/server"
	"log"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var (
	addr        = flag.String("addr", "127.0.0.1:5280", "server addr")
	port        = flag.String("port", "1234", "monitor server port")
	mode        = flag.String("mode", "none", "type of message, chat、groupchat etc.")
	interval    = flag.Int("interval", 3, "send message every interval seconds.")
	connections = flag.Int("conn", 1, "number of tcp connections")
	resource    = flag.String("resource", "WEB", "resource of tcp connection")
	idMaker     = mist.NewMist()
	chatTips    = "this is a chat message."
)

func main() {
	fmt.Println("welcome go-testing!!!")
	flag.Parse()

	if *addr == "" {
		fmt.Printf("示例: go run main.go -addr 127.0.0.1:5280 -conn 1 \n")

		flag.Usage()

		return
	}

	runtime.GOMAXPROCS(runtime.NumCPU() - 1)

	log.Printf("连接到 %s", *addr)
	go server.StartHttpSrv(*port)

	wg := new(sync.WaitGroup)
	wg.Add(*connections)
	for i := 0; i < *connections; i++ {
		go connect(wg)
	}
	wg.Wait()
}

func connect(wg *sync.WaitGroup) {
	defer wg.Done()

	conn, err := net.DialTimeout("tcp", *addr, 10*time.Second)
	if err != nil {
		log.Printf("连接服务器错误: %v \n", err)
		return
	}

	userId := idMaker.Gen()
	if _, ok := server.Connections.Load(userId); !ok {
		userId = idMaker.Gen()
	}
	m := md5.New()
	m.Write([]byte(userId))
	password := hex.EncodeToString(m.Sum(nil))

	client := NewTcpClint(conn, userId, password, *resource)

	server.Connections.Store(userId, client)

	msg := client.authMsg()
	_, err = pbutil.WriteDelimited(client.conn, &msg)
	if err != nil {
		log.Printf("发送认证包错误: %v \n", err)
		return
	}
	log.Printf("发送认证包数据: %s \n", msg.String())
	authAck := pb.IMMessage{}
	_, err = pbutil.ReadDelimited(client.r, &authAck)
	if err != nil {
		log.Printf("读取认证包错误: %v \n", err)
		return
	}
	log.Printf("接收认证包响应数据: %s \n", authAck.String())
	if authAck.GetAuthMessageAckBody() == nil {
		log.Println("认证异常!!!")
		return
	}
	if authAck.GetAuthMessageAckBody().Code == 10000 {
		client.logined = true
	} else {
		log.Printf("login error code: %d, msg: %s \n", authAck.GetAuthMessageAckBody().Code,
			authAck.GetAuthMessageAckBody().GetMessage())
		return
	}

	server.OnlineUsers.Store(client.userId, client)

	if *mode == "chat" {
		go client.chat()
	}

	go client.send()
	go client.recieve()

	select {
	case <-client.quit:
		server.OnlineUsers.Delete(client.userId)
		client.Close()
		log.Printf("用户: %s, 退出登录~~~ \n", client.userId)
	}
}

func (c *TcpClient) send() {
	beatDuration := time.Second * 10
	beatDelay := time.NewTimer(beatDuration)
	defer beatDelay.Stop()
	for c.logined {
		beatDelay.Reset(beatDuration)
		select {
		case message, ok := <-c.message:
			if !ok {
				continue
			}
			var msg pb.IMMessage
			to := idMaker.Gen()
			if *mode == "groupchat" {
				msg = c.randomChatMsg(message, to)
			} else {
				msg = c.randomChatMsg(message, to)
			}
			_, err := pbutil.WriteDelimited(c.conn, &msg)
			if err != nil {
				log.Printf("发送消息包错误: %v", err)
				c.quit <- true
			}
			atomic.AddInt64(&server.MessageSentCounter, 1)
		case <-beatDelay.C:
			pingMsg := c.pingMsg()
			log.Printf("发送心跳包数据: %s \n", pingMsg.String())
			_, err := pbutil.WriteDelimited(c.conn, &pingMsg)
			if err != nil {
				log.Printf("发送心跳包错误: %v", err)
				c.quit <- true
			}
		}
	}
}

func (c *TcpClient) chat() {
	defer func() {
		if r := recover(); r != nil {
			log.Println("chat recovered in f", r)
		}
	}()
	chatDuration := time.Second * time.Duration(*interval)
	chatDelay := time.NewTimer(chatDuration)
	defer chatDelay.Stop()
	for c.logined {
		chatDelay.Reset(chatDuration)
		select {
		case <-chatDelay.C:
			// log.Printf("send chat message>>>>[%s] \n", chatTips)
			c.message <- chatTips
		}
	}
}

func (c *TcpClient) authMsg() pb.IMMessage {
	auth := pb.IMAuthMessage{
		MsgId:  idMaker.Gen(),
		UserId: c.userId,
		Source: c.resource,
		Token:  c.password,
	}

	msg := pb.IMMessage{
		DataType: pb.DataType_IMAuthMessageType,
		DataBody: &pb.IMMessage_AuthMessageBody{
			AuthMessageBody: &auth,
		},
	}
	return msg
}

func (c *TcpClient) randomChatMsg(message string, to string) pb.IMMessage {
	chatType := pb.ChatType_SingleChat
	if *mode == "groupchat" {
		chatType = pb.ChatType_GroupChat
	}
	chat := pb.IMChatMessage{
		MsgId: idMaker.Gen(),
		From:  c.userId,
		Nick:  c.nickname,
		To:    to,
		Body: &pb.IMChatMessage_TextMessage{
			TextMessage: &pb.TextMessage{
				Content: message,
			},
		},
		CType:    chatType,
		Icon:     c.avatar,
		MType:    pb.MessageType_TextMessageType,
		IsAck:    true,
		IsEncry:  false,
		Snapchat: 0,
		SendTime: time.Now().Unix(),
	}
	msg := pb.IMMessage{
		DataType: pb.DataType_IMChatMessageType,
		DataBody: &pb.IMMessage_ChatMessageBody{
			ChatMessageBody: &chat,
		},
	}
	return msg
}

func (c *TcpClient) pingMsg() pb.IMMessage {
	pingReq := pb.IMPingMessage{
		MsgId:  idMaker.Gen(),
		UserId: c.userId,
	}
	msg := pb.IMMessage{
		DataType: pb.DataType_IMPingMessageType,
		DataBody: &pb.IMMessage_PingMessageBody{
			PingMessageBody: &pingReq,
		},
	}
	return msg
}

func (c *TcpClient) recieve() {
	for c.logined {
		resp := pb.IMMessage{}
		_, err := pbutil.ReadDelimited(c.r, &resp)
		if err != nil {
			log.Printf("接收响应数据: %s \n", err)
			c.quit <- true
			return
		}
		switch resp.DataType {
		case pb.DataType_IMPongMessageType:
			log.Printf("接收心跳包响应数据: %s \n", resp.String())
		case pb.DataType_IMChatMessageACKType:
			atomic.AddInt64(&server.MessageAckCounter, 1)
			log.Printf("接收消息包ACK数据: %s \n", resp.String())
		default:
			log.Printf("接收响应包数据: %s \n", resp.String())
		}
	}
}

func (c *TcpClient) Write(message []byte) (int, error) {
	// 读取消息的长度
	var length = uint32(len(message))
	var pkg = new(bytes.Buffer)
	//写入消息头
	err := binary.Write(pkg, binary.BigEndian, length)
	if err != nil {
		return 0, err
	}
	//写入消息体
	err = binary.Write(pkg, binary.BigEndian, message)
	if err != nil {
		return 0, err
	}
	nn, err := c.conn.Write(pkg.Bytes())
	if err != nil {
		return 0, err
	}
	return nn, nil
}

type TcpClient struct {
	userId   string
	nickname string
	avatar   string
	resource string
	password string
	conn     net.Conn
	logined  bool
	quit     chan bool
	message  chan string
	r        *bufio.Reader
}

func NewTcpClint(conn net.Conn, userId, password, resource string) *TcpClient {
	return &TcpClient{conn: conn,
		r:        bufio.NewReader(conn),
		userId:   userId,
		password: password,
		resource: resource,
		message:  make(chan string),
		quit:     make(chan bool),
	}
}

func (c *TcpClient) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *TcpClient) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *TcpClient) Close() error {
	c.logined = false
	// close(c.quit)
	// close(c.message)
	return c.conn.Close()
}

func (c *TcpClient) Read() ([]byte, error) {
	// Peek 返回缓存的一个切片，该切片引用缓存中前 n 个字节的数据，
	// 该操作不会将数据读出，只是引用，引用的数据在下一次读取操作之
	// 前是有效的。如果切片长度小于 n，则返回一个错误信息说明原因。
	// 如果 n 大于缓存的总大小，则返回 ErrBufferFull。
	lengthByte, err := c.r.Peek(4)
	if err != nil {
		return nil, err
	}
	//创建 Buffer缓冲器
	lengthBuff := bytes.NewBuffer(lengthByte)
	var length int32
	// 通过Read接口可以将buf中得内容填充到data参数表示的数据结构中
	err = binary.Read(lengthBuff, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}

	// Buffered 返回缓存中未读取的数据的长度
	if int32(c.r.Buffered()) < length+4 {
		return nil, err
	}
	// 读取消息真正的内容
	pack := make([]byte, int(4+length))
	// Read 从 b 中读出数据到 p 中，返回读出的字节数和遇到的错误。
	// 如果缓存不为空，则只能读出缓存中的数据，不会从底层 io.Reader
	// 中提取数据，如果缓存为空，则：
	// 1、len(p) >= 缓存大小，则跳过缓存，直接从底层 io.Reader 中读
	// 出到 p 中。
	// 2、len(p) < 缓存大小，则先将数据从底层 io.Reader 中读取到缓存
	// 中，再从缓存读取到 p 中。
	_, err = c.r.Read(pack)
	if err != nil {
		return nil, err
	}
	return pack[4:], nil
}
