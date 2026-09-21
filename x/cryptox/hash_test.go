package cryptox_test

import (
	"log"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/ve-weiyi/vkit/x/cryptox"
)

// 一个字节Byte由8位bit。8*64=512
const Bit256 = "012345678912345.012345678912345."
const Bit512 = "012345678912345.012345678912345." + "012345678912345.012345678912345."
const Bit1024 = Bit512 + Bit512

func TestMd5(t *testing.T) {
	//1bdf247646854ad6d841ba6b0cd376fe
	var plaintext = "dev"
	var salt = "1719910138"
	log.Println(cryptox.Md5v(plaintext, salt))
}

func TestSha256(t *testing.T) {
	var plaintext = Bit512 + Bit512
	log.Println(cryptox.Sha256v(plaintext, "123"))
}

func TestBcrypt(t *testing.T) {
	var plaintext = Bit512 + Bit512
	var ciphertext = ""
	//$2a$10$D3LbggNCcxz95XOr5CkLdeDadDc22xoSISMSADvM3p2BCmO49x1Yu
	//$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy
	//\__/\/ \____________________/\_____________________________/
	//Alg Cost      Salt 128bits               Hash 192bits
	// bcrypt 上限 72 字节，这里必须用合法密码
	plaintext = "correct-horse-battery-staple"

	ciphertext, err := cryptox.BcryptHash(plaintext)
	if err != nil {
		t.Fatalf("BcryptHash: %v", err)
	}
	if ciphertext == "" {
		t.Fatal("BcryptHash 返回了空哈希")
	}
	if !cryptox.BcryptCheck(plaintext, ciphertext) {
		t.Error("BcryptCheck(正确密码) 应为 true")
	}
	if cryptox.BcryptCheck("wrong-password", ciphertext) {
		t.Error("BcryptCheck(错误密码) 应为 false")
	}

	// 超过 72 字节必须报错：旧实现在此处静默返回空哈希，写库后该用户永远无法登录
	if _, err := cryptox.BcryptHash(strings.Repeat("a", 100)); err == nil {
		t.Error("超过 72 字节的密码应返回错误")
	}

	//6.60s
	//for i := 0; i < 10; i++ {
	//	bcrypt.GenerateFromPassword([]byte(Bit256), bcrypt.DefaultCost)
	//}
}

func BenchmarkMd5v(b *testing.B) {
	var plaintext = Bit512 + Bit512
	for i := 0; i < b.N; i++ {
		// 494.4 ns/op
		cryptox.Md5v(plaintext, "123")
	}
}

func BenchmarkSha256v(b *testing.B) {
	var plaintext = Bit512 + Bit512
	for i := 0; i < b.N; i++ {
		// 249.4 ns/op
		cryptox.Sha256v(plaintext, "123")
	}
}

func BenchmarkBcrypt(b *testing.B) {
	var plaintext = Bit512 + Bit512
	for i := 0; i < b.N; i++ {
		// cost=4:1075624 ns/op    cost=10:65876130 ns/op
		bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	}
}
