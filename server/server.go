package server

import (
	"go-testing/client"
	"go-testing/dingtalk"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	SC    = make(chan os.Signal, 1)
	robot *dingtalk.Robot
)

func init() {
	accessToken := "7d2310aa380dbdbac8f226ce12be550d665d5d719f488d0168d246100cfb830b"
	secret := "SEC60a0748d9e030006a53dae9ef9876aa6a7d34b165d7d519da99b7cf6ed0a7a35"
	robot = dingtalk.NewRobot(accessToken, secret)
}

func StartHttpSrv(port string) {
	r := gin.Default()

	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.GET("/online", online)
	r.GET("/start", start)
	r.GET("/sentResult", sentResult)
	r.GET("/receivedResult", receivedResult)
	r.GET("/shutdown", shutdown)
	r.GET("/clear", clear)
	r.GET("/sendTextMessage", sendTextMessage)
	r.GET("/sendMarkdownMessage", sendMarkdownMessage)
	r.GET("/sendLinkMessage", sendLinkMessage)

	// port := fmt.Sprintf(":%04v", rand.New(rand.NewSource(time.Now().UnixNano())).Int31n(10000))
	// go monitor()

	r.Run(":" + port)
}

func online(c *gin.Context) {
	logNum, connNum := OnlineNum()
	c.JSON(http.StatusOK, gin.H{"conns": connNum, "authed": logNum, "sent": client.MessageSentCounter, "ack": client.MessageAckCounter, "received": client.MessageReceivedCounter})
}

func OnlineNum() (int, int) {
	logined_length, connected_length := 0, 0
	client.OnlineUsers.Range(func(k interface{}, v interface{}) bool {
		logined_length++

		return true
	})
	client.Connections.Range(func(k interface{}, v interface{}) bool {
		connected_length++
		return true
	})
	return logined_length, connected_length
}

func start(c *gin.Context) {
	// count := c.GetInt("count")
	count, ok := c.GetQuery("count")
	log.Println("消息发送个数：", count)
	countInt := 1
	if ok {
		countInt, _ = strconv.Atoi(count)
	}
	client.SendAll(countInt)
	c.String(http.StatusOK, "SEND OK\n")
}

func sentResult(c *gin.Context) {
	var result = client.SentResultMsg()
	c.String(http.StatusOK, result)
}

func receivedResult(c *gin.Context) {
	var result = client.ReceivedResultMsg()
	c.String(http.StatusOK, result)
}

func shutdown(c *gin.Context) {
	client.Shutdown()
	var result = "Bye~ \n"
	c.String(http.StatusOK, result)
	go func() {
		time.Sleep(time.Second)
		SC <- os.Interrupt
	}()
}

func clear(c *gin.Context) {
	client.Clear()
	c.String(http.StatusOK, "CLEAR OK \n")
}

//curl 'https://oapi.dingtalk.com/robot/send?access_token=xxxxxxxx' \
// -H 'Content-Type: application/json' \
// -d '{"msgtype": "text","text": {"content": "我就是我, 是不一样的烟火"}}'
func sendTextMessage(c *gin.Context) {
	robot.SendTextMessage("我就是我，不一样的烟火", []string{}, false)
	c.String(http.StatusOK, "SEND OK\n")
}

func sendMarkdownMessage(c *gin.Context) {
	robot.SendMarkdownMessage(
		"Markdown Test Title",
		"### Markdown 测试消息\n* 谷歌: [Google](https://www.google.com/)\n* 一张图片\n ![](https://avatars0.githubusercontent.com/u/40748346)",
		[]string{},
		false,
	)
	c.String(http.StatusOK, "SEND OK\n")
}

func sendLinkMessage(c *gin.Context) {
	robot.SendLinkMessage(
		"Link Test Title",
		"这是一条链接测试消息",
		"https://github.com/JetBlink",
		"https://avatars0.githubusercontent.com/u/40748346",
	)
	c.String(http.StatusOK, "SEND OK\n")
}
