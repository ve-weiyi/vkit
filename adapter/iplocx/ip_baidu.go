package iplocx

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
)

const (
	ipLookupTimeout  = 2 * time.Second
	cacheTTL         = 24 * time.Hour
	negativeCacheTTL = 5 * time.Minute
	maxCacheEntries  = 10000
)

// httpClient 供测试注入；默认带超时，避免外部接口抖动拖住调用方。
var httpClient = &http.Client{Timeout: ipLookupTimeout}

type ipCacheEntry struct {
	info     *BaiduLocation
	expireAt time.Time
}

var (
	cacheMu sync.Mutex
	ipCache = make(map[string]ipCacheEntry)
)

type BaiduIpResp struct {
	Status       string           `json:"status"`
	T            string           `json:"t"`
	SetCacheTime string           `json:"set_cache_time"`
	Data         []*BaiduLocation `json:"data"`
}

type BaiduLocation struct {
	ExtendedLocation string `json:"ExtendedLocation"` // 扩展位置信息
	OriginQuery      string `json:"OriginQuery"`      // 查询的原始地址
	Appinfo          string `json:"appinfo"`          // 应用信息
	DispType         int    `json:"disp_type"`        // 显示类型
	Fetchkey         string `json:"fetchkey"`         // 获取键
	Location         string `json:"location"`         // 地址,广东省深圳市 阿里云
	Origip           string `json:"origip"`           // 原始 IP 地址,119.23.144.144
	Origipquery      string `json:"origipquery"`      // 原始 IP 地址查询
	Resourceid       string `json:"resourceid"`       // 资源 ID
	RoleId           int    `json:"role_id"`          // 角色 ID
	ShareImage       int    `json:"shareImage"`       // 分享图片
	ShowLikeShare    int    `json:"showLikeShare"`    // 显示喜欢分享
	Showlamp         string `json:"showlamp"`         // 是否显示灯泡
	Titlecont        string `json:"titlecont"`        // 标题内容
	Tplt             string `json:"tplt"`             // 模板信息
}

// GetIpSource 获取ip对应的城市地区
func GetIpInfoByBaidu(ip string) (*BaiduLocation, error) {
	if ip == "" {
		return nil, fmt.Errorf("ip is empty")
	}

	if info, ok := loadCache(ip); ok {
		if info == nil {
			return nil, fmt.Errorf("ip query failed, cached ip:%s", ip)
		}
		return info, nil
	}

	resp, err := httpClient.Get("http://opendata.baidu.com/api.php?query=" + url.QueryEscape(ip) + "&co=&resource_id=6006&oe=utf8")
	if err != nil {
		storeCache(ip, nil, negativeCacheTTL)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		storeCache(ip, nil, negativeCacheTTL)
		return nil, fmt.Errorf("ip query status %d, ip:%s", resp.StatusCode, ip)
	}

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		storeCache(ip, nil, negativeCacheTTL)
		return nil, err
	}

	var result BaiduIpResp
	if err := jsoniter.Unmarshal(out, &result); err != nil {
		storeCache(ip, nil, negativeCacheTTL)
		return nil, err
	}

	if len(result.Data) == 0 {
		storeCache(ip, nil, negativeCacheTTL)
		return nil, fmt.Errorf("ip query fail,no data ip:%v", ip)
	}

	storeCache(ip, result.Data[0], cacheTTL)
	return result.Data[0], nil
}

func loadCache(ip string) (*BaiduLocation, bool) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	entry, ok := ipCache[ip]
	if !ok || time.Now().After(entry.expireAt) {
		delete(ipCache, ip)
		return nil, false
	}
	return entry.info, true
}

func storeCache(ip string, info *BaiduLocation, ttl time.Duration) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if len(ipCache) >= maxCacheEntries {
		now := time.Now()
		for key, entry := range ipCache {
			if now.After(entry.expireAt) {
				delete(ipCache, key)
			}
		}
		if len(ipCache) >= maxCacheEntries {
			clear(ipCache)
		}
	}

	ipCache[ip] = ipCacheEntry{info: info, expireAt: time.Now().Add(ttl)}
}

func stripPort(ip string) string {
	host, _, err := net.SplitHostPort(ip)
	if err != nil {
		return ip
	}
	return host
}

func isLocalhost(ip string) bool {
	if ip == "localhost" {
		return true
	}
	addr := net.ParseIP(ip)
	return addr != nil && addr.IsLoopback()
}

func GetIpSourceByBaidu(ip string) string {
	ip = stripPort(ip)

	if isLocalhost(ip) {
		return "本机局域网"
	}

	info, err := GetIpInfoByBaidu(ip)
	if err != nil {
		return "未知IP"
	}

	return info.Location
}
