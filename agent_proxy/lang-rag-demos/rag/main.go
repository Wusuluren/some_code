// RAG mini demo（零依赖，纯 Go，不联网、不要 API key）
//
// 演示 RAG 的四步形状：
//  1. 切块 chunk：长文切成有重叠的小块
//  2. 向量化：这里用 TF-IDF 词袋代替 embedding 模型
//  3. 召回：查询向量与块向量算余弦相似度，取 top-k
//  4. 增强生成：把命中片段带编号塞进 prompt，答案标出处
//
// 换成语义检索时，只有第 2 步要改：Tokens 换成 embedding，map 换成稠密向量。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"rag-demo/rag"
)

// corpus 是内置语料，够小，能一眼看清召回为什么命中。
var corpus = []struct{ ID, Text string }{
	{"rag", "RAG 全称检索增强生成。回答前先从外部资料里检索相关片段，拼进 prompt，再让模型照着片段作答。它不改模型权重，知识更新只要重新索引。"},
	{"chunking", "切块决定检索粒度。块太大，噪声多且塞不进上下文；块太小，语义被切碎。常见做法是按句子边界切，并在相邻块之间保留重叠。"},
	{"embedding", "embedding 模型把句子压成稠密向量，语义接近的句子向量距离也近。余弦相似度用来衡量两个向量方向是否一致，是向量召回的默认打分方式。"},
	{"langchain", "LangChain 是 LLM 应用开发库，内置 retriever、vectorstore 和 RetrievalQA，能直接拼出一条 RAG 管道。它的 Chain 是单向管道，没有回环。"},
	{"langgraph", "LangGraph 把流程画成带状态的图，支持回环、条件边和检查点。做 RAG 时可以用它对召回结果打分，质量不够就改写查询再检索一次。"},
	{"claudecode", "Claude Code 是 Anthropic 的编程 Agent。它检索代码不走向量库，而是让模型自己发 Grep、Glob、Read 调用，一轮一轮缩小范围，这叫 agentic 检索。"},
	{"finetune", "微调改的是模型权重，知识被写进参数里，更新要重新训练，答案也难以给出出处。它更适合固定输出风格和格式，不适合承载时效性知识。"},
}

const scoreCutoff = 0.05 // 低于这个分数就当作"资料里没有"

func main() {
	docs := flag.String("docs", "", "用目录里的 .md/.txt 作语料，留空则用内置语料")
	ask := flag.String("q", "", "只问这一个问题")
	k := flag.Int("k", 3, "召回片段数")
	flag.Parse()

	idx, err := buildIndex(*docs)
	if err != nil {
		exit(err)
	}

	total, chunks, terms := idx.Stats()
	fmt.Printf("索引规模：%d 篇文档，%d 个块，%d 个词\n\n", total, chunks, terms)

	if *ask != "" {
		show(idx, *ask, *k, os.Stdout)
		return
	}

	for _, question := range []string{
		"余弦相似度在向量召回里用来做什么？",
		"切块太大太小各有何问题",
		"Claude Code 检索代码走的是向量库吗",
		"明天杭州天气怎么样",
	} {
		show(idx, question, *k, os.Stdout)
	}
}

// buildIndex 建好索引：dir 为空用内置语料，否则用目录里的 .md/.txt。
func buildIndex(dir string) (*rag.Index, error) {
	idx := rag.New(rag.Options{ChunkSize: 120, Overlap: 20})
	if dir != "" {
		if err := idx.AddDir(dir); err != nil {
			return nil, err
		}
	} else {
		for _, doc := range corpus {
			idx.Add(doc.ID, doc.Text)
		}
	}
	if err := idx.Build(); err != nil {
		return nil, err
	}
	return idx, nil
}

// show 打印一次完整问答：召回 -> 判定 -> prompt -> 答案。
func show(idx *rag.Index, question string, k int, out io.Writer) {
	fmt.Fprintln(out, "== 问题："+question)
	hits := idx.Search(question, k)

	for _, hit := range hits {
		fmt.Fprintf(out, "  命中 %-11s#%d 分数 %.3f 共现词 %v\n", hit.DocID, hit.Chunk, hit.Score, hit.Terms)
	}

	if len(hits) == 0 || hits[0].Score < scoreCutoff {
		fmt.Fprintf(out, "  最高分低于阈值 %.2f，判定资料里没有答案，直接拒答，不编造\n\n", scoreCutoff)
		return
	}

	prompt := rag.BuildPrompt(question, hits)
	fmt.Fprintf(out, "  拼出的 prompt（%d 字，含 %d 段资料）：\n", len([]rune(prompt)), len(hits))
	for _, line := range strings.Split(prompt, "\n") {
		if line != "" {
			fmt.Fprintln(out, "    | "+line)
		}
	}
	fmt.Fprintf(out, "  答案：%s\n", answer(question, hits))
	fmt.Fprintf(out, "  出处：%s\n\n", strings.Join(rag.Citations(hits), ", "))
}

// answer 代替真实模型：从命中片段里挑与问题重合最多的句子拼成回答，并标注资料编号。
// 真实项目里这一步就是把 prompt 发给模型，其余逻辑不用动。
func answer(question string, hits []rag.Result) string {
	queryTerms := map[string]bool{}
	for _, term := range rag.Tokens(question) {
		queryTerms[term] = true
	}

	type candidate struct {
		sentence string
		cite     int
		score    int
	}
	var best []candidate
	for i, hit := range hits {
		for _, sentence := range rag.Sentences(hit.Text) {
			score := 0
			for _, term := range rag.Tokens(sentence) {
				if queryTerms[term] {
					score++
				}
			}
			if score > 0 {
				best = append(best, candidate{sentence: sentence, cite: i + 1, score: score})
			}
		}
	}
	if len(best) == 0 {
		return "资料里没有相关内容。"
	}

	sort.SliceStable(best, func(i, j int) bool { return best[i].score > best[j].score })
	if len(best) > 2 {
		best = best[:2]
	}

	parts := make([]string, 0, len(best))
	for _, c := range best {
		// 去掉句尾标点，拼起来才读得通。
		sentence := strings.TrimRight(c.sentence, "。；，, ")
		parts = append(parts, fmt.Sprintf("%s[%d]", sentence, c.cite))
	}
	return strings.Join(parts, "；")
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, "运行失败:", err)
	os.Exit(1)
}
