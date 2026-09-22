#!/usr/bin/env bash
# haomi.sh — 豪密 · 互联网版
# 现代一次一密工具：PBKDF2 + AES-256-CTR + HMAC-SHA256
#
# 用法:
#   ./haomi.sh encrypt <明文文件> <密文文件> <口令>
#   ./haomi.sh decrypt <密文文件> <明文文件> <口令>
#   ./haomi.sh demo
#
# 密文格式 (Base64):
#   salt(16) || nonce(16) || ciphertext || mac(32)

set -euo pipefail

ITER=200000

# ---------- 依赖检查 ----------
check_deps() {
  for cmd in openssl xxd base64 awk dd mktemp head cat cut tr wc cmp; do
    command -v "$cmd" >/dev/null 2>&1 || {
      echo "错误: 缺少依赖 '$cmd'" >&2; exit 1
    }
  done
}

# ---------- 工具函数 ----------

# 同时支持参数与 stdin
hex2bin() {
  if [[ $# -gt 0 ]]; then
    printf '%s' "$1" | xxd -r -p
  else
    xxd -r -p
  fi
}
bin2hex() { xxd -p -c 256 | tr -d '\n'; }

# 临时目录池：每个函数拿独立的 work 目录，退出时统一清理
TMP_DIRS=()
cleanup() {
  local d
  for d in "${TMP_DIRS[@]:-}"; do
    [[ -n "$d" && -d "$d" ]] && rm -rf "$d"
  done
}
trap cleanup EXIT

mktmp() {
  local d
  d=$(mktemp -d)
  TMP_DIRS+=("$d")
  printf '%s' "$d"
}

# ---------- PBKDF2 派生 64 字节主密钥 ----------
# 思路:
#   1. openssl enc -pbkdf2 -P 派生 (key 32B, iv 16B) = 48 字节
#   2. SHA-256(key || iv) = 32 字节
#   3. 主密钥 = (key || iv || hash) 的前 64 字节
# 对应豪密：把口令扩展成一张足够长的乱数表种子。
derive_master() {
  local pass="$1" salt_hex="$2"
  local line key iv hash

  line=$(openssl enc -aes-256-ctr -pbkdf2 -iter "$ITER" \
           -salt -S "$salt_hex" -P -pass "pass:$pass" 2>/dev/null) || {
    echo "错误: PBKDF2 不可用，请安装 OpenSSL 1.1.0+" >&2
    return 1
  }

  key=$(printf '%s\n' "$line" | awk -F= '/^key=/{print tolower($2)}')
  iv=$(printf '%s\n'  "$line" | awk -F= '/^iv=/{print tolower($2)}')

  hash=$(printf '%s%s' "$key" "$iv" | hex2bin \
         | openssl dgst -sha256 -binary | bin2hex)

  printf '%s' "${key}${iv}${hash}" | cut -c1-128
}

# ---------- 加密 ----------
do_encrypt() {
  local in="$1" out="$2" pass="$3"
  [[ -f "$in" ]] || { echo "错误: 输入文件不存在: $in" >&2; return 1; }

  local work
  work=$(mktmp)

  # 1. 一次性随机 salt 与 nonce
  local salt_hex nonce_hex
  salt_hex=$(openssl rand -hex 16)
  nonce_hex=$(openssl rand -hex 16)

  # 2. 派生密钥（对应“乱数表种子”）
  local master
  master=$(derive_master "$pass" "$salt_hex") || return 1
  local enc_key="${master:0:64}"    # 前 32 字节
  local mac_key="${master:64:64}"   # 后 32 字节

  # 3. 生成密钥流并异或（对应“乱数表叠加底本”）
  #    AES-256-CTR 本质是“密钥流 ⊕ 明文”，与豪密的模十加法同构。
  openssl enc -aes-256-ctr -K "$enc_key" -iv "$nonce_hex" \
    -in "$in" -out "$work/cipher.bin" 2>/dev/null

  # 4. HMAC-SHA256 完整性标签（豪密没有、现代必需）
  {
    hex2bin "$salt_hex"
    hex2bin "$nonce_hex"
    cat "$work/cipher.bin"
  } | openssl dgst -sha256 -mac HMAC \
        -macopt "hexkey:$mac_key" -binary > "$work/mac.bin"

  # 5. 打包为 Base64
  {
    hex2bin "$salt_hex"
    hex2bin "$nonce_hex"
    cat "$work/cipher.bin"
    cat "$work/mac.bin"
  } | openssl base64 -A > "$out"

  local in_size out_size
  in_size=$(wc -c  < "$in"  | tr -d ' ')
  out_size=$(wc -c < "$out" | tr -d ' ')

  echo "✓ 加密完成"
  echo "  明文: $in ($in_size 字节)"
  echo "  密文: $out ($out_size 字符 Base64)"
  echo "  salt : $salt_hex"
  echo "  nonce: $nonce_hex"
}

# ---------- 解密 ----------
do_decrypt() {
  local in="$1" out="$2" pass="$3"
  [[ -f "$in" ]] || { echo "错误: 输入文件不存在: $in" >&2; return 1; }

  local work
  work=$(mktmp)

  # 1. Base64 解码
  openssl base64 -d -A -in "$in" -out "$work/payload.bin"

  local total
  total=$(wc -c < "$work/payload.bin" | tr -d ' ')
  if (( total < 16 + 16 + 32 )); then
    echo "错误: 密文长度不足（至少 64 字节）" >&2; return 1
  fi

  local cipher_len=$(( total - 16 - 16 - 32 ))

  # 2. 拆出 salt / nonce / cipher / mac
  dd if="$work/payload.bin" of="$work/salt.bin"   bs=1 count=16 2>/dev/null
  dd if="$work/payload.bin" of="$work/nonce.bin"  bs=1 skip=16 count=16 2>/dev/null
  dd if="$work/payload.bin" of="$work/cipher.bin" bs=1 skip=32 count="$cipher_len" 2>/dev/null
  dd if="$work/payload.bin" of="$work/mac.bin"    bs=1 skip=$(( 32 + cipher_len )) count=32 2>/dev/null

  local salt_hex nonce_hex
  salt_hex=$(bin2hex < "$work/salt.bin")
  nonce_hex=$(bin2hex < "$work/nonce.bin")

  # 3. 用相同口令派生相同密钥
  local master
  master=$(derive_master "$pass" "$salt_hex") || return 1
  local enc_key="${master:0:64}"
  local mac_key="${master:64:64}"

  # 4. 先验 MAC，再解密（关键安全步骤）
  local expected actual
  expected=$(cat "$work/salt.bin" "$work/nonce.bin" "$work/cipher.bin" \
    | openssl dgst -sha256 -mac HMAC -macopt "hexkey:$mac_key" -binary | bin2hex)
  actual=$(bin2hex < "$work/mac.bin")

  if [[ "$expected" != "$actual" ]]; then
    echo "✗ 认证失败：口令错误或数据被篡改" >&2
    return 1
  fi

  # 5. 还原明文
  openssl enc -d -aes-256-ctr -K "$enc_key" -iv "$nonce_hex" \
    -in "$work/cipher.bin" -out "$out" 2>/dev/null

  echo "✓ 解密完成: $in → $out ($(wc -c < "$out" | tr -d ' ') 字节)"
}

# ---------- 自检 ----------
do_demo() {
  echo "=== 豪密 · 互联网版 自检 ==="

  local work
  work=$(mktmp)

  local msg="周恩来同志设计的豪密，是密码学史上一次一密的典范。"
  printf '%s' "$msg" > "$work/plain.txt"
  echo "明文: $msg"
  echo "大小: $(wc -c < "$work/plain.txt" | tr -d ' ') 字节"
  echo

  echo "--- 1. 加密 ---"
  do_encrypt "$work/plain.txt" "$work/cipher.txt" "MySecretPass!2024"
  echo
  echo "密文 (Base64 前 96 字符):"
  head -c 96 "$work/cipher.txt"; echo "…"
  echo

  echo "--- 2. 用正确口令解密 ---"
  do_decrypt "$work/cipher.txt" "$work/restored.txt" "MySecretPass!2024"
  echo

  if cmp -s "$work/plain.txt" "$work/restored.txt"; then
    echo "✓ 明文一致性校验通过"
  else
    echo "✗ 明文不一致" >&2
    return 1
  fi
  echo

  echo "--- 3. 用错误口令解密（应失败） ---"
  if do_decrypt "$work/cipher.txt" "$work/wrong.txt" "WrongPassword" 2>/dev/null; then
    echo "✗ 错误口令竟然解密成功了" >&2
    return 1
  else
    echo "✓ 错误口令被正确拒绝"
  fi
}

# ---------- 用法 ----------
usage() {
  cat <<'EOF'
豪密 · 互联网版

用法:
  haomi.sh encrypt <明文文件> <密文文件> <口令>
  haomi.sh decrypt <密文文件> <明文文件> <口令>
  haomi.sh demo

原理对应:
  底本编码 (UTF-8)     → 明文按字节读入
  乱数表   (密钥流)    → AES-256-CTR 密钥流
  模十加法 (异或)      → 密文 = 明文 XOR 密钥流
  真随机乱数          → 每次生成新 salt(16B) + nonce(16B)
  密钥扩展            → PBKDF2-SHA256, 200000 次迭代
  一次性              → salt/nonce 随密文走，密钥不重用
  完整性              → HMAC-SHA256(salt || nonce || ciphertext)

密文格式 (Base64 编码):
  salt(16) || nonce(16) || ciphertext || mac(32)
EOF
}

# ---------- 入口 ----------
main() {
  check_deps
  local cmd="${1:-}"
  case "$cmd" in
    encrypt)
      [[ $# -eq 4 ]] || { usage; exit 1; }
      do_encrypt "$2" "$3" "$4"
      ;;
    decrypt)
      [[ $# -eq 4 ]] || { usage; exit 1; }
      do_decrypt "$2" "$3" "$4"
      ;;
    demo)
      do_demo
      ;;
    -h|--help|help|"")
      usage
      ;;
    *)
      echo "未知命令: $cmd" >&2
      usage >&2
      exit 1
      ;;
  esac
}

main "$@"
