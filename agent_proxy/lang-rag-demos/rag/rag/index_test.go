package rag

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestTokensSplitsASCIIAndCJK(t *testing.T) {
	got := Tokens("LangChain 向量库 v0.1.15 检索")

	// ASCII 连续串整块保留并小写。
	if !contains(got, "langchain") || !contains(got, "v0.1.15") {
		t.Errorf("ASCII 词没切出来: %v", got)
	}
	// 中文走 bigram：向量库 -> 向量、量库。
	if !contains(got, "向量") || !contains(got, "量库") {
		t.Errorf("中文 bigram 缺失: %v", got)
	}
	if contains(got, " ") {
		t.Errorf("不应把空格切成词: %v", got)
	}
}

func TestSplitChunksRespectsSizeAndOverlap(t *testing.T) {
	text := strings.Repeat("这是一段用来测试切块的中文句子，长度足够被切成多块。", 12)
	chunks := SplitChunks(text, 60, 10)
	if len(chunks) < 2 {
		t.Fatalf("切成 %d 块，期望多块", len(chunks))
	}
	for i, c := range chunks {
		if n := len([]rune(c)); n > 60 {
			t.Errorf("第 %d 块长度 %d，超过上限 60", i, n)
		}
	}
	// 相邻块应共享内容，否则答案跨块时会漏。
	shared := false
	for i := 0; i+1 < len(chunks); i++ {
		tail := chunks[i][len(chunks[i])-10:]
		if strings.Contains(chunks[i+1], tail) {
			shared = true
		}
	}
	if !shared {
		t.Error("相邻块没有重叠内容")
	}
}

func newTestIndex(t *testing.T) *Index {
	t.Helper()
	idx := New(Options{ChunkSize: 120, Overlap: 20})
	idx.Add("embedding", "embedding 模型把句子压成稠密向量。余弦相似度用来衡量两个向量方向是否一致，是向量召回的默认打分方式。")
	idx.Add("chunking", "切块决定检索粒度。块太大噪声多，块太小语义被切碎。常见做法是按句子边界切，并保留重叠。")
	idx.Add("finetune", "微调改的是模型权重，知识被写进参数里，更新要重新训练，答案难以给出出处。")
	if err := idx.Build(); err != nil {
		t.Fatalf("Build: %v", err)
	}
	return idx
}

func TestSearchRanksTheRightDocFirst(t *testing.T) {
	idx := newTestIndex(t)

	hits := idx.Search("余弦相似度怎么算向量", 3)
	if len(hits) == 0 {
		t.Fatal("没有召回结果")
	}
	if hits[0].DocID != "embedding" {
		t.Errorf("top1 = %s, want embedding（结果 %v）", hits[0].DocID, summarize(hits))
	}
	if hits[0].Score <= 0 || hits[0].Score > 1 {
		t.Errorf("余弦相似度应落在 (0,1]，得到 %v", hits[0].Score)
	}
	if !strings.Contains(hits[0].Text, "余弦") {
		t.Errorf("片段文本不对: %q", hits[0].Text)
	}

	// 分数按降序排列。
	for i := 1; i < len(hits); i++ {
		if hits[i].Score > hits[i-1].Score {
			t.Errorf("第 %d 条分数反弹: %v", i, summarize(hits))
			break
		}
	}
}

func TestSearchIgnoresUnrelatedQuery(t *testing.T) {
	idx := newTestIndex(t)

	if hits := idx.Search("西红柿炒鸡蛋怎么做", 3); len(hits) != 0 {
		t.Errorf("语料里没有的词不该有召回: %v", summarize(hits))
	}
	if hits := idx.Search("", 3); len(hits) != 0 {
		t.Errorf("空查询应返回空: %v", summarize(hits))
	}
	if hits := idx.Search("向量", 0); hits != nil {
		t.Error("k=0 应返回空")
	}
}

func TestBuildEmptyIndexFails(t *testing.T) {
	if err := New(Options{}).Build(); err == nil {
		t.Error("空索引 Build 应报错")
	}
}

func TestPromptAndCitationsLineUp(t *testing.T) {
	idx := newTestIndex(t)
	hits := idx.Search("切块太大太小", 2)
	if len(hits) == 0 {
		t.Fatal("没有召回")
	}

	prompt := BuildPrompt("切块太大太小各有何问题", hits)
	if !strings.Contains(prompt, "资料[1] 出处 "+hits[0].DocID) {
		t.Errorf("prompt 缺少带编号的资料:\n%s", prompt)
	}
	if !strings.Contains(prompt, "只根据下面资料回答") {
		t.Error("prompt 缺少约束语，模型会自由发挥")
	}
	if !strings.HasSuffix(prompt, "切块太大太小各有何问题") {
		t.Error("prompt 末尾应接上原始问题")
	}

	cites := Citations(hits)
	if len(cites) != len(hits) || cites[0] != hits[0].DocID+"#"+strconv.Itoa(hits[0].Chunk) {
		t.Errorf("出处与召回不一致: %v", cites)
	}
}

func TestAddDirReadsMarkdownFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"),
		[]byte("RAG 先检索再生成。知识更新只要重新索引。"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skip.log"), []byte("不该被索引"), 0o644); err != nil {
		t.Fatal(err)
	}

	idx := New(Options{ChunkSize: 60})
	if err := idx.AddDir(dir); err != nil {
		t.Fatalf("AddDir: %v", err)
	}
	if err := idx.Build(); err != nil {
		t.Fatalf("Build: %v", err)
	}

	docs, chunks, _ := idx.Stats()
	if docs != 1 || chunks < 1 {
		t.Errorf("期望 1 篇文档，得到 %d 篇 %d 块", docs, chunks)
	}
	hits := idx.Search("重新索引", 2)
	if len(hits) == 0 || hits[0].DocID != "notes" {
		t.Errorf("出处应是文件名 notes: %v", summarize(hits))
	}
}

func TestSharedTermsExplainsTheHit(t *testing.T) {
	idx := newTestIndex(t)
	hits := idx.Search("余弦相似度", 1)
	if len(hits) == 0 {
		t.Fatal("没有召回")
	}
	if !contains(hits[0].Terms, "余弦") {
		t.Errorf("共现词应解释召回原因: %v", hits[0].Terms)
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func summarize(hits []Result) []string {
	out := make([]string, 0, len(hits))
	for _, hit := range hits {
		out = append(out, hit.DocID+"#"+strconv.Itoa(hit.Chunk)+":"+strconv.Itoa(int(hit.Score*1000)))
	}
	return out
}
