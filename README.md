# Suzume CLI Framework

Suzume は Go 言語でコマンドラインアプリケーションを構築するためのフレームワークです。

## Suzume の特徴
Suzume は一つのコマンドを定義するシンプルなプロジェクトからサブコマンドを持つ複雑なプロジェクトまで、規模を問わないコマンドアプリケーションの構築を容易にサポートします。

例として、簡単な通知を行うコマンドを定義するコードは以下のようになります。

```go
package main

import (
    "fmt"
    "github.com/Luke256/suzume"
)

func main() {
    cmd, err := suzume.NewCommand("notify", "Greet and notify tasks", func(name string, tasks int) error {
        fmt.Printf("Hello, %s! You have %d tasks to complete today.\n", name, tasks)
        return nil
    })
    if err != nil {
        panic(err)
    }

    cmd.Run() // go run main.go Alice 5
}
```

非常にシンプルながら、Cobra や urfave/cli といった他の CLI フレームワークと比較して少ないコードで同様の機能を実現できます。
Suzume は以下の特徴により、Go 言語での CLI アプリケーション開発をよりシンプルかつ効率的にします。

- **スケール性** : 軽量なアプリケーションはシンプルなコードで構築でき、必要に応じてサブコマンドや追加の機能を簡単に追加できます
- **シンプルなコマンド定義** : コマンドの引数やオプションを関数のシグネチャや構造体のタグから推測するため、コードが非常にクリーンになります
- **独自型のサポート** : `TextUnmarshaler` を実装する独自の型をコマンド引数やオプションとして使用できます
- **明示的なデフォルト値** : コマンド引数やオプションのデフォルト値を明示的に指定できるため、コードの可読性が向上します
- **オプション記法** : `--option=value` 形式のオプション記法をサポートし、ユーザーにとって直感的なコマンドラインインターフェースを提供します
- **ヘルプの自動生成** : コマンドの説明や引数、オプションの情報から自動的にヘルプメッセージを生成します
- **軽量な依存関係** : Suzume は標準ライブラリのみで構築され、プロジェクト全体の依存関係を最小限に抑えます

