// miniproxy: 简化版 LLM API 转发代理（学习用，单文件，仅标准库）
//
// 原理参考 aicli 项目的 pkg/llm/openai.go —— 它作为"客户端"构造
// OpenAI 兼容请求发给上游；本程序把这层反过来，作为"服务端"收请求再转发：
//
//	客户端 --(OpenAI格式请求)--> miniproxy --(注入真实key后转发)--> 上游API
//	客户端 <--(流式SSE/JSON响应)-- miniproxy <--(原样回传)----------
//
// 运行：
//   go run main.go -upstream https://api.openai.com/v1 -key sk-xxx
//   然后任意 OpenAI 客户端把 base_url 指向 http://localhost:8080/v1 即可。
package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	listen := flag.String("listen", ":8080", "本地监听地址")
	upstream := flag.String("upstream", "https://api.openai.com/v1", "上游 API 的 base URL")
	key := flag.String("key", os.Getenv("UPSTREAM_API_KEY"), "上游真实 API Key（也可用环境变量）")
	flag.Parse()

	if *key == "" {
		log.Fatal("必须通过 -key 或 UPSTREAM_API_KEY 提供上游 API Key")
	}

	proxy := &llmProxy{
		upstream: *upstream,
		apiKey:   *key,
		// 注意不设 Client.Timeout：流式响应可能持续几分钟，
		// 超时交给每个请求的 context 控制。
		client: &http.Client{},
	}

	mux := http.NewServeMux()
	// 只转发 chat/completions 这一个核心接口，其余路径 404
	mux.HandleFunc("/v1/chat/completions", proxy.handle)

	log.Printf("miniproxy 监听 %s，转发到 %s", *listen, *upstream)
	log.Fatal(http.ListenAndServe(*listen, mux))
}

type llmProxy struct {
	upstream string
	apiKey   string
	client   *http.Client
}

func (p *llmProxy) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"only POST allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// 1. 读取客户端请求体。OpenAI 协议是纯 JSON，直接透传即可，
	//    不需要像 aicli 那样定义结构体重新序列化 —— 转发保持"无知"最稳。
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10MB 上限，防内存打满
	if err != nil {
		http.Error(w, `{"error":"read body failed"}`, http.StatusBadRequest)
		return
	}

	// 2. 构造对上游的请求（对应 aicli openai.go:101-109 的做法）
	upReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost,
		p.upstream+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		http.Error(w, `{"error":"build request failed"}`, http.StatusInternalServerError)
		return
	}
	upReq.Header.Set("Content-Type", "application/json")
	upReq.Header.Set("Authorization", "Bearer "+p.apiKey) // 关键：换成服务端持有的真实 key

	resp, err := p.client.Do(upReq)
	if err != nil {
		// 上游不可达：返回网关类错误
		http.Error(w, `{"error":"upstream unreachable: `+err.Error()+`"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 3. 把上游状态码和 Content-Type 原样回传
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)

	// 4. 流式回传响应体。
	//    - 非流式（stream=false）：上游返回一个完整 JSON，一次 copy 完。
	//    - 流式（stream=true）：上游返回 SSE（text/event-stream），
	//      这里逐块 copy 并 Flush，保证客户端实时收到每个 token，
	//      而不是等 Go 的 HTTP 缓冲区攒满。
	buf := make([]byte, 4096)
	rc := http.NewResponseController(w)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return // 客户端已断开
			}
			_ = rc.Flush()
		}
		if err == io.EOF {
			return
		}
		if err != nil {
			log.Printf("上游读取结束（非EOF）: %v", err)
			return
		}
	}
}
