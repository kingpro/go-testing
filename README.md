# go-testing

golang tcp client , use protobuf demo

# 生成pb对应的go文件

```
cd protobuf
protoc --go_out=../pb im_message.proto

protoc --go_out=pb protobuf/im_message.proto
```

# 编译

```
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build
```

# 运行

```
go run main.go -addr 121.37.248.59:5280 -conn 1 -port 1001 -node 1
go run main.go -addr 121.37.248.59:5280 -conn 1 -port 1002 -node 2

```