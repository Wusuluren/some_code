// scli: 简化版"自然语言 → shell 命令"工具（学习用，单文件，仅标准库）
//
// 复刻 aicli 的核心链路，去掉配置系统/i18n/历史记录/多Provider：
//
//	输入 → 构建提示词(带OS/目录上下文) → 调 OpenAI 兼容 API
//	     → 清理 markdown 代码块 → 危险命令正则检查 → (确认后) sh -c 执行
//
// 对应源码：internal/app/app.go:49(Run) / pkg/llm/openai.go(Translate)
//
// 运行：
//   export OPENAI_API_KEY=sk-xxx            # 必填
//   export OPENAI_BASE_URL=https://api.openai.com/v1   # 可选，兼容中转
//   export SCLI_MODEL=gpt-4o                # 可选
//   go run . 查看当前目录下最大的3个文件
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// ---------- 1. 提示词构建（对应 pkg/llm/prompt.go） ----------

func buildMessages(input, workDir string) []map[string]string {
	system := `你是一个只输出 shell 命令的助手。规则：
1. 根据用户描述输出一条可直接执行的命令
2. 只输出命令本身，不要解释
3. 命令适配当前操作系统
当前环境：OS=` + runtime.GOOS + `，工作目录=` + workDir
	return []map[string]string{
		{"role": "system", "content": system},
		{"role": "user", "content": input},
	}
}

// ---------- 2. 调用 LLM（对应 pkg/llm/openai.go 的 Translate） ----------

func askLLM(ctx context.Context, input, workDir string) (string, error) {
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := os.Getenv("SCLI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	reqBody, _ := json.Marshal(map[string]any{
		"model":    model,
		"messages": buildMessages(input, workDir),
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("OPENAI_API_KEY"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API 返回 HTTP %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Choices []struct {
			Message struct{ Content string } `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || len(parsed.Choices) == 0 {
		return "", fmt.Errorf("解析响应失败: %s", string(body))
	}
	return cleanCommand(parsed.Choices[0].Message.Content), nil
}

// cleanCommand 去掉模型爱带的 markdown 代码块（对应 openai.go:158）
func cleanCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if strings.HasPrefix(cmd, "```") {
		cmd = strings.TrimPrefix(cmd, "```bash")
		cmd = strings.TrimPrefix(cmd, "```sh")
		cmd = strings.TrimPrefix(cmd, "```")
		if i := strings.Index(cmd, "\n"); i >= 0 { // 去掉围栏后的首行换行
			cmd = cmd[i+1:]
		}
		cmd = strings.TrimSuffix(strings.TrimSpace(cmd), "```")
	}
	return strings.TrimSpace(cmd)
}

// ---------- 3. 安全检查（对应 pkg/safety 的简化版） ----------

var dangerousPatterns = regexp.MustCompile(
	`(?i)\b(rm\s+(-[a-z]*[rf][a-z]*\s+)+/|mkfs(\.\w+)?|dd\s+if=.*of=/dev/|:\(\)\s*\{\s*:\|:&\};:|shutdown|reboot)`)

func isDangerous(cmd string) bool { return dangerousPatterns.MatchString(cmd) }

func confirm(cmd string) bool {
	fmt.Fprintf(os.Stderr, "⚠️  危险命令: %s\n确认执行? [y/N] ", cmd)
	var ans string
	fmt.Scanln(&ans)
	return strings.EqualFold(ans, "y")
}

// ---------- 4. 执行（对应 pkg/executor：sh -c + 输出透传） ----------

func runCommand(cmd string) error {
	c := exec.Command("/bin/sh", "-c", cmd)
	c.Stdout = os.Stdout // 直接接到终端，实时看到输出
	c.Stderr = os.Stderr
	return c.Run()
}

// ---------- main：串起链路（对应 internal/app/app.go 的 Run） ----------

func main() {
	input := strings.TrimSpace(strings.Join(os.Args[1:], " "))
	if input == "" {
		fmt.Fprintln(os.Stderr, "用法: scli <自然语言描述>")
		os.Exit(2)
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "请先设置 OPENAI_API_KEY")
		os.Exit(2)
	}

	workDir, _ := os.Getwd()

	// 超时控制：context 一路传给 HTTP 请求（对应 app.go:74-79）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	command, err := askLLM(ctx, input, workDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "翻译失败:", err)
		os.Exit(1)
	}
	if command == "" {
		fmt.Fprintln(os.Stderr, "模型返回空命令")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "➜ %s\n", command) // 展示命令走 stderr，不污染管道输出

	if isDangerous(command) && !confirm(command) {
		fmt.Fprintln(os.Stderr, "已取消")
		os.Exit(1)
	}

	if err := runCommand(command); err != nil {
		fmt.Fprintln(os.Stderr, "执行失败:", err)
		os.Exit(1)
	}
}
