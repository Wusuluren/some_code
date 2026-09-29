go run main.go # 匿名免费档（key=public）
UPSTREAM_KEY=xxx LISTEN=127.0.0.1:8081 go run ./...  # 用自己的 key
# 调用
curl http://127.0.0.1:8081/v1/chat/completions -H 'Content-Type: application/json' \
  -d '{"model":"mimo-v2.6-flash-free","messages":[{"role":"user","content":"你好"}]}'
# 输出
{"choices":[{"finish_reason":"stop","index":0,"message":{"content":"你好！很高兴为你服务。有什么我可以帮助你的吗？","role":"assistant"}}],"created":1790649965,"id":"gen-1790649965-RvdJSkQtl5JzpTIpvcx4","model":"mimo-v2.6-flash-free","object":"chat.completion","usage":{"completion_tokens":42,"prompt_tokens":231,"total_tokens":273}}
