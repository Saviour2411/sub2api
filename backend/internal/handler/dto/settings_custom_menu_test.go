package dto

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCustomMenuIdentityForwarding(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		passUserID    bool
		passAuthToken bool
	}{
		{name: "旧配置默认关闭", raw: `[{"id":"docs","url":"https://example.com","visibility":"user"}]`},
		{name: "显式关闭", raw: `[{"pass_user_id":false,"pass_auth_token":false}]`},
		{name: "仅用户ID", raw: `[{"pass_user_id":true}]`, passUserID: true},
		{name: "仅登录Token", raw: `[{"pass_auth_token":true}]`, passAuthToken: true},
		{name: "全部开启", raw: `[{"pass_user_id":true,"pass_auth_token":true}]`, passUserID: true, passAuthToken: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := ParseCustomMenuItems(tt.raw)
			if len(items) != 1 {
				t.Fatalf("菜单数量错误：%d", len(items))
			}
			if items[0].PassUserID != tt.passUserID || items[0].PassAuthToken != tt.passAuthToken {
				t.Fatalf("身份透传开关错误：%+v", items[0])
			}
			raw, err := json.Marshal(items)
			if err != nil {
				t.Fatal(err)
			}
			if got := ParseCustomMenuItems(string(raw)); !reflect.DeepEqual(got, items) {
				t.Fatalf("JSON 往返丢失配置：%s", raw)
			}
		})
	}
}

func TestUserVisibleMenuItemsPreserveIdentityForwarding(t *testing.T) {
	raw := `[
		{"id":"legacy","visibility":"user"},
		{"id":"user-id","visibility":"user","pass_user_id":true},
		{"id":"token","visibility":"user","pass_auth_token":true,"hide_open_button":true},
		{"id":"admin","visibility":"admin","pass_user_id":true,"pass_auth_token":true}
	]`
	items := ParseUserVisibleMenuItems(raw)
	if len(items) != 3 {
		t.Fatalf("用户可见菜单数量错误：%d", len(items))
	}
	if items[0].ID != "legacy" || items[0].PassUserID || items[0].PassAuthToken {
		t.Fatalf("旧菜单未默认关闭：%+v", items[0])
	}
	if items[1].ID != "user-id" || !items[1].PassUserID || items[1].PassAuthToken {
		t.Fatalf("用户 ID 开关丢失或串用：%+v", items[1])
	}
	if items[2].ID != "token" || items[2].PassUserID || !items[2].PassAuthToken || !items[2].HideOpenButton {
		t.Fatalf("登录 Token 或隐藏按钮配置丢失：%+v", items[2])
	}
}
