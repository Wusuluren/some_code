// mini-proxy：GLM Web Code 核心转发链路的教学版（单文件、无 UI）。
//
// 原理（对照原项目 internal/cdpbridge/poller.go + internal/tools/runner.go + app_tool_result.go）：
//
//	Chrome(--remote-debugging-port=9222) 打开 chatglm.cn
//	        ▲                        │
//	        │ ③回灌 [TOOL RESULT]     │ ①CDP 轮询抓取 ```glm-web-code 块
//	        │  Input.insertText+Enter │  Runtime.evaluate
//	   ┌────┴────────────────────────▼────┐
//	   │            本 Go 进程             │
//	   │  ②goja 沙箱执行 JS（read/write/  │
//	   │    bash 工具经桥接函数注入）       │
//	   └───────────────────────────────────┘
//
// 运行：
//
//	// 1. 起一个开 CDP 端口的 Chrome（专用 profile，避免影响日常浏览器）
//	# macOS:
//	/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
//	  --remote-debugging-port=9222 --user-data-dir=/tmp/chrome-cdp &
//	# 2. 在该 Chrome 里打开 chatglm.cn 并登录，新建一个会话
//	# 3. 把下面 prompt 常量整段发给 AI（启动时也会打印）
//	# 4. go run .  然后对 AI 说"列出当前目录的文件"，它会发代码块，本进程执行后自动回灌
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/dop251/goja"
	"golang.org/x/net/websocket"
)

const (
	cdpPort     = 9222
	domainHint  = "chatglm.cn"
	pollEvery   = 1500 * time.Millisecond // 轮询间隔（原版 600ms）
	runDeadline = 30 * time.Second        // 沙箱总截止（原版 60s）
	outputLimit = 8000                    // 回灌截断（原版 20000）
)

// prompt 是人和 AI 之间的"工具调用协议"，等价原项目的系统提示词。
// 注意：内容含 ``` 围栏，不能用 Go 反引号字符串，故用拼接。
const fence = "```"
const prompt = "你是一个能操作本地电脑的 AI。当你需要执行操作时，输出一个 " + fence + "glm-web-code 围栏代码块，内容是 JavaScript。\n" +
	"代码块里可用的全局函数：log(x) 打印、await readFile(path)、await writeFile(path, content)、await bash(cmd)。\n" +
	"你的代码块会被本地程序执行，结果会以 [TOOL RESULT] 开头的消息回到对话里，你据此继续下一步。\n" +
	"首次使用时先 await bash(\"pwd\") 确认环境。"

// ---------- CDP 客户端（浏览器级 WebSocket，公开协议 Chrome DevTools Protocol） ----------

type cdpTarget struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	URL   string `json:"url"`
	WsURL string `json:"webSocketDebuggerUrl"`
}

