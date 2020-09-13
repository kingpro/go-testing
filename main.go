package main

import (
	"flag"
	"fmt"
	"go-testing/client"
	"go-testing/server"
	"strconv"
	"time"

	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
)

var (
	addr        = flag.String("addr", "127.0.0.1:5280", "server addr")
	port        = flag.String("port", "1234", "monitor server port")
	node        = flag.String("node", "1", "current node flag")
	interval    = flag.Int("interval", 3, "send message every interval seconds.")
	connections = flag.Int("conn", 1, "number of tcp connections")
	resource    = flag.String("resource", "WEB", "resource of tcp connection")
	chatTips    = "this is a chat message."
)

func main() {
	log.Println(">>>>>>>>>>>>>>>>>>>>>>>>>>欢迎进入[go-testing]系统<<<<<<<<<<<<<<<<<<<<<<<<<<")
	flag.Parse()

	if *addr == "" {
		fmt.Printf("示例: go run main.go -addr 127.0.0.1:5280 -conn 1 \n")

		flag.Usage()

		return
	}

	runtime.GOMAXPROCS(runtime.NumCPU() - 1)

	log.Printf("连接到 %s", *addr)
	go server.StartHttpSrv(*port)

	wg := &sync.WaitGroup{}
	wg.Add(*connections)
	batch := 50
	for i := 0; i < *connections; i++ {
		if i > 0 && i%batch == 0 {
			time.Sleep(time.Second)
		}
		idNode, _ := strconv.Atoi(*node)
		go client.Connect(idNode*1000000+i, wg, *addr, *resource, chatTips, *interval)
	}
	wg.Wait()

	log.Println(">>>>>>>>>>>>>>>>>>>>>>>>>>[go-testing]系统运行中...<<<<<<<<<<<<<<<<<<<<<<<<<<")

	signal.Notify(server.SC,
		os.Kill,
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	<-server.SC
	log.Println(">>>>>>>>>>>>>>>>>>>>>>>>>>谢谢使用[go-testing]系统,Bye-bye!<<<<<<<<<<<<<<<<<<<<<<<<<<")
}
