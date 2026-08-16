package config

import (
	"os"
	"strings"
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

	// TrustedProxies lists the addresses whose X-Forwarded-For header may be
	// believed. Empty means none, which is correct when VPN clients reach the
	// portal directly.
	//
	// This matters more than it looks: sessions are keyed on the client's
	// tunnel address, so a client whose forwarded header is trusted can bind a
	// session to an address it does not own, granting network access to
	// someone else's tunnel.
	TrustedProxies []string `mapstructure:"trusted_proxies"`
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
	SessionDuration      int      `mapstructure:"session_duration"`
	CaptivePortalTimeout int      `mapstructure:"captive_portal_timeout"`
	JWTSecret            string   `mapstructure:"jwt_secret"`
	AllowedDomains       []string `mapstructure:"allowed_domains"`
}

type OAuthConfig struct {
	Google    OAuthProvider  `mapstructure:"google"`
	Microsoft OAuthProvider  `mapstructure:"microsoft"`
	GitHub    OAuthProvider  `mapstructure:"github"`
	GitLab    GitLabProvider `mapstructure:"gitlab"`
}

type OAuthProvider struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
}

type OpenVPNConfig struct {
	Management OpenVPNManagement `mapstructure:"management"`
	Networks   OpenVPNNetworks   `mapstructure:"networks"`
	PKI        OpenVPNPKI        `mapstructure:"pki"`
	Server     OpenVPNServer     `mapstructure:"server"`
}

// OpenVPNPKI locates the certificate authority used to issue per-user client
// certificates.
type OpenVPNPKI struct {
	CACert string `mapstructure:"ca_cert"`
	CAKey  string `mapstructure:"ca_key"`
	TAKey  string `mapstructure:"ta_key"`

	// AutoCreate generates a CA when none exists. Convenient for development;
	// a real deployment should provision its CA deliberately.
	AutoCreate bool `mapstructure:"auto_create"`

	// ClientCertDays is how long issued client certificates last.
	ClientCertDays int `mapstructure:"client_cert_days"`
}

// OpenVPNServer describes the endpoint written into client profiles.
type OpenVPNServer struct {
	Host  string `mapstructure:"host"`
	Port  int    `mapstructure:"port"`
	Proto string `mapstructure:"proto"`
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
	BcryptCost           int `mapstructure:"bcrypt_cost"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

func Load(configPath string) (*Config, error) {
	// Load environment variables first
	loadEnvVars()

	viper.SetConfigFile(configPath)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	// Override with environment variables
	overrideWithEnvVars(&config)

	return &config, nil
}

// loadEnvVars loads environment variables into viper
func loadEnvVars() {
	// Server
	viper.BindEnv("server.port", "SERVER_PORT")
	viper.BindEnv("server.mode", "SERVER_MODE")
	viper.BindEnv("server.read_timeout", "SERVER_READ_TIMEOUT")
	viper.BindEnv("server.write_timeout", "SERVER_WRITE_TIMEOUT")

	// Database
	viper.BindEnv("database.host", "DB_HOST")
	viper.BindEnv("database.port", "DB_PORT")
	viper.BindEnv("database.name", "DB_NAME")
	viper.BindEnv("database.user", "DB_USER")
	viper.BindEnv("database.password", "DB_PASSWORD")
	viper.BindEnv("database.ssl_mode", "DB_SSL_MODE")
	viper.BindEnv("database.max_open_conns", "DB_MAX_OPEN_CONNS")
	viper.BindEnv("database.max_idle_conns", "DB_MAX_IDLE_CONNS")

	// Redis
	viper.BindEnv("redis.addr", "REDIS_ADDR")
	viper.BindEnv("redis.password", "REDIS_PASSWORD")
	viper.BindEnv("redis.db", "REDIS_DB")
	viper.BindEnv("redis.pool_size", "REDIS_POOL_SIZE")

	// Auth
	viper.BindEnv("auth.session_duration", "AUTH_SESSION_DURATION")
	viper.BindEnv("auth.captive_portal_timeout", "AUTH_CAPTIVE_PORTAL_TIMEOUT")
	viper.BindEnv("auth.jwt_secret", "AUTH_JWT_SECRET")

	// OAuth - Google
	viper.BindEnv("oauth.google.client_id", "OAUTH_GOOGLE_CLIENT_ID")
	viper.BindEnv("oauth.google.client_secret", "OAUTH_GOOGLE_CLIENT_SECRET")
	viper.BindEnv("oauth.google.redirect_url", "OAUTH_GOOGLE_REDIRECT_URL")

	// OAuth - Microsoft
	viper.BindEnv("oauth.microsoft.client_id", "OAUTH_MICROSOFT_CLIENT_ID")
	viper.BindEnv("oauth.microsoft.client_secret", "OAUTH_MICROSOFT_CLIENT_SECRET")
	viper.BindEnv("oauth.microsoft.redirect_url", "OAUTH_MICROSOFT_REDIRECT_URL")

	// OAuth - GitHub
	viper.BindEnv("oauth.github.client_id", "OAUTH_GITHUB_CLIENT_ID")
	viper.BindEnv("oauth.github.client_secret", "OAUTH_GITHUB_CLIENT_SECRET")
	viper.BindEnv("oauth.github.redirect_url", "OAUTH_GITHUB_REDIRECT_URL")

	// OpenVPN
	viper.BindEnv("openvpn.management.host", "OPENVPN_MANAGEMENT_HOST")
	viper.BindEnv("openvpn.management.port", "OPENVPN_MANAGEMENT_PORT")
	viper.BindEnv("openvpn.networks.captive_portal", "OPENVPN_CAPTIVE_PORTAL_NETWORK")
	viper.BindEnv("openvpn.networks.full_access", "OPENVPN_FULL_ACCESS_NETWORK")
	viper.BindEnv("openvpn.pki.ca_cert", "OPENVPN_CA_CERT")
	viper.BindEnv("openvpn.pki.ca_key", "OPENVPN_CA_KEY")
	viper.BindEnv("openvpn.pki.ta_key", "OPENVPN_TA_KEY")
	viper.BindEnv("openvpn.server.host", "OPENVPN_SERVER_HOST")
	viper.BindEnv("openvpn.server.port", "OPENVPN_SERVER_PORT")

	// Security
	viper.BindEnv("security.failed_login_threshold", "SECURITY_FAILED_LOGIN_THRESHOLD")
	viper.BindEnv("security.ip_block_duration", "SECURITY_IP_BLOCK_DURATION")
	viper.BindEnv("security.bcrypt_cost", "SECURITY_BCRYPT_COST")

	// Logging
	viper.BindEnv("logging.level", "LOG_LEVEL")
	viper.BindEnv("logging.format", "LOG_FORMAT")
	viper.BindEnv("logging.output", "LOG_OUTPUT")
}

// overrideWithEnvVars overrides config with direct environment variable reads for complex types
func overrideWithEnvVars(config *Config) {
	// Handle allowed domains (comma-separated)
	if domains := os.Getenv("AUTH_ALLOWED_DOMAINS"); domains != "" {
		config.Auth.AllowedDomains = strings.Split(domains, ",")
		for i, domain := range config.Auth.AllowedDomains {
			config.Auth.AllowedDomains[i] = strings.TrimSpace(domain)
		}
	}

	// Handle timeouts (convert string to time.Duration)
	if timeout := os.Getenv("SERVER_READ_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			config.Server.ReadTimeout = d
		}
	}
	if timeout := os.Getenv("SERVER_WRITE_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			config.Server.WriteTimeout = d
		}
	}
}
