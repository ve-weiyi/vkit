// 诊断程序：抓取公网数据，验证 adapter/ipx、plugins/gsm、plugins/music/netease
// 的解析逻辑是否仍适配对方页面与接口（对端改版即会失效，故适合人工按需执行）。
// 只需能出外网、不需要凭证，但会真实请求第三方站点，故不放测试目录，非正式产物。
//
//	go run ./cmd/webfetch -source=all
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/ve-weiyi/vkit/adapter/iplocx"
	"github.com/ve-weiyi/vkit/plugins/gsm/service/brand"
	"github.com/ve-weiyi/vkit/plugins/gsm/service/device"
	"github.com/ve-weiyi/vkit/plugins/gsm/service/specification"
	"github.com/ve-weiyi/vkit/plugins/music/netease"
)

func main() {
	source := flag.String("source", "all", "数据源：ip | gsm | music | all")
	ip := flag.String("ip", "46.243.122.48", "待查询的公网 IP（ip 源用）")
	brandSlug := flag.String("brand-slug", "apple-phones-48", "GSM Arena 品牌 slug（gsm 源用）")
	deviceSlug := flag.String("device-slug", "apple_iphone_16_pro_max-13123", "GSM Arena 机型 slug（gsm 源用）")
	keyword := flag.String("keyword", "许嵩", "网易云搜索关键词（music 源用）")
	songID := flag.String("song-id", "167873", "网易云歌曲 ID（music 源用）")
	albumID := flag.String("album-id", "16953", "网易云专辑 ID（music 源用）")
	artistID := flag.String("artist-id", "5771", "网易云歌手 ID（music 源用）")
	playlistID := flag.String("playlist-id", "5771", "网易云歌单 ID（music 源用）")
	flag.Parse()

	var errs []error
	switch *source {
	case "ip":
		errs = append(errs, fetchIP(*ip))
	case "gsm":
		errs = append(errs, fetchGSM(*brandSlug, *deviceSlug))
	case "music":
		errs = append(errs, fetchMusic(*keyword, *songID, *albumID, *artistID, *playlistID))
	case "all":
		errs = append(errs,
			fetchIP(*ip),
			fetchGSM(*brandSlug, *deviceSlug),
			fetchMusic(*keyword, *songID, *albumID, *artistID, *playlistID),
		)
	default:
		fatalf("未知的 -source: %s（可选 ip | gsm | music | all）", *source)
	}

	if err := errors.Join(errs...); err != nil {
		fatalf("抓取失败: %v", err)
	}
}

// fetchIP 分别用百度与 ip-api 解析同一 IP，对比两者结果。
func fetchIP(ip string) error {
	baidu, err := iplocx.GetIpInfoByBaidu(ip)
	if err != nil {
		return fmt.Errorf("GetIpInfoByBaidu: %w", err)
	}
	printJSON("baidu "+ip, baidu)

	loc, err := iplocx.GetIpInfoByApi(ip)
	if err != nil {
		return fmt.Errorf("GetIpInfoByApi: %w", err)
	}
	printJSON("ip-api "+ip, loc)
	return nil
}

// fetchGSM 依次抓取品牌列表、该品牌机型列表与其中一个机型的规格。
func fetchGSM(brandSlug, deviceSlug string) error {
	brands, err := brand.GetAllBrands()
	if err != nil {
		return fmt.Errorf("GetAllBrands: %w", err)
	}
	fmt.Printf("品牌数: %d，首个: %s\n", len(brands), brands[0].Name)

	devices, err := device.GetDeviceList(brandSlug, 1)
	if err != nil {
		return fmt.Errorf("GetDeviceList(%s): %w", brandSlug, err)
	}
	printJSON("device "+brandSlug, devices)

	spec, err := specification.GetSpecification(deviceSlug)
	if err != nil {
		return fmt.Errorf("GetSpecification(%s): %w", deviceSlug, err)
	}
	fmt.Printf("规格: 品牌=%s 机型=%s\n", spec.Brand, spec.DeviceName)
	return nil
}

// fetchMusic 逐个验证网易云各接口，返回首个失败。
func fetchMusic(keyword, songID, albumID, artistID, playlistID string) error {
	ns := netease.New()

	search, err := ns.Search(keyword)
	if err != nil {
		return fmt.Errorf("Search(%s): %w", keyword, err)
	}
	fmt.Printf("搜索 %q 命中 %d 首\n", keyword, len(search))

	song, err := ns.Song(songID)
	if err != nil {
		return fmt.Errorf("Song(%s): %w", songID, err)
	}
	printJSON("song "+songID, song)

	link, err := ns.SongLink(songID)
	if err != nil {
		return fmt.Errorf("SongLink(%s): %w", songID, err)
	}
	printJSON("songLink "+songID, link)

	lyric, err := ns.Lyric(songID)
	if err != nil {
		return fmt.Errorf("Lyric(%s): %w", songID, err)
	}
	printJSON("lyric "+songID, lyric)

	album, err := ns.Album(albumID)
	if err != nil {
		return fmt.Errorf("Album(%s): %w", albumID, err)
	}
	printJSON("album "+albumID, album)

	artist, err := ns.Artist(artistID)
	if err != nil {
		return fmt.Errorf("Artist(%s): %w", artistID, err)
	}
	printJSON("artist "+artistID, artist)

	playlist, err := ns.Playlist(playlistID)
	if err != nil {
		return fmt.Errorf("Playlist(%s): %w", playlistID, err)
	}
	printJSON("playlist "+playlistID, playlist)
	return nil
}

func printJSON(label string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", label, err)
		return
	}
	fmt.Printf("%s:\n%s\n", label, data)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
