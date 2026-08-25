# Suzume CLI Framework

[日本語](README.md) | **English**

> [!NOTE]
> This README was translated from Japanese using machine translation. If there are any discrepancies, please refer to the [Japanese version](README.md).

Suzume is a framework for building command-line applications in Go.

## Features

Suzume supports projects of any size, from a simple application with a single command to a more complex application with nested subcommands.

For example, the following code defines a command that prints a short notification:

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

Despite its simplicity, Suzume can provide comparable functionality with less code than other CLI frameworks such as Cobra and urfave/cli.
The following features make CLI application development in Go simpler and more efficient:

- **Scalability**: Start with a lightweight command and add subcommands or other features as needed.
- **Simple command definitions**: Infer command arguments and options from function signatures or struct tags.
- **Custom type support**: Use custom argument and option types that implement `encoding.TextUnmarshaler`.
- **Explicit defaults**: Define default option values in code.
- **Automatic help generation**: Generate help from command descriptions, arguments, and options.
- **No third-party dependencies**: Suzume is built entirely with the Go standard library.

## Installation

Suzume requires Go 1.22 or later.

```bash
go get github.com/Luke256/suzume
```

## Defining commands

Commands can be defined in two ways: with `suzume.NewCommand`, or from a struct with `suzume.UseCommand`.

### Defining a command with `suzume.NewCommand`

`suzume.NewCommand` defines a command name, description, and implementation in one call. Positional arguments are inferred from the function signature.

Handlers may accept `context.Context` and positional arguments convertible from strings (strings, numbers, or types implementing `encoding.TextUnmarshaler`). Booleans and slices are not supported. A handler must return either nothing or a single `error`. `NewCommand` rejects `nil` handlers and invalid signatures when the command is created.

```go
cmd, err := suzume.NewCommand("greet", "Greet someone", func(name string, num int) error {
    println("Hello,", name, "you have", num, "messages.")
    return nil
})
```

Call `cmd.Run()` to execute the command. You can also pass arguments directly, as in `cmd.Run("Luke", "5")`.

You may call `Run` or `RunContext` repeatedly or concurrently on the same command; arguments are bound independently for each execution. Concurrent runs also invoke the handler concurrently, so callers must synchronize any state shared by the handler or configured output writers.

Use `cmd.RunAndExit()` when the process should exit with status code 1 if the command returns an error.

### Defining a command with `suzume.UseCommand`

`suzume.UseCommand` supports more detailed command definitions.

First, define a struct that implements `suzume.CommandDefinition`. Implement both `Run()` and `Default()` with pointer receivers. Embedding `suzume.Command` is optional. The struct's fields represent command arguments and options, and tags specify their command-line metadata.

The embedded `suzume.Command` and unexported fields are not recognized as arguments or options.

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

Suzume recognizes the following struct tags:

- `cli:"0"`: Defines a positional argument's order with a non-negative integer. Arguments are processed from the smallest value, while a non-integer value defines the long option name.
- `short:"n"`: Defines a short option name. In this example, `Num` can be set with `-n`.
- `usage:"..."`: Describes the argument or option in generated help.
- `default:"..."`: Overrides the default-value text shown in help without changing the runtime value. Use an empty string to hide the default value.

Positional arguments are processed in ascending order of their non-negative integer indexes. Option names and short names must not be duplicated, including across the two forms. The built-in help identifiers `help` and `h` are reserved. `UseCommand` rejects invalid tags when the command is created.

Then create the command with `suzume.UseCommand`:

```go
cmd, err := suzume.UseCommand[*GreetCommand]("greet", "Greet someone")
```

`suzume.Command` provides a `Run()` method that does nothing and a `Default()` method that leaves zero values unchanged. If you do not embed it, explicitly implement both methods on the pointer type.

To define a default option value, set the field in `Default()`:

```go
func (r *GreetCommand) Default() {
    r.Num = 5
}
```

> [!IMPORTANT]
> Pass a pointer-to-struct type to `UseCommand`, as in `UseCommand[*GreetCommand]`.

