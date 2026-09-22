#!/usr/bin/env bash
# emoji-steg.sh — 基于 Unicode 变体选择符的文本隐写工具
# 用法:
#   ./emoji-steg.sh hide   <输入文件> <输出文件> [载体表情]
#   ./emoji-steg.sh extract <输入文件> <输出文件>
#   ./emoji-steg.sh demo
set -euo pipefail

# ---------- 工具函数 ----------

# 单字节 -> 变体选择符的 UTF-8 字节序列（直接输出字节）
byte_to_vs() {
  local b=$1
  if (( b < 16 )); then
    # U+FE00 + b : UTF-8 = EF B8 (80+b)
    printf '%b' "\\xEF\\xB8\\x$(printf '%02X' $(( 0x80 + b )))"
  else
    # U+E0100 + (b - 0x10) : UTF-8 = F3 A0 (84+((b-0x10)>>6)) (80+((b-0x10)&0x3F))
    local offset=$(( b - 0x10 ))
    local b3=$(( 0x84 + (offset >> 6) ))
    local b4=$(( 0x80 + (offset & 0x3F) ))
    printf '%b' "\\xF3\\xA0\\x$(printf '%02X' "$b3")\\x$(printf '%02X' "$b4")"
  fi
}

# 文件 -> 变体选择符序列（写入 stdout）
encode_file() {
  local infile=$1
  od -An -v -tu1 "$infile" | tr -s ' ' '\n' | grep -v '^$' | while read -r byte; do
    byte_to_vs "$byte"
  done
}

# ---------- 子命令 ----------

cmd_hide() {
  local infile=$1 outfile=$2 carrier=${3:-"😀"}
  [[ -f "$infile" ]] || { echo "错误: 输入文件不存在: $infile" >&2; exit 1; }

  # 写入载体字符
  printf '%s' "$carrier" > "$outfile"
  # 追加编码后的变体选择符序列
  encode_file "$infile" >> "$outfile"

  echo "已隐藏: $infile -> $outfile (载体: $carrier)"
}

cmd_extract() {
  local infile=$1 outfile=$2
  [[ -f "$infile" ]] || { echo "错误: 输入文件不存在: $infile" >&2; exit 1; }

  # 将文件转为十进制字节数组
  local -a bytes
  while IFS= read -r b; do
    bytes+=("$b")
  done < <(od -An -v -tu1 "$infile" | tr -s ' ' '\n' | grep -v '^$')

  local n=${#bytes[@]}
  local i=0
  : > "$outfile"  # 清空输出文件

  while (( i < n )); do
    local b1=${bytes[i]}
    # 检查 EF B8 80-8F
    if (( b1 == 0xEF && i+2 < n && bytes[i+1] == 0xB8 )); then
      local b3=${bytes[i+2]}
      if (( b3 >= 0x80 && b3 <= 0x8F )); then
        local orig=$(( b3 - 0x80 ))
        printf '%b' "\\x$(printf '%02X' "$orig")" >> "$outfile"
        i=$(( i + 3 ))
        continue
      fi
    fi
    # 检查 F3 A0 84-87 80-BF
    if (( b1 == 0xF3 && i+3 < n && bytes[i+1] == 0xA0 )); then
      local b3=${bytes[i+2]}
      local b4=${bytes[i+3]}
      if (( b3 >= 0x84 && b3 <= 0x87 && b4 >= 0x80 && b4 <= 0xBF )); then
        local offset=$(( ((b3 - 0x84) << 6) + (b4 - 0x80) ))
        local orig=$(( offset + 0x10 ))
        printf '%b' "\\x$(printf '%02X' "$orig")" >> "$outfile"
        i=$(( i + 4 ))
        continue
      fi
    fi
    # 不是变体选择符，跳过
    i=$(( i + 1 ))
  done

  echo "已还原: $infile -> $outfile"
}

cmd_demo() {
  echo "=== 自检演示 ==="
  local tmpdir
  tmpdir=$(mktemp -d)
  trap "rm -rf '$tmpdir'" EXIT

  # 构造测试数据
  printf 'Hello, 世界!' > "$tmpdir/secret.txt"
  echo "原始数据:"
  xxd "$tmpdir/secret.txt"

  # 隐藏
  "$0" hide "$tmpdir/secret.txt" "$tmpdir/stego.txt" "🔐"
  echo -e "\n伪装后的文件内容（视觉上只有表情）:"
  cat "$tmpdir/stego.txt"
  echo -e "\n\n伪装后的文件字节大小: $(wc -c < "$tmpdir/stego.txt") bytes"

  # 还原
  "$0" extract "$tmpdir/stego.txt" "$tmpdir/recovered.txt"
  echo -e "\n还原后的数据:"
  xxd "$tmpdir/recovered.txt"

  # 验证
  if cmp -s "$tmpdir/secret.txt" "$tmpdir/recovered.txt"; then
    echo -e "\n✅ 自检通过：原始数据与还原数据完全一致。"
  else
    echo -e "\n❌ 自检失败：数据不一致！" >&2
    exit 1
  fi
}

# ---------- 入口 ----------

main() {
  local cmd=${1:-}
  case "$cmd" in
    hide)
      [[ $# -ge 3 ]] || { echo "用法: $0 hide <输入文件> <输出文件> [载体表情]" >&2; exit 1; }
      cmd_hide "$2" "$3" "${4:-😀}"
      ;;
    extract)
      [[ $# -ge 3 ]] || { echo "用法: $0 extract <输入文件> <输出文件>" >&2; exit 1; }
      cmd_extract "$2" "$3"
      ;;
    demo)
      cmd_demo
      ;;
    *)
      echo "用法: $0 {hide|extract|demo} ..." >&2
      exit 1
      ;;
  esac
}

main "$@"
