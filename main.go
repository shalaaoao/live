package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// WebSocket 配置：允许跨域，方便局域网调试
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// 简单的内存存储，保存连接的客户端
// 在真实场景中应该用更复杂的房间管理，这里简化为：
// 一个 "sender" (推流者) 和多个 "receivers" (观众)
var (
	senderConn *websocket.Conn
	// key: viewerId, value: 该观众的 WebSocket 连接
	receivers = make(map[string]*websocket.Conn)
	mutex     sync.Mutex
)

// 定义信令消息结构
type SignalMessage struct {
	Type     string      `json:"type"`               // "offer", "answer", "candidate", "ready"
	Data     interface{} `json:"data"`               // SDP 或 ICE Candidate 数据
	ViewerID string      `json:"viewerId,omitempty"` // 哪个观众，对应前端的 viewerId
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 升级 HTTP 连接为 WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	// 获取角色参数 ?role=sender 或 ?role=receiver
	role := r.URL.Query().Get("role")
	// 对于 receiver，我们还会通过 ?id=xxx 标识这是哪个观众
	viewerID := r.URL.Query().Get("id")
	fmt.Printf("新连接接入: 角色 = %s, viewerID = %s\n", role, viewerID)

	mutex.Lock()
	if role == "sender" {
		senderConn = conn
		// 主播刚连上时，把当前所有已在线观众的 ready 状态推送给主播，
		// 避免观众先连、主播后连时，早期的 ready 消息丢失。
		for id := range receivers {
			readyMsg := SignalMessage{
				Type:     "ready",
				Data:     nil,
				ViewerID: id,
			}
			if err := senderConn.WriteJSON(readyMsg); err != nil {
				log.Println("向主播发送历史观众 ready 失败:", err, "viewerId =", id)
			}
		}
	} else if role == "receiver" {
		if viewerID != "" {
			receivers[viewerID] = conn
		}
	}
	mutex.Unlock()

	// 连接关闭时，清理全局引用
	defer func() {
		mutex.Lock()
		defer mutex.Unlock()

		if role == "sender" && senderConn == conn {
			senderConn = nil
		}
		if role == "receiver" && viewerID != "" {
			if c, ok := receivers[viewerID]; ok && c == conn {
				delete(receivers, viewerID)
			}
		}

		conn.Close()
	}()

	for {
		// 读取 JSON 消息
		var msg SignalMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("读取错误 (%s): %v", role, err)
			break
		}

		// 简单的转发逻辑（带上 viewerId，支持多个观众）
		mutex.Lock()
		if role == "sender" {
			// 主播发来的消息必须带 viewerId，转给对应观众
			if msg.ViewerID != "" {
				if rc, ok := receivers[msg.ViewerID]; ok {
					if err := rc.WriteJSON(msg); err != nil {
						log.Println("转发给观众失败:", err, "viewerId =", msg.ViewerID)
					}
				}
			}
		} else if role == "receiver" && senderConn != nil {
			// 观众发来的消息，统一打上 viewerId，再转给主播
			if viewerID != "" {
				msg.ViewerID = viewerID
			}
			if err := senderConn.WriteJSON(msg); err != nil {
				log.Println("转发给主播失败:", err)
			}
		}
		mutex.Unlock()
	}
}

func main() {
	// 1. 处理 WebSocket 信令
	http.HandleFunc("/ws", handleWebSocket)

	// 2. 托管静态文件 (HTML)
	// 将当前目录下的 public 文件夹作为静态资源目录
	fs := http.FileServer(http.Dir("./public"))
	http.Handle("/", fs)

	port := ":8080"
	fmt.Printf("Go 服务器启动! \n请在手机浏览器访问 http://你的电脑IP%s/sender.html (主播)\n", port)
	fmt.Printf("请在手机浏览器访问 http://你的电脑IP%s/receiver.html (观众)\n", port)

	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal("服务器启动失败: ", err)
	}
}
