package handlers

import (
	"testing"

	"callcenter-service/internal/gateway/pbxcontrol"
)

func TestInterfaceIsExtensionMatchesRealPBXShapes(t *testing.T) {
	cases := []struct {
		iface, name, ext string
		want             bool
	}{
		{"5955@from-internal", "Local/5955@from-internal", "5955", true},
		{"5959@from-internal", "Local/5959@from-internal", "5955", false},
		{"", "SIP/5955", "5955", true},
		{"PJSIP/5955", "", "5955", true},
		{"59550@from-internal", "Local/59550@from-internal", "5955", false},
		{"40010", "40010", "40010", true},
	}
	for _, c := range cases {
		got := interfaceIsExtension(pbxMember(c.iface, c.name), c.ext)
		if got != c.want {
			t.Errorf("iface=%q name=%q ext=%q: got %v want %v", c.iface, c.name, c.ext, got, c.want)
		}
	}
}

func pbxMember(iface, name string) pbxcontrol.QueueMember {
	return pbxcontrol.QueueMember{Interface: iface, Name: name}
}
