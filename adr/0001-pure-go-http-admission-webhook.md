# ADR-0001: 標準 net/http と公式 API 型による軽量 Admission Webhook の採用

- **ステータス**: 承認 (Accepted)
- **日付**: 2026-10-28

---

## 1. コンテキストと課題

Kubernetes の Admission Webhook を実装する際、フレームワークとして `sigs.k8s.io/controller-runtime` を採用することが一般的です。しかし以下の課題があります。

1. **過大な依存関係とバイナリサイズ**:
   - `controller-runtime` はクライアントキャッシュ、マネージャー、インフォーマー、Leader Election など完全なオペレータ向け機能を内包しており、Webhook 単機能としては過剰。
   - コンパイル時間や最終バイナリサイズが肥大化しやすい。
2. **学習の透明性（ブラックボックス化の回避）**:
   - Webhook の実体はシンプルな HTTPS サーバーであり、API Server から `AdmissionReview` JSON を受け取り、JSON Patch を含む `AdmissionReview` を返すだけである。
   - 内部構造やハンドシェイク、RFC 6902 パッチ生成の原理を手触り感を持って理解するためには、余計なラッパーを排除することが望ましい。

---

## 2. 決定事項

本プロジェクトでは、`controller-runtime` を使用せず、**Go 標準ライブラリ (`net/http`) および公式 API 型定義 (`k8s.io/api/admission/v1`, `k8s.io/api/core/v1`, `k8s.io/apimachinery`) のみを用いて Webhook を実装** します。

### アーキテクチャ構成
1. **HTTP ルーティング**: Go 標準の `net/http` (Go 1.22+ のメソッドルーティング対応 `http.NewServeMux`)。
2. **型定義**: `k8s.io/api/admission/v1` の `AdmissionReview`, `AdmissionRequest`, `AdmissionResponse`。
3. **リソースパース**: `k8s.io/apimachinery/pkg/api/resource.Quantity` による Kubernetes メモリ表記（`500Mi`, `1Gi` 等）のバイト変換。
4. **証明書管理**: 自己署名 CA およびサーバー証明書を動的生成または Secret からマウント。

---

## 3. メリットとトレードオフ

### メリット
- **極小バイナリと高速起動**: 不要な依存関係を排除し、数MB〜十数MB程度の静的バイナリで数十ミリ秒以内に起動可能。
- **デバッグ容易性**: 標準の `httptest.NewServer` や `httptest.NewRecorder` を用いて、モックなしで完全な単体テスト・結合テストをローカル完結で実行できる。
- **Kubernetes 内部プロトコルの直接理解**: API Server が送ってくる JSON やパッチ形式がコード上に明示されるため、学習効果が極めて高い。

### トレードオフ
- `controller-runtime` が提供する一部のユーティリティ（証明書の自動更新など）を必要に応じて自前で管理する必要がある（本PoCでは Secret または Go コード内の起動時自己署名生成で対応）。

---

## 4. 信頼できる公式一次ソース

- [Kubernetes API Reference - AdmissionReview v1](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#admissionreview-v1-admission-k8s-io)
- [RFC 6902 - JavaScript Object Notation (JSON) Patch](https://datatracker.ietf.org/doc/html/rfc6902)
- [Go net/http Package Documentation](https://pkg.go.dev/net/http)
