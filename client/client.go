package client

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-testing/mist"
	"go-testing/pb"
	"go-testing/pbutil"
	"log"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var (
	OnlineUsers             = sync.Map{}
	Connections             = sync.Map{}
	MessageSentCounter      int64
	MessageAckCounter       int64
	MessageReceivedCounter  int64
	test_start_time               = ""
	test_end_time                 = ""
	delay_limit             int64 = 10 // seconds
	under_delay_limit_count       = 0
	delay_count                   = 0
	total_delay             int64 = 0
	max_delay               int64 = 0
	min_delay               int64 = 0
	idMaker                       = mist.NewMist()
)

func Connect(id int, wg *sync.WaitGroup, addr, resource, tips string, interval int) (authed bool) {
	defer wg.Done()

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		log.Printf("连接服务器错误: %v \n", err)
		return
	}

	// userId := idMaker.Gen()
	// if _, ok := Connections.Load(userId); !ok {
	// 	userId = idMaker.Gen()
	// }
	userId := strconv.Itoa(id)
	m := md5.New()
	m.Write([]byte(userId))
	password := hex.EncodeToString(m.Sum(nil))
	client := NewTcpClient(conn, userId, password, resource)

	Connections.Store(userId, client)

	msg := client.authMsg()
	_, err = pbutil.WriteDelimited(client.conn, &msg)
	if err != nil {
		log.Printf("发送认证包错误: %v \n", err)
		client.Close()
		return
	}
	log.Printf("发送认证包数据: %s \n", msg.String())
	authAck := pb.IMMessage{}
	_, err = pbutil.ReadDelimited(client.r, &authAck)
	if err != nil {
		client.Close()
		log.Printf("读取认证包错误: %v \n", err)
		return
	}
	log.Printf("接收认证包响应数据: %s \n", authAck.String())
	if authAck.GetAuthMessageAckBody() == nil {
		client.Close()
		log.Println("认证异常!!!")
		return
	}
	if authAck.GetAuthMessageAckBody().Code == 10000 {
		client.logined = true
	} else {
		client.Close()
		log.Printf("login error code: %d, msg: %s \n", authAck.GetAuthMessageAckBody().Code,
			authAck.GetAuthMessageAckBody().GetMessage())
		return
	}

	OnlineUsers.Store(client.userId, client)

	go client.send()
	go client.recieve()

	// select {
	// case <-client.quit:
	// 	server.OnlineUsers.Delete(client.userId)
	// 	client.Close()
	// 	log.Printf("用户: %s, 退出登录~~~ \n", client.userId)
	// }
	return true
}

func NewTcpClient(conn net.Conn, userId, password, resource string) *TcpClient {
	return &TcpClient{conn: conn,
		r:        bufio.NewReader(conn),
		userId:   userId,
		password: password,
		resource: resource,
		message:  make(chan string),
		quit:     make(chan bool),
	}
}

func (c *TcpClient) send() {
	beatDuration := time.Second * 20
	beatDelay := time.NewTimer(beatDuration)
	defer beatDelay.Stop()
	for c.logined {
		beatDelay.Reset(beatDuration)
		select {
		case message, ok := <-c.message:
			if !ok {
				continue
			}
			var data MsgData
			err := json.Unmarshal([]byte(message), &data)
			if err != nil {
				log.Printf("json解析消息错误: %v", err)
				continue
			}
			var msg pb.IMMessage
			to := idMaker.Gen()
			if data.To != "" {
				to = data.To
			}

			msg = c.randomChatMsg(message, to)

			_, err = pbutil.WriteDelimited(c.conn, &msg)
			if err != nil {
				log.Printf("发送消息包错误: %v", err)
				c.quit <- true
			}
			atomic.AddInt64(&MessageSentCounter, 1)
		case <-beatDelay.C:
			pingMsg := c.pingMsg()
			log.Printf("发送心跳包数据: %s \n", pingMsg.String())
			_, err := pbutil.WriteDelimited(c.conn, &pingMsg)
			if err != nil {
				log.Printf("发送心跳包错误: %v", err)
				c.quit <- true
			}
		case _, ok := <-c.quit:
			log.Println("连接已断开！！！！")
			if ok {
				c.Close()
			}
		}
	}
}

// func (c *TcpClient) chat(interval int, msg string) {
// 	defer func() {
// 		if r := recover(); r != nil {
// 			log.Println("chat recovered in f", r)
// 		}
// 	}()
// 	chatDuration := time.Second * time.Duration(interval)
// 	chatDelay := time.NewTimer(chatDuration)
// 	defer chatDelay.Stop()
// 	for c.logined {
// 		chatDelay.Reset(chatDuration)
// 		select {
// 		case <-chatDelay.C:
// 			// log.Printf("send chat message>>>>[%s] \n", chatTips)
// 			c.message <- msg
// 		}
// 	}
// }