> [!NOTE]
> Boolean option fields behave as flags: `--flag` sets the field to `true`. Use the `--flag=false` form to set an explicit boolean value. Boolean defaults are not shown in help.
>
> Slice options accept multiple following values in the separated form, such as `--tag stable fast`. The valued form, such as `--tag=stable`, produces a one-element slice containing only the specified value.
>
> Use `default:"..."` to customize help output for custom types or to mask/hide sensitive values. For example, `default:"from environment"` displays that text instead of the runtime value, while `default:""` hides the default value.

### Defining subcommands

Create an application with `suzume.NewApp`, then add commands with `AddCommand`. For static application names, you can also use `suzume.MustNewApp`, which panics if creation fails:

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

`AddCommand` and `AddApp` register a copy of the command or sub-application as it exists when the method is called. Configure aliases, settings, and children before adding them. Structural changes made to the original command or sub-application afterward are not reflected in the registered application. Command, sub-application, and alias names cannot begin with `-`.

> [!IMPORTANT]
> An application may contain any number of commands and sub-applications, but **the application itself cannot be executed as a command**. This is an intentional constraint: applications are designed to act as hubs for subcommands. If an application were executable, the `subcmd` part of an invocation such as `myapp subcmd` would be ambiguous—it could be either a positional argument or a subcommand.

### Nesting subcommands

Subcommands can contain further subcommands, allowing you to build complex command hierarchies:

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

Settings such as log destinations and signal handling can be configured for each command. Create a configuration with `suzume.DefaultConfig()` or `suzume.NewConfig(options ...ConfigOption)`, then pass it to a command or application's `SetConfig` method.

Use the following configuration API:

- `suzume.DefaultConfig()` creates the default configuration.
- `suzume.NewConfig(options ...ConfigOption)` starts from the defaults and applies the supplied options.
- `suzume.WithLog(writer)` sets the normal log destination.
- `suzume.WithErrorLog(writer)` sets the error log destination.
- `suzume.WithIgnoreSignals(signals...)` selects signals to intercept during execution.

### Defaults and inheritance

`DefaultConfig()` and `NewConfig()` without options both use these defaults:

- Normal log: `os.Stdout`
- Error log: `os.Stderr`
- Intercepted signals: none

Commands and applications without explicit configuration inherit their parent's configuration at runtime. Calling `SetConfig` makes that command or application use the supplied configuration instead of inheriting its parent. The defaults are used when there is no parent.

To set the defaults explicitly:

```go
app.SetConfig(suzume.DefaultConfig())
```

### Log configuration

Use `WithLog` and `WithErrorLog` to change the output destinations. Passing `nil` keeps the corresponding default destination. To intentionally suppress output, pass `io.Discard` rather than `nil`.

```go
cmd.SetConfig(suzume.NewConfig(
    suzume.WithLog(logWriter),
    suzume.WithErrorLog(io.Discard), // Suppress error logs
))
```

For example, `suzume.WithLog(nil)` keeps `os.Stdout`, while `suzume.WithErrorLog(nil)` keeps `os.Stderr`.

### Signal handling

Signals passed to `WithIgnoreSignals` are intercepted while the command is running. Receiving one cancels the command's context instead of immediately terminating the process, allowing shutdown work through `ctx.Done()`.

```go
cmd.SetConfig(suzume.NewConfig(
    suzume.WithIgnoreSignals(os.Interrupt, syscall.SIGTERM),
))
```

By default, Suzume does not intercept signals, so normal process signal handling remains in effect.

## Automatic help generation

Suzume automatically generates help messages from command descriptions, arguments, and options. Users can display help with the `--help` or `-h` option. A `help` subcommand is also added automatically to applications.

Because it takes precedence over every other subcommand, you cannot define a subcommand named `help`.

Example:

```text
$ go run main.go --help
mycli

A simple CLI application

Usage:
  mycli [command] [args...]

Commands:
  greet  Greet someone
  help   Show this help message
```

## License

Suzume is released under the MIT License.
