package emailx

import "testing"

// 测试 Option 装配：各 With* 落到对应字段，后传入的覆盖先传入的
func TestNewEmailDeliverAppliesOptions(t *testing.T) {
	deliver := NewEmailDeliver(&EmailConfig{},
		WithHost("smtp.example.com"),
		WithPort(465),
		WithUsername("sender@example.com"),
		WithPassword("secret"),
		WithNickname("发件人"),
		WithDeliver([]string{"cc@example.com"}),
		WithSSL(true),
	)

	cases := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"Host", deliver.Host, "smtp.example.com"},
		{"Port", deliver.Port, 465},
		{"Username", deliver.Username, "sender@example.com"},
		{"Password", deliver.Password, "secret"},
		{"Nickname", deliver.Nickname, "发件人"},
		{"SSL", deliver.SSL, true},
	}

	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}

	if len(deliver.CC) != 1 || deliver.CC[0] != "cc@example.com" {
		t.Errorf("CC = %v, want [cc@example.com]", deliver.CC)
	}

	overridden := NewEmailDeliver(&EmailConfig{}, WithHost("first"), WithHost("second"))
	if overridden.Host != "second" {
		t.Errorf("Host = %q, want %q（后传入的 Option 应覆盖先传入的）", overridden.Host, "second")
	}
}
