package netease

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"io"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

// weapi 的算法常量（网易云 weapi 的公开固定值，非密钥）
const (
	testPresetKey = "0CoJUm6Qyw8W8jud"
	testIV        = "0102030405060708"
)

// 测试 weapi 加密链路：secret 随机，故只校验结构、编码合法性与可逆性
func TestWeapi(t *testing.T) {
	ns := &Netease{}

	got, err := ns.weapi(map[string]string{"id": "167873"})
	if err != nil {
		t.Fatalf("weapi() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("weapi 应只返回 params 与 encSecKey，实际 %d 个: %v", len(got), got)
	}
	if got["params"] == "" || got["encSecKey"] == "" {
		t.Fatalf("params 与 encSecKey 都不应为空: %v", got)
	}
	if _, err := base64.StdEncoding.DecodeString(got["params"]); err != nil {
		t.Errorf("params 应为合法 base64: %v", err)
	}
	if _, err := hex.DecodeString(got["encSecKey"]); err != nil {
		t.Errorf("encSecKey 应为合法 hex: %v", err)
	}
	// RSA-1024 加密结果最多 128 字节，hex 后不超过 256 字符
	if len(got["encSecKey"]) > 256 {
		t.Errorf("encSecKey 长度不应超过 256，实际 %d", len(got["encSecKey"]))
	}

	t.Run("aesEncrypt 可逆", func(t *testing.T) {
		plain := []byte(`{"id":"167873"}`)
		encrypted := aesEncrypt(plain, []byte(testPresetKey), []byte(testIV))
		if bytes.Equal(encrypted, plain) {
			t.Fatal("密文不应等于明文")
		}

		block, err := aes.NewCipher([]byte(testPresetKey))
		if err != nil {
			t.Fatalf("aes.NewCipher: %v", err)
		}
		decrypted := make([]byte, len(encrypted))
		cipher.NewCBCDecrypter(block, []byte(testIV)).CryptBlocks(decrypted, encrypted)

		// 去掉 PKCS#7 填充
		pad := int(decrypted[len(decrypted)-1])
		if got := decrypted[:len(decrypted)-pad]; !bytes.Equal(got, plain) {
			t.Errorf("解密结果 = %q, want %q", got, plain)
		}
	})
}

// 测试 toSong 解析：字段映射、http 转 https、歌手拼接
func TestToSong(t *testing.T) {
	raw := `{
		"id": 167873,
		"name": "断桥残雪",
		"al": {"picUrl": "http://p1.music.126.net/cover.jpg"},
		"ar": [{"id": 5771, "name": "许嵩"}, {"id": 2, "name": "合辑"}]
	}`

	song := (&Netease{}).toSong(gjson.Parse(raw))

	if song.Id != "167873" {
		t.Errorf("Id = %q, want %q", song.Id, "167873")
	}
	if song.Name != "断桥残雪" {
		t.Errorf("Name = %q, want %q", song.Name, "断桥残雪")
	}
	if want := "https://p1.music.126.net/cover.jpg"; song.Picture != want {
		t.Errorf("Picture = %q, want %q（http 应转 https）", song.Picture, want)
	}
	if len(song.Artists) != 2 {
		t.Fatalf("Artists 长度 = %d, want 2", len(song.Artists))
	}
	if want := "许嵩/合辑"; song.GetArtist() != want {
		t.Errorf("GetArtist() = %q, want %q", song.GetArtist(), want)
	}

	t.Run("无 ar 字段", func(t *testing.T) {
		empty := (&Netease{}).toSong(gjson.Parse(`{"id": 1}`))
		if len(empty.Artists) != 0 {
			t.Errorf("Artists 应为空，实际 %v", empty.Artists)
		}
		if empty.GetArtist() != "" {
			t.Errorf("GetArtist() 应为空串，实际 %q", empty.GetArtist())
		}
	})
}

// 测试纯编解码工具
func TestUtils(t *testing.T) {
	t.Run("long2ip", func(t *testing.T) {
		if got := long2ip(0x01020304); got != "1.2.3.4" {
			t.Errorf("long2ip(0x01020304) = %q, want %q", got, "1.2.3.4")
		}
	})

	t.Run("hexBytes", func(t *testing.T) {
		if got := string(hexBytes([]byte{0x0f, 0xff})); got != "0fff" {
			t.Errorf("hexBytes = %q, want %q", got, "0fff")
		}
	})

	t.Run("base64Bytes", func(t *testing.T) {
		if got := string(base64Bytes([]byte("abc"))); got != "YWJj" {
			t.Errorf("base64Bytes = %q, want %q", got, "YWJj")
		}
	})

	t.Run("reverseBytes 两次还原", func(t *testing.T) {
		src := []byte{1, 2, 3, 4}
		once := reverseBytes(src)
		if !bytes.Equal(once, []byte{4, 3, 2, 1}) {
			t.Errorf("reverseBytes = %v, want [4 3 2 1]", once)
		}
		if !bytes.Equal(reverseBytes(once), src) {
			t.Errorf("反转两次应还原为 %v，实际 %v", src, reverseBytes(once))
		}
		if !bytes.Equal(src, []byte{1, 2, 3, 4}) {
			t.Errorf("reverseBytes 不应改动入参，实际 %v", src)
		}
	})

	t.Run("fromData 按键排序编码", func(t *testing.T) {
		data, err := io.ReadAll(fromData(map[string]string{"b": "2", "a": "1"}))
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if got := string(data); got != "a=1&b=2" {
			t.Errorf("fromData = %q, want %q", got, "a=1&b=2")
		}
	})

	t.Run("randomBytes 长度、字符集与失败分支", func(t *testing.T) {
		a, err := randomBytes(16, "abc")
		if err != nil {
			t.Fatalf("randomBytes: %v", err)
		}
		if len(a) != 16 {
			t.Errorf("长度 = %d, want 16", len(a))
		}
		for _, c := range a {
			if !strings.ContainsRune("abc", rune(c)) {
				t.Errorf("字符 %q 不在 charset 内", c)
			}
		}

		b, err := randomBytes(16, "abc")
		if err != nil {
			t.Fatalf("randomBytes: %v", err)
		}
		if bytes.Equal(a, b) {
			t.Error("两次生成不应相同")
		}

		if _, err := randomBytes(1, ""); err == nil {
			t.Error("charset 为空应返回错误")
		}
	})
}
