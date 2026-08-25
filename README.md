# Suzume CLI Framework

**日本語** | [English](README.en.md)

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
- **独自型のサポート** : `TextUnmarshaler` を実装する値型・ポインタ型をコマンド引数やオプションとして使用できます。ポインタ型には変換時に新しい値が割り当てられます
- **明示的なデフォルト値** : オプションのデフォルト値を明示的に指定できるため、コードの可読性が向上します
- **ヘルプの自動生成** : コマンドの説明や引数、オプションの情報から自動的にヘルプメッセージを生成します
- **軽量な依存関係** : Suzume は標準ライブラリのみで構築され、プロジェクト全体の依存関係を最小限に抑えます

## インストール

Suzume を使用するには Go 1.22 以降が必要です。

```bash
go get github.com/Luke256/suzume
```

## コマンドの定義
コマンドは `suzume.NewCommand` 関数を使用して定義する方法と、 `suzume.UseCommand` 関数を使用して構造体から定義する方法の2通りがあります。

### `suzume.NewCommand` を使用したコマンド定義
`suzume.NewCommand` を用いると、コマンド名と説明、そしてコマンドの実装を一度に定義できます。コマンドの引数は関数のシグネチャから自動的に推測されます。

ハンドラーは、`context.Context` と文字列から変換可能な位置引数（文字列、数値、`encoding.TextUnmarshaler` 実装型）を受け取れます。`bool` とスライスは使用できません。戻り値は、なしまたは単一の `error` のみ指定できます。不正なシグネチャや `nil` は `NewCommand` の呼び出し時にエラーになります。

```go
cmd, err := suzume.NewCommand("greet", "Greet someone", func(name string, num int) error {
    println("Hello,", name, "you have", num, "messages.")
    return nil
})
```

コマンドは `cmd.Run()` を呼び出すことで実行できます。また、`cmd.Run("Luke", "5")`のように引数を直接渡すことも可能です。

同じコマンドの `Run` または `RunContext` を繰り返し、あるいは並行して呼び出しても、引数は実行ごとに独立してバインドされます。並行実行時はハンドラーも並行して呼び出されるため、ハンドラーや設定した出力先が共有する状態の同期は呼び出し側で行ってください。

コマンドを `cmd.RunAndExit()` で実行すると、コマンドがエラーを返した場合にプロセスが終了コード1で終了します。

### `suzume.UseCommand` を使用したコマンド定義
`suzume.UseCommand` を使用すると、より詳細なコマンド定義が可能になります。

まず、`suzume.CommandDefinition` を実装する構造体を定義します。`Run()` と `Default()` はどちらもポインタレシーバーで実装します。`suzume.Command` の埋め込みは任意です。ここで定義した構造体のフィールドはコマンドの引数やオプションを表し、タグを使用してコマンドライン引数やオプションの情報を指定します。
(埋め込んだ `suzume.Command` とプライベートなフィールドは、コマンドの引数やオプションとして認識されません)

```go
type GreetCommand struct {
    suzume.Command
    Name string `cli:"0" usage:"Name of the person to greet"`
    Num  int    `cli:"num" short:"n" usage:"Number of messages"`
}

func (c *GreetCommand) Run(ctx context.Context) error {
    println("Hello,", c.Name, "you have", c.Num, "messages.")
    return nil
}
```

Suzumeで使用される構造体タグは次の通りです：

- `cli:"0"` : 位置引数の順序を非負整数で指定します。値の小さい引数から処理され、非整数値は先頭の `--` を含まないオプション名を表します。 `=` は使用できません。
- `short:"n"` : 先頭の `-` を含まない短縮名を指定します。この場合、`-n` で `Num` フィールドを指定できます。
- `usage:"..."` : 引数やオプションの説明を指定します。
- `default:"..."` : ヘルプに表示するデフォルト値の文字列を上書きします。このタグは実行時の値には影響しません。空文字列を指定した場合はデフォルト値を表示しません。

位置引数は指定した非負整数の小さい順に処理されます。フィールド型には文字列、Bool、数値、`encoding.TextUnmarshaler` 実装型を使用できます。スライスはオプションだけで使用でき、その要素も対応型である必要があります。オプション名と短縮名は重複できず、両者の間でも同じ名前は使えません。組み込みヘルプが使用する `help` と `h` も予約されています。未対応のフィールド型や不正なタグは `UseCommand` の呼び出し時にエラーになります。

次に、`suzume.UseCommand` を使用してコマンドを定義します。

```go
cmd, err := suzume.UseCommand[*GreetCommand]("greet", "Greet someone")
```

`suzume.Command` は既定で何もしない `Run()` と、ゼロ値を変更しない `Default()` を提供します。埋め込まない場合は、ポインタレシーバーの `Run()` と `Default()` を両方明示的に実装してください。

オプションにデフォルト値を明示的に指定する場合は、ポインタレシーバーの `Default()` でフィールドを設定します。

```go
func (r *GreetCommand) Default() {
	r.Num = 5
}
```

> [!Important]
> `UseCommand[*GreetCommand]` のように、コマンド定義には構造体のポインタ型を指定してください。

> [!Note]
> オプションとして定義した Bool 型のフィールドはフラグとして処理され、指定すると `true` になります。`--flag=false` の形式で明示的に値を指定した場合は、その値が使用されます。Bool 型のデフォルト値はヘルプには表示されません。
>
> スライス型のオプションは、`--tag stable fast` の分離形式では後続の値を複数受け取り、`--tag=stable` の値付き形式では指定値だけを含む1要素のスライスを受け取ります。
>
> 独自の型の表示やデフォルト値を表示させたくない場合などには `default:"..."` を使用してください。例えば `default:"from environment"` は実値の代わりに指定した文字列を表示し、特に`default:""` はデフォルト値の表示を抑止します。

