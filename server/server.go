package server

import (
	"go-testing/client"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	SC = make(chan os.Signal, 1)
)

func StartHttpSrv(port string) {
	r := gin.Default()

	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.GET("/online", online)
	r.GET("/start", start)
	r.GET("/sentResult", sentResult)
	r.GET("/receivedResult", receivedResult)
	r.GET("/shutdown", shutdown)

	// port := fmt.Sprintf(":%04v", rand.New(rand.NewSource(time.Now().UnixNano())).Int31n(10000))

	// go monitor()

	r.Run(":" + port)
}

func online(c *gin.Context) {
	logNum, connNum := OnlineNum()
	c.JSON(http.StatusOK, gin.H{"conns": connNum, "authed": logNum, "sent": client.MessageSentCounter, "ack": client.MessageAckCounter})
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
	count := c.GetInt("count")
	if count == 0 {
		count = 1
	}
	client.SendAll(count)
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

// func monitor() {
// 	ticker := time.NewTicker(10 * time.Second)
// 	for {
// 		select {
// 		case <-ticker.C:
// 			log.Printf("当前在线连接数: %d", OnlineNum())
// 		}
// 	}
// }
