package suzume

import (
	"io"
	"os"
)

type Config struct {
	// 親のアプリケーションの設定を継承
	inherit bool

	// 標準出力の書き込み先
	Log io.Writer

	// エラー出力の書き込み先
	ErrorLog io.Writer
}

func defaultConfig() Config {
	return Config{
		inherit:  true,
		Log:      os.Stdout,
		ErrorLog: os.Stderr,
	}
}