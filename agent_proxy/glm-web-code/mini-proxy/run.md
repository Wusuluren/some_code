# 1. 起一个开 CDP 调试端口的 Chrome（专用 profile，不影响日常浏览器）
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
  --remote-debugging-port=9222 \
  --user-data-dir=/tmp/chrome-cdp \
  https://chatglm.cn &

# 2. 验证 CDP 端口通了（应返回 JSON，含 chatglm.cn 的 target）
curl -s http://127.0.0.1:9222/json/list | head -20

# 3. 在新开的 Chrome 里登录智谱清言，新建一个会话，
#    把 mini-proxy 启动时打印的 prompt 全文（或运行下面这行取出来）发给 AI
cd /Users/wav/tmpwork/glm-web-code/mini-proxy && grep -A4 'const prompt' main.go

# 4. 跑代理
go run .

# 5. 对 AI 说："列出当前目录的文件" —— 它输出 glm-web-code 块后，
#    终端会依次打印 [抓取] → [sandbox] → [执行结果] → [回灌]，网页里 AI 收到 [TOOL RESULT] 继续回答
