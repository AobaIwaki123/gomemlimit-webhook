# GOMEMLIMIT 自動注入 Mutating Webhook の設計と仕様

このノートでは、Go 1.19 以降のソフトメモリリミット機能（`GOMEMLIMIT`）を Kubernetes の Pod に対して動的に計算・注入する Mutating Admission Webhook の設計、背景、および実装仕様を整理します。

---

## 1. 背景と課題

### 1.1 Kubernetes における Go アプリケーションの OOMKilled 問題
Go のガベージコレクション（GC）は、デフォルトで前回の GC 完了時ヒープサイズを基準に次の GC トリガーを決定します（`GOGC=100` の場合、ヒープが2倍に達した時点で実行）。

Kubernetes 環境では、Pod に対して `resources.limits.memory` が設定されます（cgroups によるメモリ上限）。しかし、Go ランタイムは cgroups のメモリ制限を自動的には感知しないため、一時的なトラフィック増やヒープ伸張によってコンテナの上限値を超過し、Linux カーネルの OOM Killer によって強制終了（`SIGKILL`, Exit Code 137）される事象が頻発していました。

### 1.2 GOMEMLIMIT による解決と新たな運用課題
Go 1.19 において、ランタイムが目標とするメモリ上限を指定できる `GOMEMLIMIT` 環境変数および `runtime/debug.SetMemoryLimit` が導入されました。
メモリ制限に近づくと、GC は `GOGC` の目標値にかかわらず高頻度で実行され、上限突破を防ぎます。

- **推奨設定値**: コンテナのメモリ上限（cgroups limit）の **85%〜90%**
  - 残りの 10%〜15% は、非ヒープメモリ（スタック、Goroutine、Go ランタイムのメタデータ、Cgo、カーネルのページキャッシュ等）のために確保する必要があります。
- **運用の課題**:
  - 各 Deployment や StatefulSet の YAML に個別に `GOMEMLIMIT` を記述すると、`resources.limits.memory` を変更した際に書き換え忘れが発生しやすい。
  - 単位の変換（`500Mi` -> `450MiB` / バイト数計算）を開発者が都度手動で行うのはヒューマンエラーの原因になる。

---

## 2. Mutating Admission Webhook のアーキテクチャ

Kubernetes の Mutating Admission Webhook は、API Server がリソースを etcd に永続化する直前に HTTP(S) 経由でリクエストをインターセプトし、オブジェクトの定義を動的に変更（Patch）できる拡張メカニズムです。

### 2.1 処理の流れ
1. **Pod 作成リクエスト**: 開発者またはコントローラ（Deployment 等）が Pod 作成 API を発行。
2. **Admission 判定**: API Server が `MutatingWebhookConfiguration` に従い、対象 Pod の `AdmissionReview` (v1) を本 Webhook に POST 送信。
3. **対象判定**:
   - アノテーション `gomemlimit.aooba.net/inject: "true"` の有無（または Namespace のオプトイン設定）。
   - すでに `GOMEMLIMIT` が環境変数に明示されているコンテナは上書きをスキップ。
4. **メモリリミット計算**:
   - コンテナ定義の `resources.limits.memory`（未設定時は `resources.requests.memory`）を取得。
   - 設定比率（デフォルト: 90%、アノテーションで `gomemlimit.aooba.net/ratio: "0.85"` のように上書き可能）を乗算。
   - 単位をバイト数値または `MiB` 表記でフォーマット。
5. **JSON Patch 生成**:
   - RFC 6902 形式の JSON Patch（例: `[{"op": "add", "path": "/spec/containers/0/env", "value": [...]}]`）を生成。
   - `AdmissionResponse` に Base64 エンコードして API Server へ返却。
6. **Pod 作成実行**: API Server がパッチを適用した状態で Pod をクラスタへスケジュール。

---

## 3. Webhook の仕様定義

### 3.1 アノテーション仕様

| アノテーション | 型 / デフォルト | 説明 |
| :--- | :--- | :--- |
| `gomemlimit.aooba.net/inject` | `string` (`"true"` / `"false"`) | Pod または Namespace に対して自動注入を有効化するかどうか |
| `gomemlimit.aooba.net/ratio` | `string` (`"0.90"`) | メモリ上限に対して割り当てる比率（0.50〜0.95） |
| `gomemlimit.aooba.net/gogc` | `string` (未指定) | 併せて設定したい `GOGC` の値（例: `"100"` や `"off"`） |

### 3.2 注入例

**注入前の Pod spec（抜粋）**:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: api-server
  annotations:
    gomemlimit.aooba.net/inject: "true"
spec:
  containers:
    - name: app
      image: my-go-app:v1.0.0
      resources:
        limits:
          memory: "500Mi"
```

**注入後の Pod spec（抜粋）**:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: api-server
  annotations:
    gomemlimit.aooba.net/inject: "true"
spec:
  containers:
    - name: app
      image: my-go-app:v1.0.0
      env:
        - name: GOMEMLIMIT
          value: "450MiB"
      resources:
        limits:
          memory: "500Mi"
```

---

## 4. 信頼できる公式一次ソース

- [Go GC Guide (Go Official Documentation)](https://go.dev/doc/gc-guide) - `GOMEMLIMIT` のアルゴリズム、推奨マージン設定、OOM 回避の原理
- [Go 1.19 Release Notes - Memory limit](https://go.dev/doc/go1.19#runtime) - `GOMEMLIMIT` および `debug.SetMemoryLimit` の追加仕様
- [Dynamic Admission Control (Kubernetes Official Documentation)](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) - MutatingWebhookConfiguration と AdmissionReview v1 仕様
- [Managing Resources for Containers (Kubernetes Official Documentation)](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/) - `limits.memory` と cgroups メモリ制限の動作仕様
