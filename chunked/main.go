package main

import (
	"fmt"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Transfer-Encoding", "chunked")

	// 添加大量自定义头信息以增加头部大小
	for i := 0; i < 100; i++ { // 假设每个头字段占用约64字节
		key := fmt.Sprintf("Custom-Header-%d", i)
		value := fmt.Sprintf("Value%d", i)
		w.Header().Set(key, value)
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	for i := 0; i < 5; i++ { // 发送5个数据块作为示例
		fmt.Fprintf(w, "This is chunk %d\n", i+1)
		flusher.Flush()
	}
}

func main() {
	http.HandleFunc("/", handler)
	fmt.Println("Server running on port 9999...")
	err := http.ListenAndServe(":9999", nil)
	if err != nil {
		panic(err)
	}
}
