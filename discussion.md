# Go GOMEMLIMIT / GOGC Auto-Tuning Mutating Admission Webhook ディスカッション & 進捗管理 (discussion.md)

このドキュメントでは、Go GOMEMLIMIT / GOGC Auto-Tuning Mutating Admission Webhook に関する目的、ロードマップ、議論の経緯、進捗を記録・管理します。

---

## 目的・ゴール

- Kubernetes 環境において、Go 製コンテナが Linux カーネルの OOM Killer（Exit Code 137）によって不意に強制終了されるのを防ぐ。
- Pod の `resources.limits.memory`（または requests）から適切な `GOMEMLIMIT`（デフォルト 90%）および `GOGC` を動的に算出し、RFC 6902 JSON Patch で自動注入する軽量 Mutating Admission Webhook を提供する。
- 巨大な外部フレームワーク（`controller-runtime` 等）をあえて使わず、標準 `net/http` と公式 API 型のみで実装することで、Kubernetes 拡張の内部構造（AdmissionReview プロトコル、TLS 要件、JSON Patch）を深く理解する。

---

## ロードマップ / タスク

| Step | 項目 | 内容 | 状況 |
| :--- | :--- | :--- | :--- |
| **Step 1** | **調査・仕様策定** | Go 1.19+ の GOMEMLIMIT 原理、Webhook プロトコル、アノテーション仕様の策定 | **完了** |
| **Step 2** | **設計・方針決定 (ADR)** | ADR-0001: 標準 `net/http` 採用、バイナリ軽量化・学習透明性方針 | **完了** |
| **Step 3** | **コアロジック実装** | `pkg/mutator`（Pod/Container 判定、比率計算、JSON Patch 生成、単体テスト） | **完了** |
| **Step 4** | **TLS / 証明書モジュール** | `pkg/cert`（インメモリ自己署名 CA / サーバー証明書生成、SAN 対応） | **完了** |
| **Step 5** | **Webhook サーバー実装** | `cmd/webhook`（`/mutate`, `/healthz`、slog ログ、統合テスト通過） | **完了** |
| **Step 6** | **コンテナ化 & マニフェスト** | Dockerfile（Distroless 最小化）、k8s マニフェスト、ローカルデプロイスクリプト | **完了** |
| **Step 7** | **クラスタ実機検証** | 自宅 K8s クラスタへのデプロイとサンプルのテスト Pod による動作確認 | **準備完了** |

---

## 決定事項 (ADR一覧)

- [ADR-0001: 標準 net/http と公式 API 型による軽量 Admission Webhook の採用](adr/0001-pure-go-http-admission-webhook.md)

---

## 関連ドキュメント

- [note/01_architecture_and_gomemlimit_spec.md](note/01_architecture_and_gomemlimit_spec.md) - GOMEMLIMIT 自動注入 Mutating Webhook の設計と仕様
