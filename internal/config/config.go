package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Simulation struct {
		TimeSeconds float64 `yaml:"time_seconds"` // общая продолжительность симуляции
		StepSeconds float64 `yaml:"step_seconds"` // шаг симуляции (для сбора snapshot'ов, построения графиков)
		Seed        int64   `yaml:"seed"`
	} `yaml:"simulation"`

	Traffic struct {
		BaseRPS     float64 `yaml:"base_rps"`     // rps, \lambda Пуассона
		UsersAmount int64   `yaml:"users_amount"` // кол-во возможных уникальных пользователей (сессий)
	} `yaml:"traffic"`

	Spikes []struct {
		At       float64 `yaml:"at"`       // секунда старта
		Duration float64 `yaml:"duration"` // длительность
		Factor   float64 `yaml:"factor"`   // множитель к base_rps
	} `yaml:"spikes"`

	Cluster struct {
		Servers int `yaml:"servers"` // кол-во серверов

		Bitrate          float64 `yaml:"bitrate"`          // mbps fullhd bitrate одного потока
		SegmentDuration  float64 `yaml:"segment_duration"` // длительность одного .ts-фрагмента (секунд)
		SegmentSizeBytes float64 // сколько весит (байт) один .ts-фрагмент

		CapMean float64 `yaml:"cap_mean_mbps"` // mbps средняя пропускная способность
		CapCV   float64 `yaml:"cap_cv"`        // относительное ст. отклонение пропускной способности
		OWDMean float64 `yaml:"owd_mean"`      // среднее one-way delay (2e2-delay), мс
		OWDCV   float64 `yaml:"owd_cv"`        // относительное ст. отклонение OWD

		SigmaServer float64 `yaml:"sigma_server"` // CV лог-нормального шума

		MaxRetriesPerSegment  int `yaml:"max_retries"`  // количество попыток запросить один и тот же .ts без смены сервера
		MaxSwitchesPerSession int `yaml:"max_switches"` // сколько раз можем менять сервер во время получения одного видео

		FirstPickRetries int     `yaml:"first_pick_retries"` // количество попыток начать сессию
		FirstPickBackoff float64 `yaml:"first_pick_backoff"` // интервал для следующего запроса на начало сессии
	} `yaml:"cluster"`

	Jitter struct {
		Tick       float64 `yaml:"tick_s"`           // шаг обновления OWD
		SpikeP     float64 `yaml:"spike_prob"`       // вероятность всплеска зедержки
		SpikeExtra float64 `yaml:"spike_extra"`      // +мс при всплеске
		SpikeDur   float64 `yaml:"spike_duration_s"` // длительность всплеска
	} `yaml:"jitter"`

	Balancer struct {
		Strategy   string `yaml:"strategy"`
		CHReplicas int    `yaml:"ch_replicas"`
	} `yaml:"balancer"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("error when parsing config: %w", err)
	}

	fillDefaults(&cfg)
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("error when validating config: %w", err)
	}
	return &cfg, nil
}

func fillDefaults(c *Config) {
	if c.Simulation.TimeSeconds == 0 {
		c.Simulation.TimeSeconds = 600
	}
	if c.Simulation.StepSeconds == 0 {
		c.Simulation.StepSeconds = 1
	}
	if c.Simulation.Seed == 0 {
		c.Simulation.Seed = time.Now().UnixNano()
	}
	if c.Traffic.BaseRPS == 0 {
		c.Traffic.BaseRPS = 200
	}
	if c.Traffic.UsersAmount == 0 {
		c.Traffic.UsersAmount = 10_000
	}
	if c.Cluster.Servers == 0 {
		c.Cluster.Servers = 5
	}
	if c.Cluster.Bitrate == 0 {
		c.Cluster.Bitrate = 4
	}
	if c.Cluster.SegmentDuration == 0 {
		c.Cluster.SegmentDuration = 6
	}
	if c.Cluster.CapMean == 0 {
		c.Cluster.CapMean = 500
	}
	if c.Cluster.CapCV == 0 {
		c.Cluster.CapCV = 0.2
	}
	if c.Cluster.OWDMean == 0 {
		c.Cluster.OWDMean = 100
	}
	if c.Cluster.OWDCV == 0 {
		c.Cluster.OWDCV = 0.3
	}
	if c.Cluster.SigmaServer == 0 {
		c.Cluster.SigmaServer = 0.25
	}
	if c.Cluster.MaxRetriesPerSegment == 0 {
		c.Cluster.MaxRetriesPerSegment = 2
	}
	if c.Cluster.MaxSwitchesPerSession == 0 {
		c.Cluster.MaxSwitchesPerSession = 4
	}
	if c.Cluster.FirstPickRetries == 0 {
		c.Cluster.FirstPickRetries = 3
	}
	if c.Cluster.FirstPickBackoff == 0 {
		c.Cluster.FirstPickBackoff = 100
	}
	if c.Jitter.Tick == 0 {
		c.Jitter.Tick = 1
	}
	if c.Jitter.SpikeP == 0 {
		c.Jitter.SpikeP = 0.002
	}
	if c.Jitter.SpikeExtra == 0 {
		c.Jitter.SpikeExtra = 300
	}
	if c.Jitter.SpikeDur == 0 {
		c.Jitter.SpikeDur = 5.0
	}
	if c.Balancer.Strategy == "" {
		c.Balancer.Strategy = "ch"
	}
	if c.Balancer.CHReplicas == 0 {
		c.Balancer.CHReplicas = 100
	}

	c.Cluster.SegmentSizeBytes = c.Cluster.Bitrate * 1_000_000 / 8 * c.Cluster.SegmentDuration
}

func validate(cfg *Config) error {
	return nil
}
