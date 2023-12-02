package utils

import (
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type MultiToken string

type Config struct {
	JwtSecret       string   `envconfig:"JWT_SECRET" required:"true"`
	Https           bool     `envconfig:"HTTPS" default:"false"`
	CookieSameSite  bool     `envconfig:"COOKIE_SAME_SITE" default:"true"`
	AllowedUsers    []string `envconfig:"ALLOWED_USERS"`
	DatabaseUrl     string   `envconfig:"DATABASE_URL" required:"true"`
	RunMigrations   bool     `envconfig:"RUN_MIGRATIONS" default:"true"`
	Port            int      `envconfig:"PORT" default:"8080"`
	UploadRetention int      `envconfig:"UPLOAD_RETENTION" default:"15"`
	ExecDir         string
}

var config Config

func InitConfig() {

	execDir := getExecutableDir()

	godotenv.Load(filepath.Join(execDir, "drive.env"))
	err := envconfig.Process("", &config)
	if err != nil {
		panic(err)
	}
	config.ExecDir = execDir
}

func GetConfig() *Config {
	return &config
}