type MsgData struct {
	Msg string
	To  string
}

func (c *TcpClient) sendMsg(msg string, to int) {
	data := MsgData{Msg: msg, To: strconv.Itoa(to)}
	dbytes, _ := json.Marshal(data)
	c.message <- string(dbytes)
}

func SendAll(count int) {
	MessageSentCounter = 0
	MessageAckCounter = 0
	test_start_time = time.Now().Format("2006-01-02 15:04:05")
	for i := 0; i < count; i++ {
		f := func(k, v interface{}) bool {
			if v != nil {
				client := v.(*TcpClient)
				to, _ := strconv.Atoi(k.(string))
				to += 1000000
				client.sendMsg("hello "+strconv.Itoa(i), to)
			}
			return true
		}
		Connections.Range(f)
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

func (c *TcpClient) messageAck(ackMsgId, sMsgId string) pb.IMMessage {
	ackMsg := pb.IMChatMessageACK{
		MsgId:    idMaker.Gen(),
		AckMsgId: ackMsgId,
		SMsgId:   sMsgId,
		CType:    pb.ChatType_SingleChat,
	}
	msg := pb.IMMessage{
		DataType: pb.DataType_IMChatMessageACKType,
		DataBody: &pb.IMMessage_ChatMessageAckBody{
			ChatMessageAckBody: &ackMsg,
		},
	}
	return msg
}

func (c *TcpClient) recieve() {
	defer func() {
		if r := recover(); r != nil {
			log.Println("recieve recovered in f", r)
		}
	}()
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
			atomic.AddInt64(&MessageAckCounter, 1)
			log.Printf("接收消息包ACK数据: %s \n", resp.String())
		case pb.DataType_IMChatMessageToACKType:
			log.Printf("接收对方消息确认回执数据: %s \n", resp.String())
		case pb.DataType_IMChatMessageType:
			log.Printf("接收消息数据: %s \n", resp.String())
			messageReceived := resp.GetChatMessageBody()
			if messageReceived.SendTime > 0 {
				delay := time.Now().Unix() - messageReceived.SendTime
				if delay > max_delay {
					max_delay = delay
				} else if delay > 0 && delay < min_delay {
					min_delay = delay
				}
				if delay > 0 {
					total_delay += delay
					delay_count++
					if delay <= delay_limit {
						under_delay_limit_count++
					}
				}
			}
			c.messageAck(messageReceived.MsgId, messageReceived.SMsgId)
			test_end_time = time.Now().Format("2006-01-02 15:04:05")
			atomic.AddInt64(&MessageReceivedCounter, 1)
		default:
			log.Printf("接收响应包数据: %s \n", resp.String())
		}
	}
}

func ReceivedResultMsg() (result string) {
	if test_end_time == "" {
		result += "Test is not started yet.\n"
	} else {
		result += "--------------------------------------\n"
		result += fmt.Sprintf("Successfully received message: %d \n", MessageReceivedCounter)
		result += fmt.Sprintf("End time: %s \n", test_end_time)
		result += fmt.Sprintf("Min delay: %d s\n", min_delay)
		result += fmt.Sprintf("Max delay: %d s\n", max_delay)
		delayRate := 0.0
		if delay_count > 0 {
			delayRate = float64((under_delay_limit_count / delay_count * 1.0) * 100)
		}
		result += fmt.Sprintf("Delay under %d seconds: %0.f %%\n", delay_limit, delayRate)
		//result += 'Average delay: ' + (Math.floor(total_delay / delay_count) / 1000) + ' s\n';
		result += "--------------------------------------\n"
		//result += under_delay_limit_count + " -- " + delay_count;
	}
	return
}

func SentResultMsg() (result string) {
	if test_start_time == "" {
		result = "Test is not started yet.\n"
	} else {
		result += "--------------------------------------\n"
		result += "Successfully sent: " + fmt.Sprintf("%d", MessageSentCounter) + " \n"
		result += "Successfully sent ack: " + fmt.Sprintf("%d", MessageAckCounter) + " \n"
		result += "Start time: " + test_start_time + "\n"
		result += "--------------------------------------\n"
	}
	return
}

func Clear() {
	MessageSentCounter = 0
	MessageAckCounter = 0
	MessageReceivedCounter = 0
	test_start_time = ""
	test_end_time = ""
}

func Shutdown() {
	Connections.Range(func(k, v interface{}) bool {
		cli := v.(*TcpClient)
		cli.Close()
		return true
	})
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
	once     sync.Once
	r        *bufio.Reader
}

func (c *TcpClient) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *TcpClient) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *TcpClient) Close() {
	c.once.Do(func() {
		c.logined = false
		close(c.quit)
		close(c.message)
		c.conn.Close()
		Connections.Delete(c.userId)
		OnlineUsers.Delete(c.userId)
	})
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
