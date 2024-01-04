package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/hertz-contrib/sse"
)

func main() {
	h := server.Default()
	// h.LoadHTMLGlob("frontend/view/*")
	h.Static("/", "frontend/view")
	// h.GET("/index", func(c context.Context, ctx *app.RequestContext) {
	// 	ctx.HTML(consts.StatusOK, "index.html", utils.H{
	// 		"title": "Main website",
	// 	})
	// })

	h.GET("/ping", func(c context.Context, ctx *app.RequestContext) {
		ctx.JSON(consts.StatusOK, utils.H{"message": "pong"})
	})

	// h.GET("/sse/v1", func(c context.Context, ctx *app.RequestContext) {
	// 	ctx.Response.Header.Set("Content-Type", "text/event-stream")
	// 	ctx.Response.Header.Set("Cache-Control", "no-cache")
	// 	ctx.Response.Header.Set("Connection", "keep-alive")
	// 	// Hijack the writer of Response
	// 	ctx.Response.HijackWriter(resp.NewChunkedBodyWriter(&ctx.Response, ctx.GetWriter()))
	//
	// 	for i := 0; i < 10; i++ {
	// 		sendData(c, ctx, map[string]string{
	// 			fmt.Sprintf("%d", i): "0",
	// 		})
	// 		if i == 8 {
	// 			time.Sleep(30 * time.Second)
	// 		}
	// 		time.Sleep(200 * time.Millisecond)
	// 	}
	// })

	h.GET("/sse/v2", func(ctx context.Context, c *app.RequestContext) {
		// 客户端可以通过 Last-Event-ID 告知服务器收到的最后一个事件
		lastEventID := sse.GetLastEventID(c)
		hlog.CtxInfof(ctx, "last event ID: %s", lastEventID)

		// 在第一次渲染调用之前必须先行设置状态代码和响应头文件
		c.SetStatusCode(http.StatusOK)
		sChan, done := NewStreamHandler(c)

		for i := 0; i < 10; i++ {
			bytes, _ := json.Marshal(map[string]string{
				fmt.Sprintf("%d", i): "0",
			})
			event := &sse.Event{
				ID:   fmt.Sprintf("%d", i),
				Data: []byte(bytes),
			}
			sChan <- event
			if i == 8 {
				time.Sleep(10 * time.Second)
				sChan <- &sse.Event{
					Event: "slow_message",
					ID:    fmt.Sprintf("%d", i),
					Data:  []byte(bytes),
				}
			}
			time.Sleep(200 * time.Millisecond)
		}
		sChan <- &sse.Event{
			ID:   "10",
			Data: []byte("DONE"),
		}
		close(sChan)
		<-done
	})
	h.Spin()
}

func NewStreamHandler(c *app.RequestContext) (chan<- *sse.Event, <-chan interface{}) {
	s := sse.NewStream(c)

	respChan := make(chan *sse.Event)
	done := make(chan interface{})
	go func() {
		defer close(done)
		const KeepAliveSec = 3
		ticker := time.NewTicker(KeepAliveSec * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				err := s.Publish(&sse.Event{Data: []byte("ping")})
				if err != nil {
					fmt.Println("publish time err %w", err)
					return
				}
			case event, ok := <-respChan:
				if !ok {
					fmt.Println("close")
					return
				}
				ticker.Reset(KeepAliveSec * time.Second)
				fmt.Println("before pub", event)
				err := s.Publish(event)
				fmt.Println("after pub", event)
				if err != nil {
					fmt.Println("publish err %w", err, event)
					return
				}
			}
		}

	}()

	return respChan, done
}

func sendData(c context.Context, ctx *app.RequestContext, data any) {
	bytes, _ := json.Marshal(data)
	_, err := ctx.Write([]byte(fmt.Sprintf("data: %s\n\n", string(bytes))))
	if err != nil {
		fmt.Println("write err %w", err)
	}
	err = ctx.Flush()
	if err != nil {
		fmt.Println("flush err %w", err)
	}
}
