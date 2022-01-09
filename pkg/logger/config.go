package logger

type Config struct {
	Level      string
	FileName   string
	MaxSize    int
	MaxAge     int
	MaxBackups int
	Comperss   bool
}

func DefaultConfig() *Config {
	return &Config{
		Level:      "INFO",
		FileName:   "./logs/debug.log",
		MaxSize:    500,
		MaxAge:     360,
		MaxBackups: 20,
		Comperss:   true,
	}
}
