// Package rag 是一个零依赖的最小 RAG 引擎：切块 -> 词袋向量化 -> 余弦相似度召回 -> 拼检索增强 prompt。
//
// 它演示的是 RAG 的形状而不是效果：真实项目把 Tokens 换成 embedding 模型、
// 把 map 向量换成稠密向量和向量库，其余流程一模一样。
package rag

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Options 是索引参数。
type Options struct {
	ChunkSize int // 每个块最多多少个字符，默认 200
	Overlap   int // 相邻块保留多少字符重叠，默认 ChunkSize/8
}

// Result 是一条召回结果。
type Result struct {
	DocID string  // 出处标识，通常是文件名或文档名
	Chunk int     // 该文档的第几块
	Score float64 // 余弦相似度，0 到 1
	Text  string  // 命中的原文片段
	Terms []string
}

type indexed struct {
	docID string
	no    int
	text  string
	vec   map[string]float64 // 已按 L2 归一化的 tf-idf 向量
}

// Index 是文档索引。用法：New -> Add/AddFile -> Build -> Search。
type Index struct {
	opts    Options
	pending []pendingChunk
	chunks  []indexed
	idf     map[string]float64
	terms   int
	built   bool
}

type pendingChunk struct {
	docID string
	no    int
	text  string
}

// New 创建空索引。
func New(opts Options) *Index {
	if opts.ChunkSize <= 0 {
		opts.ChunkSize = 200
	}
	if opts.Overlap < 0 || opts.Overlap >= opts.ChunkSize {
		opts.Overlap = opts.ChunkSize / 8
	}
	return &Index{opts: opts, idf: map[string]float64{}}
}

// Add 把一段文本切块后加入待建索引。
func (x *Index) Add(docID, text string) int {
	var added int
	for i, chunk := range SplitChunks(text, x.opts.ChunkSize, x.opts.Overlap) {
		x.pending = append(x.pending, pendingChunk{docID: docID, no: i, text: chunk})
		added++
	}
	x.built = false
	return added
}

// AddFile 读取一个文件，用文件名（去掉扩展名）当作出处标识。
func (x *Index) AddFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("rag: 读取 %s: %w", path, err)
	}
	x.Add(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), string(raw))
	return nil
}

// AddDir 读取目录下所有 .md 与 .txt 文件建索引，文件名即出处。
// dir 由本地命令行传入，不做网络输入处理。
func (x *Index) AddDir(dir string) error {
	var names []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".md", ".txt":
			names = append(names, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("rag: 遍历 %s: %w", dir, err)
	}
	sort.Strings(names)

	for _, path := range names {
		if err := x.AddFile(path); err != nil {
			return err
		}
	}
	return nil
}

// Build 统计词频、算 IDF，并把每个块转成归一化的 tf-idf 向量。
func (x *Index) Build() error {
	if len(x.pending) == 0 {
		return fmt.Errorf("rag: 索引为空，先 Add 文档再 Build")
	}

	// 每个块的词频。
	tfs := make([]map[string]int, len(x.pending))
	df := map[string]int{}
	for i, p := range x.pending {
		counts := freq(Tokens(p.text))
		tfs[i] = counts
		for term := range counts {
			df[term]++
		}
	}

	n := float64(len(x.pending))
	x.idf = make(map[string]float64, len(df))
	for term, docs := range df {
		// BM25 形式的 IDF：出现块越少权重越高，而且始终不为 0。
		// 若用 log(N/df)，只有一个块时所有词的 IDF 都是 0，整个索引召不出东西。
		x.idf[term] = math.Log(1 + (n-float64(docs)+0.5)/(float64(docs)+0.5))
	}

	x.chunks = make([]indexed, 0, len(x.pending))
	for i, p := range x.pending {
		x.chunks = append(x.chunks, indexed{
			docID: p.docID,
			no:    p.no,
			text:  p.text,
			vec:   normalize(weight(tfs[i], x.idf)),
		})
	}
	x.terms = len(df)
	x.built = true
	return nil
}

// Search 返回与查询最相近的前 k 个块，按分数从高到低排列。
// 词完全不相交的块分数为 0，直接丢掉。
func (x *Index) Search(query string, k int) []Result {
	if !x.built || k <= 0 {
		return nil
	}

	qCounts := freq(Tokens(query))
	if len(qCounts) == 0 {
		return nil
	}
	qVec := normalize(weight(qCounts, x.idf))

	var hits []Result
	for _, c := range x.chunks {
		score := dot(qVec, c.vec)
		if score <= 0 {
			continue
		}
		hits = append(hits, Result{
			DocID: c.docID,
			Chunk: c.no,
			Score: score,
			Text:  c.text,
			Terms: shared(qCounts, c.vec),
		})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			if hits[i].DocID == hits[j].DocID {
				return hits[i].Chunk < hits[j].Chunk
			}
			return hits[i].DocID < hits[j].DocID
		}
		return hits[i].Score > hits[j].Score
	})
	if len(hits) > k {
		hits = hits[:k]
	}
	return hits
}

// Stats 返回索引规模，用于打印和断言。
func (x *Index) Stats() (docs, chunks, terms int) {
	seen := map[string]bool{}
	for _, c := range x.chunks {
		seen[c.docID] = true
	}
	return len(seen), len(x.chunks), x.terms
}

// BuildPrompt 拼出检索增强提示：资料带编号列出，并要求答案标注引用。
// 这一步就是 RAG 里的 Augmented：知识进了上下文，模型不需要"记得"。
func BuildPrompt(question string, hits []Result) string {
	var sb strings.Builder
	sb.WriteString("只根据下面资料回答。资料里没有的，直接说不知道。\n\n")
	for i, hit := range hits {
		fmt.Fprintf(&sb, "资料[%d] 出处 %s#%d：%s\n", i+1, hit.DocID, hit.Chunk, hit.Text)
	}
	sb.WriteString("\n回答时用上标的资料编号标注出处。\n问题：")
	sb.WriteString(question)
	return sb.String()
}

// Citations 给出与 BuildPrompt 编号一致的出处列表。
func Citations(hits []Result) []string {
	out := make([]string, 0, len(hits))
	for _, hit := range hits {
		out = append(out, fmt.Sprintf("%s#%d", hit.DocID, hit.Chunk))
	}
	return out
}

// weight 把词频换算成 tf-idf 权重，tf 用频率形式以免长块天然占优。
func weight(counts map[string]int, idf map[string]float64) map[string]float64 {
	total := 0
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		return map[string]float64{}
	}

	out := make(map[string]float64, len(counts))
	for term, count := range counts {
		idfValue, ok := idf[term]
		if !ok {
			continue // 查询里的新词在语料中没出现，权重为 0
		}
		out[term] = float64(count) / float64(total) * idfValue
	}
	return out
}

func normalize(vec map[string]float64) map[string]float64 {
	var sum float64
	for _, v := range vec {
		sum += v * v
	}
	norm := math.Sqrt(sum)
	if norm == 0 {
		return vec
	}
	for term, v := range vec {
		vec[term] = v / norm
	}
	return vec
}

func dot(a, b map[string]float64) float64 {
	// 只遍历短的那一边。
	if len(a) > len(b) {
		a, b = b, a
	}
	var sum float64
	for term, v := range a {
		sum += v * b[term]
	}
	return sum
}

// shared 列出查询与片段共现的词，用来解释这条结果为什么被召回。
func shared(queryCounts map[string]int, vec map[string]float64) []string {
	var out []string
	for term := range queryCounts {
		if vec[term] > 0 {
			out = append(out, term)
		}
	}
	sort.Strings(out)
	return out
}
