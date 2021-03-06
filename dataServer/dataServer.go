package main

import (
	"sync"
	"time"

	"../auth"
	"../config"
	"../iotcoap"
	"../iothttp"
	"../iotmqtt"
)

var wgAuth sync.WaitGroup

func main() {
	wgAuth.Add(1)
	go auth.SvrListen(&wgAuth)

	//http
	//路由部分
	router := iothttp.RouterRegister()
	//静态资源
	//router.Static("/static", "./linuxdashboard/godashboard")
	//运行的端口
	go router.Run(":8080")

	//coap
	go iotcoap.StartCoapServer("5683") //port 5683

	//mqtt
	go iotmqtt.StartMqttServer([]byte("1883")) //port 1883
	time.Sleep(time.Duration(2) * time.Second)
	if config.Cluster == true {
		go iotmqtt.ServerSubscriberCluster([]byte("127.0.0.1"), []byte("1883"), config.MqttTopic)
	} else {
		go iotmqtt.ServerSubscriberSingle([]byte("127.0.0.1"), []byte("1883"), config.MqttTopic)
	}

	//raw socket get it through other protocl

	wgAuth.Wait()

}