// findChatTarget 走 CDP 公开的 HTTP 端点 /json/list 枚举页面。
func findChatTarget() (*cdpTarget, error) {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", cdpPort))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var targets []cdpTarget
	if err := json.Unmarshal(body, &targets); err != nil {
		return nil, err
	}
	for i := range targets {
		t := targets[i]
		if t.Type == "page" && strings.Contains(t.URL, domainHint) && t.WsURL != "" {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("找不到含 %s 的页面标签（先在 CDP Chrome 里打开并登录）", domainHint)
}

// cdpCall 在页面级 ws 上发一个 CDP 方法并等对应 id 的响应（事件帧无 id，跳过）。
func cdpCall(ws *websocket.Conn, id int, method string, params map[string]interface{}) (json.RawMessage, error) {
	frame, _ := json.Marshal(map[string]interface{}{"id": id, "method": method, "params": params})
	_ = ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := websocket.Message.Send(ws, string(frame)); err != nil {
		return nil, err
	}
	_ = ws.SetReadDeadline(time.Now().Add(10 * time.Second))
	for {
		var msg string
		if err := websocket.Message.Receive(ws, &msg); err != nil {
			return nil, err
		}
		var recv struct {
			ID     *int            `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(msg), &recv) != nil || recv.ID == nil || *recv.ID != id {
			continue
		}
		if recv.Error != nil {
			return nil, fmt.Errorf("%s: %s", method, recv.Error.Message)
		}
		return recv.Result, nil
	}
}

// evalJS 执行 Runtime.evaluate（returnByValue：结果直接 JSON 序列化回来）。
func evalJS(ws *websocket.Conn, id *int, expression string) (string, error) {
	*id++
	raw, err := cdpCall(ws, *id, "Runtime.evaluate", map[string]interface{}{
		"expression": expression, "returnByValue": true,
	})
	if err != nil {
		return "", err
	}
	// CDP 响应形如 {"result": {"result": {"value": ...}}}，两层 result 都来自不同结构体
	var wrap struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
	}
	_ = json.Unmarshal(raw, &wrap)
	if len(wrap.Result.Value) == 0 {
		return "", nil
	}
	var s string
	if json.Unmarshal(wrap.Result.Value, &s) == nil {
		return s, nil
	}
	return string(wrap.Result.Value), nil
}

// ---------- ① 抓取：把 AI 最新回复里的 glm-web-code 块读出来 ----------

// 原版靠精确 DOM 选择器 + 稳定门控；教学版简化为：DOM lang 属性优先，
// 全页文本正则兜底（会命中用户消息里的围栏，靠下方 hash 去重缓解）。
const scrapeJS = `(() => {
  const hits = [...document.querySelectorAll('div[lang]')]
    .filter(d => (d.getAttribute('lang')||'').toLowerCase() === 'glm-web-code');
  let code = '';
  if (hits.length) {
    const d = hits[hits.length-1];
    const c = d.querySelector('pre code') || d;
    code = c.textContent || '';
  }
  if (!code.trim()) {
    // 正则里的反引号用 unicode 转义写：裸反引号会截断 Go 的 raw string（原版同款技巧）
    const re = /\u0060{3}glm-web-code[^\n]*\n([\s\S]*?)\u0060{3}/g;
    let m; while ((m = re.exec(document.body.innerText))) code = m[1];
  }
  return code.trim();
})()`

// ---------- ② 执行：goja 沙箱（原版 internal/tools/runner.go 的最小复刻） ----------

func runSandbox(code string) string {
	vm := goja.New()

	// 加固：封死 eval / Function / .constructor 字符串编译逃逸，
	// 防 AI 生成的代码在沙箱里再生成代码（原版 sandboxHardenJS 同款思路）。
	_, _ = vm.RunString(`
	  const deny = () => { throw new Error('code generation disabled'); };
	  try { Object.defineProperty(globalThis, 'Function', {value: deny}); } catch(e){}
	  try { Object.defineProperty(globalThis, 'eval', {value: undefined}); } catch(e){}`)

	_ = vm.Set("log", func(call goja.FunctionCall) goja.Value {
		parts := make([]string, 0, len(call.Arguments))
		for _, a := range call.Arguments {
			parts = append(parts, a.String())
		}
		log.Printf("[sandbox] %s", strings.Join(parts, " "))
		return goja.Undefined()
	})
	// 工具即普通 Go 闭包 —— 原版统一走 __hostBridge(name, json) 查注册表，
	// 这里直接一函数一工具，省去注册表这层抽象。
	_ = vm.Set("readFile", func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			panic(err) // goja 把 panic 转成 JS exception
		}
		return string(b)
	})
	_ = vm.Set("writeFile", func(path, content string) string {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			panic(err)
		}
		return "ok: 已写入 " + path
	})
	_ = vm.Set("bash", func(cmd string) string {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
		if err != nil {
			return fmt.Sprintf("exit error: %v\n%s", err, out)
		}
		return string(out)
	})

	// 包 async IIFE 支持顶层 await；deadline 到点用 vm.Interrupt 强杀死循环。
	done := make(chan string, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- "panic: " + fmt.Sprint(r)
			}
		}()
		v, err := vm.RunString("(async () => {\n" + code + "\n})()")
		if err != nil {
			done <- "error: " + err.Error()
			return
		}
		// async 函数返回 Promise，轮询等它 settled（原版同做法）
		if p, ok := v.Export().(*goja.Promise); ok {
			for p.State() == goja.PromiseStatePending {
				time.Sleep(5 * time.Millisecond)
			}
			if p.State() == goja.PromiseStateRejected {
				done <- "error: " + p.Result().String()
				return
			}
			v = p.Result()
		}
		if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
			done <- "(执行完成，无返回值)"
		} else {
			done <- v.String()
		}
	}()
	timer := time.AfterFunc(runDeadline, func() { vm.Interrupt("超时 30s，已中断") })
	defer timer.Stop()

	select {
	case out := <-done:
		if len(out) > outputLimit {
			out = out[:outputLimit] + "\n...[截断]"
		}
		return out
	case <-time.After(runDeadline + 5*time.Second):
		return "error: 兜底超时"
	}
}

// ---------- ③ 回灌：结果写回输入框并模拟敲回车 ----------

func pushResult(ws *websocket.Conn, id *int, output string) error {
	text := "[TOOL RESULT]\n" + output
	// 先聚焦输入框（原版选择器 textarea.scroll-display-none，站点改版即失效，这里放宽）
	if _, err := evalJS(ws, id, `(() => {
	  const ta = document.querySelector('textarea.scroll-display-none') || document.querySelector('textarea');
	  if (ta) ta.focus();
	  return ta ? 'ok' : 'no-textarea';
	})()`); err != nil {
		return err
	}
	*id++
	if _, err := cdpCall(ws, *id, "Input.insertText", map[string]interface{}{"text": text}); err != nil {
		return fmt.Errorf("Input.insertText: %w", err)
	}
	time.Sleep(500 * time.Millisecond)
	// 按下再松开 = 一次 Enter，触发网页的"发送"
	for _, typ := range []string{"keyDown", "keyUp"} {
		*id++
		_, _ = cdpCall(ws, *id, "Input.dispatchKeyEvent", map[string]interface{}{
			"type": typ, "key": "Enter", "code": "Enter",
			"windowsVirtualKeyCode": 13, "nativeVirtualKeyCode": 13,
		})
	}
	return nil
}

// ---------- 主循环 ----------

func main() {
	log.Println("=== 使用说明 ===\n" + prompt + "\n================")
	seen := map[string]bool{} // hash 去重：抓到的块可能连续几轮不变（AI 生成中/已执行过）
	ws, msgID := (*websocket.Conn)(nil), 0
	dial := func() {
		t, err := findChatTarget()
		if err != nil {
			log.Printf("[cdp] %v（3s 后重试）", err)
			return
		}
		conn, err := websocket.Dial(t.WsURL, "", fmt.Sprintf("http://127.0.0.1:%d/", cdpPort))
		if err != nil {
			log.Printf("[cdp] dial %s: %v", t.URL, err)
			return
		}
		conn.MaxPayloadBytes = 16 << 20
		ws, msgID = conn, 0
		log.Printf("[cdp] 已连接页面 %s，开始轮询", t.URL)
	}
	dial()

	for {
		time.Sleep(pollEvery)
		if ws == nil {
			dial()
			continue
		}
		code, err := evalJS(ws, &msgID, scrapeJS)
		if err != nil {
			log.Printf("[cdp] 连接断开: %v", err)
			_ = ws.Close()
			ws = nil
			continue
		}
		if strings.TrimSpace(code) == "" {
			continue
		}
		// 流式输出时同一块会逐字变长，hash 只认"完全没见过的最终形态"；
		// 原版还叠加 loading 门控 + 稳定窗口，教学版省略，够用。
		sum := sha256.Sum256([]byte(code))
		key := hex.EncodeToString(sum[:])
		if seen[key] {
			continue
		}
		seen[key] = true
		log.Printf("[抓取] 新代码块 %d 字符:\n%s", len(code), code)

		output := runSandbox(code)
		log.Printf("[执行结果] %s", output)

		if err := pushResult(ws, &msgID, output); err != nil {
			log.Printf("[回灌失败] %v（结果已作废，重连后可让 AI 重发）", err)
			delete(seen, key)
			_ = ws.Close()
			ws = nil
			continue
		}
		log.Printf("[回灌] 成功，等待 AI 继续...")
	}
}
