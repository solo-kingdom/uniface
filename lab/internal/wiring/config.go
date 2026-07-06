package wiring

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/solo-kingdom/uniface/lab/app/daghttp"
)

// LabConfig 定义 lab 各域默认配置。
//
// DAG 字段类型由 lab/app/daghttp 自治（daghttp.Config）；LabConfig 仅作
// 「跨域共享配置聚合」引用，不重复定义 DAG schema。
type LabConfig struct {
	KV       KVConfig       `yaml:"kv"`
	Config   ConfigDomain   `yaml:"config"`
	LB       LBConfig       `yaml:"lb"`
	Queue    QueueConfig    `yaml:"queue"`
	DAG      daghttp.Config `yaml:"dag"`
	Services ServicesConfig `yaml:"services"`
}

type KVConfig struct {
	Impl string `yaml:"impl"`
	Addr string `yaml:"addr"`
	Path string `yaml:"path"`
}

type ConfigDomain struct {
	Impl   string `yaml:"impl"`
	Addr   string `yaml:"addr"`
	Prefix string `yaml:"prefix"`
}

type LBConfig struct {
	Algo string `yaml:"algo"`
}

type QueueConfig struct {
	Impl     string   `yaml:"impl"`
	Addr     string   `yaml:"addr"`
	Brokers  []string `yaml:"brokers"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
}

type ServicesConfig struct {
	KV     string `yaml:"kv"`
	Config string `yaml:"config"`
	LB     string `yaml:"lb"`
	Queue  string `yaml:"queue"`
	DAG    string `yaml:"dag"`
}

// LoadConfig 加载 yaml 配置并应用环境变量覆盖。
func LoadConfig() (*LabConfig, error) {
	path := os.Getenv("LAB_CONFIG")
	if path == "" {
		candidates := []string{
			"configs/default.yaml",
			filepath.Join("lab", "configs", "default.yaml"),
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
		if path == "" {
			return nil, fmt.Errorf("config file not found; set LAB_CONFIG or create configs/default.yaml")
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	cfg := &LabConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	applyEnvOverrides(cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *LabConfig) {
	if v := os.Getenv("LAB_KV_IMPL"); v != "" {
		cfg.KV.Impl = v
	}
	if v := os.Getenv("LAB_CONFIG_IMPL"); v != "" {
		cfg.Config.Impl = v
	}
	if v := os.Getenv("LAB_LB_IMPL"); v != "" {
		cfg.LB.Algo = v
	}
	if v := os.Getenv("LAB_QUEUE_IMPL"); v != "" {
		cfg.Queue.Impl = v
	}
	if v := os.Getenv("LAB_DAG_IMPL"); v != "" {
		cfg.DAG.Store = v
	}
}
