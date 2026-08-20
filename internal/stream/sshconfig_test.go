package stream

import (
	"os"
	"reflect"
	"testing"
)

func TestParseSSHConfigHosts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgDir := home + "/.ssh"
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := `# comment
Host web1
  HostName 10.0.0.1
  User deploy

Host db-*
  HostName 10.0.0.2

Host web2 jumpbox
  ProxyJump bastion

Include ~/.ssh/work.conf
`
	if err := os.WriteFile(cfgDir+"/config", []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	got := ParseSSHConfigHosts()
	want := []string{"web1", "web2", "jumpbox"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseSSHConfigHosts() = %v, want %v", got, want)
	}
}

func TestMergeSSHHosts(t *testing.T) {
	got := MergeSSHHosts(
		[]string{"fav", "web1", ""},
		[]string{"web1", "web2"},
	)
	want := []string{"fav", "web1", "web2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MergeSSHHosts() = %v, want %v", got, want)
	}
}
