# LESSONS

## 2026-03-20: 初期実装

### パーサーの正規表現設計

- **セクションヘッダーの多様性**: `Commands:`, `Available Commands:`, `Basic Commands (Beginner):`, `The commands are:` など形式が多い。カッコを含むヘッダー (`[\w)]` で終端) や "The X are:" 形式も考慮が必要
- **タブ vs スペース**: `go --help` はタブ区切り、他はスペース区切り。`\s+` で統一的に扱う。ただし先頭インデントの `\s{2,}` はタブ1文字にマッチしないため `\s+` に緩和した

### Bubble Tea のレシーバ型

- `Init()`, `Update()`, `View()` は value receiver が必要（Bubble Tea の interface 要件）
- 内部ヘルパーは pointer receiver を使ってよい
- 混在は意図的だが、コメントで明記すること

### append のスライス変異

- `append(args, "--help")` は元スライスに余剰容量があると破壊的。`make` + `copy` で防御する