### サブコマンドの定義
サブコマンドを作成するには、`suzume.NewApp` を使用してアプリケーションを作成し、`AddCommand` メソッドでコマンドを追加します。静的なアプリケーション名には、作成に失敗するとpanicする `suzume.MustNewApp` も使用できます。

```go
cmd1, _ := suzume.NewCommand("foo", "bar", func() error {
    // do something
    return nil
})

cmd2, _ := suzume.NewCommand("hoge", "fuga", func() error {
    // do something
    return nil
})

app := suzume.MustNewApp("myapp", "My CLI Application")
if err := app.AddCommand(cmd1); err != nil { // myapp foo
    panic(err)
}
if err := app.AddCommand(cmd2); err != nil { // myapp hoge
    panic(err)
}
app.Run()
```

`AddCommand` と `AddApp` は、呼び出された時点のコマンドやサブアプリケーションをコピーして登録します。エイリアス、設定、子要素は追加前に構成してください。追加後に元のコマンドやサブアプリケーションの構造を変更しても、登録済みのアプリケーションには反映されません。コマンド、サブアプリケーション、エイリアスの名前を `-` で始めることはできません。

> [!Important]
> アプリケーションは0個以上のコマンドと0個以上のサブアプリケーションを持つことができますが、**アプリケーション自体はコマンドとして実行できません**。これは、アプリケーションをサブコマンドのハブとして設計するための意図的な制約です。もしアプリケーション自体が実行能力を持ってしまうと、`myapp subcmd` のようなコマンドの `subcmd` 部分が引数であるのか、サブコマンドであるのかの区別がつかなくなってしまいます。

### サブコマンドのネスト
サブコマンドはさらにサブコマンドを持つことができます。これにより、複雑なコマンド階層を構築することが可能になります。

```go
root := suzume.MustNewApp("root", "Root Command")
sub1 := suzume.MustNewApp("sub1", "Sub Command 1")
cmd, _ := suzume.NewCommand("cmd", "A command", func() error {
    // do something
    return nil
})
if err := sub1.AddCommand(cmd); err != nil { // root sub1 cmd
    panic(err)
}
if err := root.AddApp(sub1); err != nil {
    panic(err)
}
root.Run() // go run main.go sub1 cmd
```

## Config

ログの出力先やシグナルのハンドリング等の設定は、コマンドごとに設定を行うことができます。`suzume.DefaultConfig()` または `suzume.NewConfig(options ...ConfigOption)` で作成し、コマンドやアプリケーションの `SetConfig` メソッドに渡します。

設定には次の API を使用します。

- `suzume.DefaultConfig()`：既定の設定を作成します。
- `suzume.NewConfig(options ...ConfigOption)`：既定値から開始し、指定されたオプションを適用します。
- `suzume.WithLog(writer)`：通常ログの出力先を設定します。
- `suzume.WithErrorLog(writer)`：エラーログの出力先を設定します。
- `suzume.WithIgnoreSignals(signals...)`：実行中に捕捉するシグナルを設定します。

### 既定値と継承

`DefaultConfig()` とオプションを指定しない `NewConfig()` は、どちらも次の既定値を使用します。

- 通常ログ：`os.Stdout`
- エラーログ：`os.Stderr`
- 捕捉するシグナル：なし

明示的に設定を行っていないコマンドやアプリケーションは、実行時に親の設定を継承します。`SetConfig` で設定を明示すると、そのコマンドまたはアプリケーションでは親の設定を継承せず、指定された設定を使用します。親を持たない場合は既定値が使用されます。

既定の設定を明示する場合は、次のように指定できます。

```go
app.SetConfig(suzume.DefaultConfig())
```

### ログの設定

`WithLog` と `WithErrorLog` で出力先を変更できます。`nil` を渡した場合は対応する既定の出力先が維持されます。出力を意図的に抑止する場合は、`nil` ではなく `io.Discard` を指定してください。

```go
cmd.SetConfig(suzume.NewConfig(
    suzume.WithLog(logWriter),
    suzume.WithErrorLog(io.Discard), // エラーログを抑止
))
```

例えば `suzume.WithLog(nil)` は `os.Stdout` を維持し、`suzume.WithErrorLog(nil)` は `os.Stderr` を維持します。

### シグナルのハンドリング

`WithIgnoreSignals` に指定したシグナルは、コマンドの実行中に捕捉されます。シグナルを受け取ると、プロセスを直ちに終了する代わりにコマンドのコンテキストがキャンセルされるため、`ctx.Done()` を通じて終了処理を実行できます。

```go
cmd.SetConfig(suzume.NewConfig(
    suzume.WithIgnoreSignals(os.Interrupt, syscall.SIGTERM),
))
```

既定では Suzume はシグナルを捕捉せず、通常のプロセスのシグナル処理が維持されます。

## ヘルプの自動生成
Suzume はコマンドの説明や引数、オプションの情報から自動的にヘルプメッセージを生成します。ユーザーは `--help` `-h` オプションを使用してヘルプを表示できます。また、アプリケーションに対しては `help` サブコマンドも自動的に追加されます。

アプリケーション経由でコマンドヘルプを表示すると、Usageには `root child run` のような完全なコマンドパスが表示されます。値を取るオプションは、スカラー型なら `--count <value>`、スライス型なら `--tag <value...>` と表示されます。Bool型のオプションは値を取らないフラグとして表示されます。

(これはどのサブコマンドよりも優先されるため、`help` という名前のサブコマンドを定義することはできません)

例:

```
$ go run main.go --help
mycli

A simple CLI application

Usage:
  mycli [command] [args...]

Commands:
  greet  Greet someone
  help   Show this help message
```

## ライセンス
Suzume は MIT ライセンスのもとで公開されています。
