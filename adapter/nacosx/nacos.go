package nacosx

import (
	"fmt"
	"path"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

type NacosConfig struct {
	// Nacos相关配置
	NacosHost      string // Nacos服务地址
	NacosPort      uint64 // Nacos服务端口
	NacosNamespace string // Nacos命名空间
	NacosDataID    string // 配置DataID
	NacosGroup     string // 配置Group

	NacosUsername   string // Nacos用户名
	NacosPassword   string // Nacos密码
	NacosRuntimeDir string // Nacos运行时目录
}

// Listener 用于停止配置监听并释放 Nacos 客户端。
type Listener interface {
	Close() error
}

type nacosListener struct{ closeFn func() }

// Close 停止监听并释放客户端。SDK 的 CloseClient 没有返回值，这里统一成 error 以便实现 Listener。
func (l *nacosListener) Close() error {
	l.closeFn()
	return nil
}

// LoadConfigFromNacos 加载 Nacos 配置并监听变更，首次加载即回调一次。
// 返回的 Listener 必须被持有并最终 Close：否则监听 goroutine 与 Nacos 客户端会一直存活。
func LoadConfigFromNacos(option *NacosConfig, callback func(content string)) (Listener, error) {
	// 1. 创建Nacos客户端配置
	serverConfigs := []constant.ServerConfig{
		{
			IpAddr: option.NacosHost,
			Port:   option.NacosPort, // 默认端口，可根据实际修改
		},
	}

	clientConfig := &constant.ClientConfig{
		Username:            option.NacosUsername,
		Password:            option.NacosPassword,
		NamespaceId:         option.NacosNamespace, // 命名空间ID（默认为public）
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              path.Join(option.NacosRuntimeDir, "logs"),
		CacheDir:            path.Join(option.NacosRuntimeDir, "cache"),
		LogLevel:            "info",
	}

	// 2. 创建Nacos配置客户端
	client, err := clients.NewConfigClient(
		vo.NacosClientParam{
			ClientConfig:  clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Nacos client: %w", err)
	}

	// 3. 首次从Nacos获取配置
	content, err := client.GetConfig(vo.ConfigParam{
		DataId: option.NacosDataID,
		Group:  option.NacosGroup,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get Nacos config: %w", err)
	}

	// 4. 解析Nacos配置（修复：传指针给Unmarshal）
	callback(content)

	// 5. 监听Nacos配置变化：更新时直接修改cfg指向的内存
	err = client.ListenConfig(vo.ConfigParam{
		DataId: option.NacosDataID,
		Group:  option.NacosGroup,
		OnChange: func(namespace, group, dataId, data string) {
			//logger.GetLogger().Info("Nacos config changed: dataId=%s, group=%s\n", dataId, group)
			// 重新解析变更后的配置到原指针
			callback(data)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to listen Nacos config changes: %w", err)
	}

	return &nacosListener{closeFn: client.CloseClient}, nil
}
