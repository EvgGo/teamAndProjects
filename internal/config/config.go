package config

import (
	"github.com/ilyakaznacheev/cleanenv"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
	"os"
	"time"
)

type Config struct {
	Env      string `yaml:"env" env-default:"local"`
	LogFile  string `yaml:"log_file" env:"LOG_FILE"`
	LogLevel string `yaml:"log_level" env-default:"info" env:"LOG_LEVEL"`

	Auth        AuthConfig       `yaml:"auth"`
	AuthService AuthGRPCConfig   `yaml:"auth_service" env-required:"true"`
	Limits      LimitsConfig     `yaml:"limits"`
	Workspace   WorkspaceConfig  `yaml:"workspace"`
	Postgres    DatabaseConfig   `yaml:"postgres" env-required:"true"`
	GRPC        GRPCConfig       `yaml:"grpc"`
	Migrations  MigrationsConfig `yaml:"migrations"`
	DialConfig  DialConfig       `yaml:"dial"`
}

type GRPCConfig struct {
	Port    int           `yaml:"port" env:"GRPC_PORT"`
	Timeout time.Duration `yaml:"timeout" env:"GRPC_TIMEOUT"`
}

type AuthConfig struct {
	JWT JWTVerifyConfig `yaml:"jwt"`
}

// JWTVerifyConfig - параметры проверки JWT.
type JWTVerifyConfig struct {
	Issuer     string        `yaml:"issuer" env:"JWT_ISSUER"`
	SigningKey string        `yaml:"signing_key" env:"JWT_SIGNING_KEY"`
	ClockSkew  time.Duration `yaml:"clock_skew" env:"JWT_CLOCK_SKEW"`
}

type LimitsConfig struct {
	DefaultPageSize int32 `yaml:"default_page_size" env:"DEFAULT_PAGE_SIZE" env-default:"20"`
	MaxPageSize     int32 `yaml:"max_page_size" env:"MAX_PAGE_SIZE" env-default:"100"`
}

type WorkspaceConfig struct {
	Invites      InvitesConfig      `yaml:"invites"`
	JoinRequests JoinRequestsConfig `yaml:"join_requests"`
}

type InvitesConfig struct {
	// default_ttl - дефолтный срок жизни приглашения.
	// Пример значения в env/yaml: "168h" (7 дней), "24h".
	DefaultTTL time.Duration `yaml:"default_ttl" env:"INVITE_DEFAULT_TTL" env-default:"168h"`

	// MaxActivePerTeam - мягкий лимит на активные инвайты у команды
	// 0 => без лимита.
	MaxActivePerTeam int32 `yaml:"max_active_per_team" env:"INVITE_MAX_ACTIVE_PER_TEAM" env-default:"0"`
}

type JoinRequestsConfig struct {
	// MaxPendingPerUser - сколько pending-заявок может иметь один пользователь одновременно
	// 0 => без лимита.
	MaxPendingPerUser int32 `yaml:"max_pending_per_user" env:"JOINREQ_MAX_PENDING_PER_USER" env-default:"0"`
}

type DatabaseConfig struct {
	Host        string        `yaml:"host" env:"PG_HOST" env-required:"true"`
	Port        int           `yaml:"port" env:"PG_PORT" env-required:"true"`
	User        string        `yaml:"user" env:"PG_USER" env-required:"true"`
	Password    string        `yaml:"password" env:"PG_PASSWORD" env-required:"true"`
	DBName      string        `yaml:"DBName" env:"PG_DBNAME" env-required:"true"`
	ConnectConf ConnectConfig `yaml:"connect_conf"`
}

type AuthGRPCConfig struct {
	Host    string        `yaml:"host"`
	Port    int           `yaml:"port"`
	Timeout time.Duration `yaml:"timeout"`
}

type ConnectConfig struct {
	// Максимум соединений в пуле.
	MaxConns int32 `yaml:"max_conns" env:"PG_MAX_CONNS"`

	// Минимум соединений, которые пул будет стараться держать открытыми
	MinConns int32 `yaml:"min_conns" env:"PG_MIN_CONNS"`

	// Максимальная продолжительность жизни соединения
	MaxConnLifetime time.Duration `yaml:"max_conn_lifetime" env:"PG_MAX_CONN_LIFETIME"`

	// Максимальное время простоя соединения.
	MaxConnIdleTime time.Duration `yaml:"max_conn_idle_time" env:"PG_MAX_CONN_IDLE_TIME"`
}

type MigrationsConfig struct {
	Enabled bool   `yaml:"enabled" env:"MIGRATIONS_ENABLED" env-default:"false"`
	Dir     string `yaml:"dir" env:"MIGRATIONS_DIR"`
}

type DialConfig struct {
	Attempts          int
	PerAttemptTimeout time.Duration // таймаут на одну попытку Dial
	BaseBackoff       time.Duration // стартовый бэкофф
	MaxBackoff        time.Duration // верхняя граница бэкоффа
	UseTLS            bool          // если true - TLS
	ExtraOptions      []grpc.DialOption
}

func MustLoad(name string) *Config {
	path := os.Getenv(name)

	if path == "" {
		panic("путь в конфигу пуст")
	}

	return MustLoadPath(path)
}

func MustLoadPath(configPath string) *Config {
	// проверяем существует ли файл
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("конфиг файл не существует: " + configPath)
	}

	// Читаем файл полностью в память
	raw, err := os.ReadFile(configPath)
	if err != nil {
		panic("не удалось прочитать файл конфига: " + err.Error())
	}

	// Расширяем в нeм все ${VARS}, os.Getenv("VARS") или "" если не задана
	expanded := os.ExpandEnv(string(raw))

	// Декодируем развeрнутый YAML
	var cfg Config
	if err = yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		panic("не получилось распарсить YAML: " + err.Error())
	}

	// читаем env теги
	if err = cleanenv.ReadEnv(&cfg); err != nil {
		panic("не получилось прочитать конфиг: " + err.Error())
	}

	return &cfg
}
