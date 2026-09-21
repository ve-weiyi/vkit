// Package main 演示 knife4j 插件挂载接口文档 UI 的用法。
//
// 运行后访问 http://localhost:8088/api/v1/swagger/index.html
//
// 该目录以 _ 开头，Go 工具链会忽略它，不参与 build / vet / test。
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/ve-weiyi/vkit/plugins/knife4j"
)

// 实际的文档内容由业务方从 .api / .proto 生成，这里给一份最小样例
const docs = `
{
  "swagger": "2.0",
  "info": {
    "title": "",
    "version": ""
  },
  "paths": {
    "/api/v1/ping": {
      "get": {
        "summary": "ping",
        "operationId": "Ping"
      }
    }
  }
}
`

func main() {
	prefix := "/api/v1/swagger/"

	// 以下两个接口告知文档 UI 去哪里取文档，通常由业务方提供
	http.HandleFunc(fmt.Sprintf("%s%s", prefix, "swagger-resources"), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(knife4j.SwaggerResourcesText))
	})

	http.HandleFunc(fmt.Sprintf("%s%s", prefix, "v2/api-docs"), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(docs))
	})

	// 挂载 Knife4j 的静态资源与文档入口
	plugin := knife4j.NewKnife4jPlugin(docs)
	http.Handle(prefix, http.StripPrefix(prefix, plugin.Handler(prefix)))

	log.Println("接口文档地址: http://localhost:8088/api/v1/swagger/index.html")
	if err := http.ListenAndServe(":8088", nil); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
