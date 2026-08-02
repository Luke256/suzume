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

```go
cmd, err := suzume.NewCommand("greet", "Greet someone", func(name string, num int) error {
    println("Hello,", name, "you have", num, "messages.")
    return nil
})
```

Call `cmd.Run()` to execute the command. You can also pass arguments directly, as in `cmd.Run("Luke", "5")`.

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

- `cli:"0"`: Defines a positional argument. The integer is its zero-based position, so `0` means the first argument. A non-integer value defines the long option name.
- `short:"n"`: Defines a short option name. In this example, `Num` can be set with `-n`.
- `usage:"..."`: Describes the argument or option in generated help.

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
> Boolean option fields behave as flags: `--flag` sets the field to `true`. Use the `--flag=false` form to set an explicit boolean value.

### Defining subcommands

Create an application with `suzume.NewApp`, then add commands with `AddCommand`:

```go
cmd1, _ := suzume.NewCommand("foo", "bar", func() error {
    // do something
    return nil
})

cmd2, _ := suzume.NewCommand("hoge", "fuga", func() error {
    // do something
    return nil
})

app := suzume.NewApp("myapp", "My CLI Application")
app.AddCommand(cmd1) // myapp foo
app.AddCommand(cmd2) // myapp hoge
app.Run()
```

> [!IMPORTANT]
> An application may contain any number of commands and sub-applications, but **the application itself cannot be executed as a command**. This is an intentional constraint: applications are designed to act as hubs for subcommands. If an application were executable, the `subcmd` part of an invocation such as `myapp subcmd` would be ambiguous—it could be either a positional argument or a subcommand.

### Nesting subcommands

Subcommands can contain further subcommands, allowing you to build complex command hierarchies:

```go
root := suzume.NewApp("root", "Root Command")
sub1 := suzume.NewApp("sub1", "Sub Command 1")
cmd, _ := suzume.NewCommand("cmd", "A command", func() error {
    // do something
    return nil
})
sub1.AddCommand(cmd) // root sub1 cmd
root.AddApp(sub1)
root.Run() // go run main.go sub1 cmd
```

## Config

Use `suzume.Config` for settings such as log output destinations and signal handling.
Apply a configuration to a command or application with the `SetConfig` method.

Unless explicitly configured with `SetConfig`, commands and applications inherit their parent's configuration.

### Log configuration

Set the log output destination with the `Log` field of `suzume.Config`, and the error log destination with `ErrorLog`:

```go
cmd.SetConfig(suzume.Config{
    Log:      os.Stdout,
    ErrorLog: os.Stderr,
})
```

By default, normal output is written to standard output and errors are written to standard error.

### Signal handling

Signals listed in the `IgnoreSignals` field of `suzume.Config` are intercepted while the command is running. Receiving one cancels the command's context instead of immediately terminating the process, allowing the command to perform shutdown processing.

```go
cmd.SetConfig(suzume.Config{
    Log:           os.Stdout,
    ErrorLog:      os.Stderr,
    IgnoreSignals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
})
```

With this configuration, when a user attempts to terminate the process with `Ctrl+C` or a similar action, the command does not exit immediately and can wait for the signal through `ctx.Done()`.

```go
// Example command processing
func commandFunc(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            // Clean up after receiving a signal
            return nil
        default:
        }
        // Normal processing
    }
}
```

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
  greet                Greet someone
  help                 Show this help message
```

## License

Suzume is released under the MIT License.
