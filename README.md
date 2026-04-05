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

## インストール
```bash
go get github.com/Luke256/suzume
```

## コマンドの定義
コマンドは `suzume.NewCommand` 関数を使用して定義する方法と、 `suzume.UseCommand` 関数を使用して構造体から定義する方法の2通りがあります。

### `suzume.NewCommand` を使用したコマンド定義
`suzume.NewCommand` を用いると、コマンド名と説明、そしてコマンドの実装を一度に定義できます。コマンドの引数は関数のシグネチャから自動的に推測されます。

```go
cmd, err := suzume.NewCommand("greet", "Greet someone", func(name string, num int) error {
    println("Hello,", name, "you have", num, "messages.")
    return nil
})
```

コマンドは `cmd.Run()` を呼び出すことで実行できます。また、`cmd.Run("Luke", "5")`のように引数を直接渡すことも可能です。

### `suzume.UseCommand` を使用したコマンド定義
`suzume.UseCommand` を使用すると、より詳細なコマンド定義が可能になります。

まず、`suzume.Runner` を実装する構造体を定義します。この構造体のフィールドはコマンドの引数やオプションを表し、タグを使用してコマンドライン引数やオプションの情報を指定します。

```go
type GreetCommand struct {
    Name string `cli:"0" usage:"Name of the person to greet"`
    Num  int    `cli:"num" short:"n" usage:"Number of messages"`
}

func (c GreetCommand) Run() error {
    println("Hello,", c.Name, "you have", c.Num, "messages.")
    return nil
}
```

Suzumeで使用される構造体タグは次の通りです：

- `cli:"0"` : 引数の位置を指定します。整数は引数の位置を表し、0は最初の引数を意味します。非整数値はオプションを表します。
- `short:"n"` : オプションの短い形式を指定します。この場合、`-n` で `Num` フィールドを指定できます。
- `usage:"..."` : 引数やオプションの説明を指定します。

次に、`suzume.UseCommand` を使用してコマンドを定義します。

```go
cmd, err := suzume.UseCommand[MyRunner]("notify", "Say goodbye")
```

引数に対してデフォルト値を明示的に指定する場合、`suzume.Defaulter` を実装することで可能になります。

```go
func (r MyRunner) Default() suzume.Defaulter {
	return MyRunner{
		Num: 5,
	}
}
```

> [!Important]
> `Run()` 及び `Default()` メソッドは、構造体の**値**レシーバーで定義してください。

## サブコマンドの定義
サブコマンドを作成するには、`suzume.NewApplication` を使用してアプリケーションを作成し、`AddCommand` メソッドでコマンドを追加します。

```go
cmd1, _ := suzume.NewCommand("foo", "bar", func() error {
    // do something
    return nil
})

cmd2, _ := suzume.NewCommand("hoge", "fuga", func() error {
    // do something
    return nil
})

app := suzume.NewApplication("myapp", "My CLI Application")
app.AddCommand(cmd1) // myapp foo
app.AddCommand(cmd2) // myapp hoge
app.Run()
```

> [!Important]
> アプリケーションは0個以上のコマンドと0個以上のサブアプリケーションを持つことができますが、**アプリケーション自体はコマンドとして実行できません**。これは、アプリケーションをサブコマンドのハブとして設計するための意図的な制約です。もしアプリケーション自体が実行能力を持ってしまうと、`myapp subcmd` のようなコマンドの `subcmd` 部分が引数であるのか、サブコマンドであるのかの区別がつかなくなってしまいます。

## Config
ログの出力先などの設定は、`SetConfig` メソッドを使用して行います。

```go
cmd.SetConfig(suzume.Config{
    Log:        os.Stdout,
    ErrorLog:   os.Stderr,
})
```
デフォルトでは、ログは標準出力に、エラーログは標準エラー出力に出力されます。
また `SetConfig` で明示的に設定されない限り、コマンドやアプリケーションは親の設定を継承します。

## ヘルプの自動生成
Suzume はコマンドの説明や引数、オプションの情報から自動的にヘルプメッセージを生成します。ユーザーは `--help` `-h` オプションを使用してヘルプを表示できます。また、アプリケーションに対しては `help` サブコマンドも自動的に追加されます。

(これはどのサブコマンドよりも優先されるため、`help` という名前のサブコマンドを定義することはできません)

例:

```
$ go run main.go --help
mycli

A simple CLI application

Usage:
  mycli [command] [args...]

Commands:
  greet                Greet someone
  notify               Say goodbye
  help                 Show this help message
```

## ライセンス
Suzume は MIT ライセンスのもとで公開されています。
