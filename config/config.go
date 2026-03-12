package config

type Server struct {
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"`
}

type MySQL struct {
	DSN          string `yaml:"dsn"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
	MaxOpenConns int    `yaml:"max_open_conns"`
}

type Redis struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

type LLM struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

type AppConfig struct {
	Server Server `yaml:"server"`
	MySQL  MySQL  `yaml:"mysql"`
	Redis  Redis  `yaml:"redis"` 
	LLM    LLM    `yaml:"llm"`
}
