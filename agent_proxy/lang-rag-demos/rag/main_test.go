package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rag-demo/rag"
)

func TestBuiltInCorpusIsFullyIndexed(t *testing.T) {
	idx, err := buildIndex("")
	if err != nil {
		t.Fatalf("buildIndex: %v", err)
	}
	docs, chunks, terms := idx.Stats()
	if docs != len(corpus) {
		t.Errorf("文档数 = %d, want %d", docs, len(corpus))
	}
	if chunks < docs {
		t.Errorf("块数 %d 少于文档数 %d", chunks, docs)
	}
	if terms == 0 {
		t.Error("词表为空")
	}
}

func TestAnswerOnlyUsesRetrievedMaterial(t *testing.T) {
	idx, err := buildIndex("")
	if err != nil {
		t.Fatalf("buildIndex: %v", err)
	}

	hits := idx.Search("余弦相似度怎么算", 3)
	if len(hits) == 0 {
		t.Fatal("没有召回")
	}
	if hits[0].DocID != "embedding" {
		t.Errorf("top1 = %s, want embedding", hits[0].DocID)
	}

	got := answer("余弦相似度怎么算", hits)
	if !strings.Contains(got, "[1]") {
		t.Errorf("答案应带资料编号: %q", got)
	}
	for _, part := range strings.Split(got, "；") {
		sentence := part[:strings.LastIndex(part, "[")]
		if !containsAny(hits, sentence) {
			t.Errorf("答案有资料外的内容: %q", sentence)
		}
	}
}

func TestRefusesQuestionOutsideTheCorpus(t *testing.T) {
	idx, err := buildIndex("")
	if err != nil {
		t.Fatalf("buildIndex: %v", err)
	}

	var buf bytes.Buffer
	show(idx, "明天杭州天气怎么样", 3, &buf)
	out := buf.String()
	if !strings.Contains(out, "判定资料里没有答案") {
		t.Errorf("语料外问题应走拒答分支:\n%s", out)
	}
	if strings.Contains(out, "答案：") {
		t.Errorf("拒答时不该给出答案:\n%s", out)
	}
}

func TestShowPrintsPromptAndCitations(t *testing.T) {
	idx, err := buildIndex("")
	if err != nil {
		t.Fatalf("buildIndex: %v", err)
	}

	var buf bytes.Buffer
	show(idx, "切块太大太小各有何问题", 2, &buf)
	out := buf.String()
	for _, want := range []string{"命中 chunking", "资料[1]", "出处：", "答案："} {
		if !strings.Contains(out, want) {
			t.Errorf("输出缺少 %q:\n%s", want, out)
		}
	}
}

func TestCustomCorpusDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "deploy.md"),
		[]byte("服务发布要先跑数据库迁移，再滚动重启实例。"), 0o644); err != nil {
		t.Fatal(err)
	}

	idx, err := buildIndex(dir)
	if err != nil {
		t.Fatalf("buildIndex(dir): %v", err)
	}
	hits := idx.Search("数据库迁移", 2)
	if len(hits) == 0 || hits[0].DocID != "deploy" {
		t.Errorf("应命中 deploy.md: %v", hits)
	}
}

// containsAny 判断片段是否原样出现在任一条检索到的资料里。
func containsAny(hits []rag.Result, sentence string) bool {
	for _, hit := range hits {
		if strings.Contains(hit.Text, sentence) {
			return true
		}
	}
	return false
}
