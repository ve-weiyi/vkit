package kafkax

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"

	"github.com/ve-weiyi/pkg/kit/mqx"
)

// Kafka Kafka实现
type Kafka struct {
	client sarama.Client
	config *Config
}

// Config Kafka配置
type Config struct {
	Brokers []string // Kafka broker地址列表
	Version string   // Kafka版本 (例如: "2.8.0")
	SASL    *SASLConfig
	TLS     *TLSConfig
}

// SASLConfig SASL认证配置
type SASLConfig struct {
	Enable    bool
	Mechanism string // PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
	User      string
	Password  string
}

// TLSConfig TLS配置
type TLSConfig struct {
	Enable bool
	// 可以添加更多TLS配置
}

// NewKafka 创建Kafka实例
func NewKafka(config *Config) (*Kafka, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("brokers cannot be empty")
	}

	// 创建Sarama配置
	saramaConfig := sarama.NewConfig()

	// 设置Kafka版本
	if config.Version != "" {
		version, err := sarama.ParseKafkaVersion(config.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to parse kafka version: %w", err)
		}
		saramaConfig.Version = version
	}

	// 配置SASL
	if config.SASL != nil && config.SASL.Enable {
		saramaConfig.Net.SASL.Enable = true
		saramaConfig.Net.SASL.User = config.SASL.User
		saramaConfig.Net.SASL.Password = config.SASL.Password
		saramaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	}

	// 配置TLS
	if config.TLS != nil && config.TLS.Enable {
		saramaConfig.Net.TLS.Enable = true
	}

	// 创建Kafka客户端
	client, err := sarama.NewClient(config.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka client: %w", err)
	}

	return &Kafka{
		client: client,
		config: config,
	}, nil
}

// Producer 实现 mqx.MessageQueue 接口
func (k *Kafka) Producer() (mqx.Producer, error) {
	return newProducer(k.client, k.config)
}

// Consumer 实现 mqx.MessageQueue 接口
func (k *Kafka) Consumer(config *mqx.ConsumerConfig) (mqx.Consumer, error) {
	return newConsumer(k.client, k.config, config)
}

// Close 实现 mqx.MessageQueue 接口
func (k *Kafka) Close() error {
	if k.client != nil {
		return k.client.Close()
	}
	return nil
}

// Ping 实现 mqx.MessageQueue 接口
func (k *Kafka) Ping(ctx context.Context) error {
	if k.client == nil || k.client.Closed() {
		return fmt.Errorf("client is closed")
	}
	return nil
}

// 确保实现了接口
var _ mqx.MessageQueue = (*Kafka)(nil)
