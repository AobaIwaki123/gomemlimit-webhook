# gomemlimit-webhook

Kubernetes Pod のメモリ制限（`resources.limits.memory`）に基づき、Go アプリケーション向けに `GOMEMLIMIT` および `GOGC` 環境変数を動的に計算・注入する軽量な Mutating Admission Webhook です。

`controller-runtime` などの巨大フレームワークに依存せず、**Go 標準ライブラリ (`net/http`) と Kubernetes 公式 API 型 (`k8s.io/api`) のみ** で構築されており、極小のフットプリント（静的バイナリ 約 9.8MB）と数十ミリ秒未満の高速コールドスタートを実現しています。

---

## 背景と解決する課題

Kubernetes 環境では、Go のランタイム GC がコンテナの cgroup メモリ制限を自動認識しないため、一時的なヒープ伸張によってコンテナ上限を超過し、Linux カーネルの OOM Killer（`SIGKILL`, Exit Code 137）で強制終了されるリスクが常に存在します。

Go 1.19 以降で導入されたソフトメモリリミット機能（`GOMEMLIMIT`）をコンテナ制限の **85%〜90%** に設定することで、上限到達前に GC を積極的に走らせて OOM 回避が可能です。

自社コードを変更できる場合は `go.uber.org/automemlimit` 等をインポートするのが手軽ですが、外部 OSS やサードパーティ製コンテナなど、アプリケーションコードに手を入れられない環境において、インフラ層から透過的に自動注入する仕組みとして本 Webhook を利用できます。

---

## 主な特徴

- **外部フレームワーク依存ゼロ**: Go 標準ライブラリ (`net/http`) と公式型定義のみで実装。内部プロトコル（AdmissionReview v1, RFC 6902 JSON Patch）のブラックボックス化を排除。
- **極小バイナリ・低リソース消費**: シンボルや DWARF を削ぎ落とした静的バイナリは約 9.8MB。Google Distroless (`nonroot`) イメージで安全に動作。
- **インメモリ TLS 自己署名対応**: 外部の `cert-manager` を導入しなくても、起動時にメモリ上で CA および SAN 対応サーバー証明書を動的生成可能。もちろん Secret マウントによる証明書管理にも対応。
- **RFC 6902 準拠の JSON Patch**: コンテナの `env` フィールドに対して正確なパッチを生成。
- **柔軟なアノテーション設定**: Pod ごとの有効化、比率（デフォルト 90%）、追加の `GOGC` 指定に対応。

---

## アノテーション仕様

| アノテーション | 型 / デフォルト値 | 説明 |
| :--- | :--- | :--- |
| `gomemlimit.aooba.net/inject` | `string` (`"true"` / `"false"`) | Webhook による自動注入を有効化するかどうか |
| `gomemlimit.aooba.net/ratio` | `string` (`"0.90"`) | メモリ制限に対して割り当てる比率（例: `"0.85"`） |
| `gomemlimit.aooba.net/gogc` | `string` (未指定) | 併せて設定したい `GOGC` の目標値（例: `"100"`, `"off"`） |

---

## クイックスタート

### 1. ビルドとテスト

```bash
# レースディテクター付きで単体・結合テストを実行
make test

# 最適化された静的バイナリのビルド (約 9.8MB)
make build-stripped
```

### 2. Kubernetes クラスタへのデプロイ

TLS 証明書（Secret）の生成から Webhook Configuration への CA Bundle 注入までをワンライナーで実行します。

```bash
./scripts/deploy-local.sh
```

### 3. 動作確認

メモリ制限を設定したサンプルのテスト Pod を作成します。

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: sample-go-app
  annotations:
    gomemlimit.aooba.net/inject: "true"
    gomemlimit.aooba.net/ratio: "0.85"
spec:
  containers:
    - name: app
      image: alpine:latest
      command: ["sh", "-c", "env | grep -E 'GOMEMLIMIT|GOGC' && sleep 3600"]
      resources:
        limits:
          memory: "256Mi"
```

注入された環境変数を確認します。

```bash
kubectl get pod sample-go-app -o jsonpath='{.spec.containers[0].env}'
# 実行結果例: [{"name":"GOMEMLIMIT","value":"217MiB"}]
```

---

## 参考文献

- [Go GC ガイド (Go 公式ドキュメント)](https://go.dev/doc/gc-guide)
- [Dynamic Admission Control (Kubernetes 公式ドキュメント)](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)
- [RFC 6902 JavaScript Object Notation (JSON) Patch](https://datatracker.ietf.org/doc/html/rfc6902)

---

## ライセンス

MIT License
