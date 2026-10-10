package rag

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Tokens 把文本切成可比较的词。
//
// 真实 RAG 用 embedding 模型把句子压成稠密向量，这里没有模型，就退化成词袋：
//   - ASCII 字母数字串合成一个小写词，go、LangChain、v0.1.15 都能整块命中；
//   - 中日韩连续段切成二元组（bigram），"向量库" 出 "向量" "量库"，绕开中文分词。
//
// 换成语义检索时，只需要把 Tokens 换成 embedding 并改一下向量类型，索引与召回逻辑不动。
func Tokens(text string) []string {
	var out []string
	var ascii []rune
	var cjk []rune

	flushASCII := func() {
		if len(ascii) == 0 {
			return
		}
		out = append(out, strings.ToLower(string(ascii)))
		ascii = ascii[:0]
	}
	flushCJK := func() {
		if len(cjk) == 0 {
			return
		}
		if len(cjk) == 1 {
			out = append(out, string(cjk))
		} else {
			for i := 0; i+1 < len(cjk); i++ {
				out = append(out, string(cjk[i:i+2]))
			}
		}
		cjk = cjk[:0]
	}

	for _, r := range text {
		switch {
		case isCJK(r):
			flushASCII()
			cjk = append(cjk, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.':
			flushCJK()
			ascii = append(ascii, r)
		default:
			flushASCII()
			flushCJK()
		}
	}
	flushASCII()
	flushCJK()
	return out
}

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana)
}

// freq 统计词频。
func freq(tokens []string) map[string]int {
	out := make(map[string]int, len(tokens))
	for _, t := range tokens {
		out[t]++
	}
	return out
}

// Sentences 按中文标点、换行和英文句号切句，保留标点。
func Sentences(text string) []string {
	var out []string
	var cur []rune

	for _, r := range text {
		cur = append(cur, r)
		switch r {
		case '。', '！', '？', '；', '\n', '.', ';', '!', '?':
			out = append(out, strings.TrimSpace(string(cur)))
			cur = cur[:0]
		}
	}
	if last := strings.TrimSpace(string(cur)); last != "" {
		out = append(out, last)
	}

	var merged []string
	for _, s := range out {
		if s != "" {
			merged = append(merged, s)
		}
	}
	return merged
}

// SplitChunks 把长文切成不超过 size 个字符的块。
//
// 优先在句子边界断开；相邻块之间保留 overlap 个字符的重叠，
// 免得答案正好跨在两块之间时被切断。
func SplitChunks(text string, size, overlap int) []string {
	if size <= 0 {
		size = 200
	}
	if overlap < 0 || overlap >= size {
		overlap = size / 8
	}

	var chunks []string
	var cur []rune
	tail := func(r []rune, n int) []rune {
		if n <= 0 || len(r) <= n {
			return append([]rune(nil), r...)
		}
		return append([]rune(nil), r[len(r)-n:]...)
	}

	for _, sentence := range Sentences(text) {
		rs := []rune(sentence)
		// 单句超长时硬切，保证块大小有上界。
		for len(rs) > size {
			if len(cur) > 0 {
				chunks = append(chunks, string(cur))
				cur = tail(cur, overlap)
			}
			chunks = append(chunks, string(rs[:size]))
			cur = append(cur, tail(rs[:size], overlap)...)
			rs = rs[size:]
		}

		if utf8.RuneCountInString(string(cur))+len(rs) > size && len(cur) > 0 {
			chunks = append(chunks, string(cur))
			cur = tail(cur, overlap)
		}
		cur = append(cur, rs...)
	}
	if len(strings.TrimSpace(string(cur))) > 0 {
		chunks = append(chunks, strings.TrimSpace(string(cur)))
	}
	return chunks
}
