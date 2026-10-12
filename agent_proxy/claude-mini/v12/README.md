# v12 — 健壮文件写：原子写 + 陈旧拒绝 + 多编辑原子批（含 v8..v11）

在 v11 之上补 **健壮文件写**。Agent 会覆盖文件、跑格式化、还可能和别的 Agent 并发。
v12 把 jingwei 那两条"值回票价"的写语义搬进 Write/Edit。

## 机制：写语义收口到一个 ledger + atomicWrite

1. **`tools/robust.go` 新增**：
   - **字节台账 `ledger`**：`noteRead` 在整文件读取时记下最后看到的字节；
     `checkStale` 在写前比对磁盘——**见过且变了**就拒绝并提示"先重读"，
     **从没见过**照旧放行（刻意不做 read-before-edit 门禁）。
   - **`atomicWrite`**：同目录 `CreateTemp` + `Sync` + `Chmod` + `Rename`，
     崩溃也不会留下半截文件，读者永不见中间态。
2. **`write.go`**：读现状 → `checkStale` → `atomicWrite` → `commitWrite`。
3. **`edit.go` 重写**：同样先 `checkStale`；支持 `edits[]` **多对一次提交**，
   任一 `old_string` 找不到/不唯一就整批不落盘（all-or-nothing）；
   歧义时**报出命中的行号**；`replace_all` 对全批生效。
4. **`read.go`**：整文件读取登记台账，供后续写校验。

## 与 v11 的 diff

| 文件 | 变化 |
|---|---|
| `tools/robust.go` | 新增：字节台账 + 陈旧校验 + 原子写 |
| `tools/write.go` | 写前陈旧检查、改原子写并登记 |
| `tools/edit.go` | 陈旧检查、`edits[]` 原子批、歧义行号、原子提交 |
| `tools/read.go` | 整文件读登记台账 |
| `tools/robust_test.go` | 新增：陈旧拒绝、原子批、行号、无临时残留 |

## 运行

```bash
env -u GOROOT go build -o v12-agent ./v12
./v12-agent -v "把 main.go 里两处 TODO 都改成 FIXME"
# 若外部编辑器在你上次读后改过该文件，Edit 会拒绝并让你先重读，避免覆盖别人的改动
```

## 测试

```bash
env -u GOROOT go test ./v12/...
```
