package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hoptrace/internal/biz"
	"hoptrace/internal/conf"

	"golang.org/x/crypto/ssh"
)

type controlExecutor struct {
	simulate bool
	client   *http.Client
}

// NewControlExecutor 根据 collector.simulate 返回模拟执行器或真实 NETCONF/SSH 执行器。
func NewControlExecutor(c *conf.Collector) biz.ControlExecutor {
	return &controlExecutor{
		simulate: c.GetSimulate(),
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (e *controlExecutor) Execute(ctx context.Context, dev *biz.Device, cred *biz.WriteCredential, payload string, protocol string) (string, error) {
	if e.simulate || protocol == biz.ProtoSim {
		return fmt.Sprintf("[SIM %s -> %s (%s)] executed OK\n%s", protocol, dev.Name, dev.MgmtIP, payload), nil
	}
	if cred == nil {
		return "", fmt.Errorf("no write credential configured for protocol %s", protocol)
	}
	addr := fmt.Sprintf("%s:%d", dev.MgmtIP, cred.Port)
	sshCfg := &ssh.ClientConfig{
		User:            cred.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(cred.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 生产建议配置 known_hosts
		Timeout:         10 * time.Second,
	}

	conn, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return "", fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	defer conn.Close()

	sess, err := conn.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	if protocol == biz.ProtoNetconf {
		return execNetconf(sess, payload)
	}
	return execSSHCLI(sess, dev.Vendor, payload)
}

// execNetconf 通过 RFC 6242 NETCONF-over-SSH subsystem 下发 <rpc> 载荷。
func execNetconf(sess *ssh.Session, rpcBody string) (string, error) {
	if err := sess.RequestSubsystem("netconf"); err != nil {
		return "", fmt.Errorf("request netconf subsystem: %w", err)
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		return "", err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return "", err
	}
	const delim = "]]>]]>"
	hello := `<?xml version="1.0" encoding="UTF-8"?><hello xmlns="urn:ietf:params:xml:ns:netconf:base:1.0"><capabilities><capability>urn:ietf:params:netconf:base:1.0</capability></capabilities></hello>` + delim
	rpc := `<?xml version="1.0" encoding="UTF-8"?><rpc message-id="101" xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">` + rpcBody + `</rpc>` + delim
	closeRPC := `<?xml version="1.0" encoding="UTF-8"?><rpc message-id="102" xmlns="urn:ietf:params:xml:ns:netconf:base:1.0"><close-session/></rpc>` + delim

	if _, err := io.WriteString(stdin, hello+rpc+closeRPC); err != nil {
		return "", err
	}
	_ = stdin.Close()
	raw, _ := io.ReadAll(io.LimitReader(stdout, 64*1024))
	resp := string(raw)
	if strings.Contains(resp, "<rpc-error>") {
		return resp, fmt.Errorf("netconf rpc-error: %s", resp)
	}
	return resp, nil
}

// execSSHCLI 通过交互式命令行向不同厂商设备下发配置。
func execSSHCLI(sess *ssh.Session, vendor, commands string) (string, error) {
	var out bytes.Buffer
	sess.Stdout = &out
	sess.Stderr = &out
	stdin, err := sess.StdinPipe()
	if err != nil {
		return "", err
	}
	if err := sess.Shell(); err != nil {
		return "", err
	}
	switch strings.ToLower(strings.TrimSpace(vendor)) {
	case "ruijie", "cisco", "arista", "zte":
		_, _ = io.WriteString(stdin, "terminal length 0\n"+commands+"\nend\nexit\n")
	default:
		_, _ = io.WriteString(stdin, "screen-length 0 temporary\n"+commands+"\nreturn\nquit\n")
	}
	_ = stdin.Close()
	_ = sess.Wait()
	return out.String(), nil
}

// NotifyNAC 可选回调外部准入系统(iMaster NCE-Campus / RADIUS / 802.1X Webhook)。
func (e *controlExecutor) NotifyNAC(ctx context.Context, webhookURL string, act *biz.ControlAction, rollback bool) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"action":      act.Action,
		"rollback":    rollback,
		"device_id":   act.DeviceID,
		"device_name": act.DeviceName,
		"if_name":     act.IfName,
		"mac":         act.EndpointMAC,
		"param":       act.ParamValue,
		"operator":    act.ApprovedBy,
		"at":          time.Now().Unix(),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}
	return fmt.Sprintf("HTTP %d", resp.StatusCode), nil
}
