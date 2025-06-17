package config

import (
	"time"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Auth     AuthConfig     `mapstructure:"auth"`
	OAuth    OAuthConfig    `mapstructure:"oauth"`
	OpenVPN  OpenVPNConfig  `mapstructure:"openvpn"`
	Security SecurityConfig `mapstructure:"security"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	Mode         string        `mapstructure:"mode"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Name         string `mapstructure:"name"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	SSLMode      string `mapstructure:"ssl_mode"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type AuthConfig struct {
	SessionDuration        int      `mapstructure:"session_duration"`
	CaptivePortalTimeout   int      `mapstructure:"captive_portal_timeout"`
	JWTSecret             string   `mapstructure:"jwt_secret"`
	AllowedDomains        []string `mapstructure:"allowed_domains"`
}

type OAuthConfig struct {
	Google    OAuthProvider `mapstructure:"google"`
	Microsoft OAuthProvider `mapstructure:"microsoft"`
	GitHub    OAuthProvider `mapstructure:"github"`
}

type OAuthProvider struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
}

type OpenVPNConfig struct {
	Management OpenVPNManagement `mapstructure:"management"`
	Networks   OpenVPNNetworks   `mapstructure:"networks"`
}

type OpenVPNManagement struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type OpenVPNNetworks struct {
	CaptivePortal string `mapstructure:"captive_portal"`
	FullAccess    string `mapstructure:"full_access"`
}

type SecurityConfig struct {
	FailedLoginThreshold int `mapstructure:"failed_login_threshold"`
	IPBlockDuration      int `mapstructure:"ip_block_duration"`
	BcryptCost          int `mapstructure:"bcrypt_cost"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
