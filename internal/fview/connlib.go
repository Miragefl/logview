// 连接库聚合：frp.json（Servers/Conns）与 rules.yaml ssh_hosts 的共享正本访问。
// 密码永远不进连接库（会话态经 SSHSource.SetPassword 直传，Review Focus 3）。
package fview

// UpdateSSHHosts 写回共享 rules.yaml 的 ssh_hosts 键（SSH 主机列表，TUI 同源可见）。
func UpdateSSHHosts(path string, hosts []string) error {
	return updateRulesKey(path, "ssh_hosts", hosts)
}
