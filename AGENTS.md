# Go GOMEMLIMIT / GOGC Auto-Tuning Mutating Admission Webhook 規約 (AGENTS.md)

このワークスペースは、**Go GOMEMLIMIT / GOGC Auto-Tuning Mutating Admission Webhook** に関する調査・設計・検証・自動化を推進するための協業領域です。

---

## ディレクトリ構成と役割

```
space/gomemlimit-webhook/
├── AGENTS.md             # 本規約ファイル（スコープ・開発ルール）
├── discussion.md         # 議論の方向性、要件定義、進捗管理、アクションアイテム
├── note/                 # 調査・検証・技術メモ（01_xxx.md 形式で蓄積）
├── adr/                  # 確定した方針・アーキテクチャ選定 (0001-xxx.md)
└── scripts/              # 本テーマ専用のスクリプト・ツール・自動化コード
```

---

## 運用ルール

1. **進捗・議論の記録 (`discussion.md`)**
   - 議論の前提、目的、課題、ネクストアクションを常に最新に保ちます。
2. **知見の蓄積 (`note/`)**
   - 調査した仕様や技術検証の結果は `note/` に連番付きマークダウンで記録します。
3. **意思決定の記録 (`adr/`)**
   - 設計方針やアーキテクチャの選定理由は `adr/` に記録します。
4. **再現性の確保 (`scripts/`)**
   - 検証用スクリプトや実行手順は `scripts/` 配下にコード化して残します。
