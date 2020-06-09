package server

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	OnlineUsers        sync.Map
	Connections        sync.Map
	MessageSentCounter int64
	MessageAckCounter  int64
)

func StartHttpSrv(port string) {
	r := gin.Default()

	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.GET("/online", online)

	// port := fmt.Sprintf(":%04v", rand.New(rand.NewSource(time.Now().UnixNano())).Int31n(10000))

	// go monitor()

	r.Run(":" + port)
}

func online(c *gin.Context) {
	logNum, connNum := OnlineNum()
	c.JSON(http.StatusOK, gin.H{"conns": connNum, "authed": logNum, "sent": MessageSentCounter, "ack": MessageAckCounter})
}

func OnlineNum() (int, int) {
	logined_length, connected_length := 0, 0
	OnlineUsers.Range(func(k interface{}, v interface{}) bool {
		logined_length++
		return true
	})
	Connections.Range(func(k interface{}, v interface{}) bool {
		connected_length++
		return true
	})
	return logined_length, connected_length
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
