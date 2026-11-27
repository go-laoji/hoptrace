package biz

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	v1 "hoptrace/api/hoptrace/v1"

	"github.com/go-kratos/kratos/v3/errors"
)

// 管控动作类型
const (
	ActionShutdownPort     = "shutdown_port"
	ActionUndoShutdownPort = "undo_shutdown_port"
	ActionSetPortVLAN      = "set_port_vlan"
	ActionSetPortDesc      = "set_port_desc"
	ActionBlockEndpoint    = "block_endpoint"
	ActionUnblockEndpoint  = "unblock_endpoint"
)

// 管控工单状态
const (
	ControlPending    = "pending_approval"
	ControlApproved   = "approved"
	ControlRejected   = "rejected"
	ControlExecuted   = "executed"
	ControlFailed     = "failed"
	ControlRolledBack = "rolled_back"
)

// 写通道协议
const (
	ProtoNetconf = "netconf"
	ProtoSSH     = "ssh"
	ProtoSim     = "sim"
)

var (
	ErrControlNotFound        = errors.NotFound(v1.ErrorReason_CONTROL_NOT_FOUND.String(), "control action not found")
	ErrControlInvalidState    = errors.BadRequest(v1.ErrorReason_CONTROL_INVALID_STATE.String(), "invalid control action state or parameter")
	ErrControlProtectedTarget = errors.Forbidden(v1.ErrorReason_CONTROL_PROTECTED_TARGET.String(), "target is protected (core/firewall/uplink/lag)")
)

func ErrControlExecFailed(msg string) error {
	return errors.New(502, v1.ErrorReason_CONTROL_EXEC_FAILED.String(), msg)
}

// WriteCredential NETCONF/SSH 写通道凭据(与只读 SNMP 凭据隔离)。
type WriteCredential struct {
	ID            uint
	Name          string
	Protocol      string // netconf / ssh
	Username      string
	Password      string
	Port          int
	NACWebhookURL string // 可选: iMaster NCE / RADIUS 准入联动回调地址
	CreatedAt     time.Time
}

// ControlAction 管控工单 DO。
type ControlAction struct {
	ID              uint
	Action          string
	DeviceID        uint
	DeviceName      string
	IfIndex         int
	IfName          string
	EndpointID      uint
	EndpointMAC     string
	ParamValue      string
	PrevValue       string
	Protocol        string
	DryRunPreview   string
	RollbackPreview string
	Status          string
	Reason          string
	RequestedBy     string
	ApprovedBy      string
	ResultLog       string
	CreatedAt       time.Time
	ExecutedAt      *time.Time
	RolledBackAt    *time.Time
}

// ControlFilter 管控工单筛选。
type ControlFilter struct {
	Status   string
	DeviceID uint
}

// ControlRepo 管控工单与写凭据仓储。
type ControlRepo interface {
	ListActions(ctx context.Context, f ControlFilter, page, size int) ([]*ControlAction, int64, error)
	GetAction(ctx context.Context, id uint) (*ControlAction, error)
	CreateAction(ctx context.Context, a *ControlAction) (*ControlAction, error)
	UpdateAction(ctx context.Context, a *ControlAction) error
	ListWriteCredentials(ctx context.Context) ([]*WriteCredential, error)
	GetWriteCredential(ctx context.Context, id uint) (*WriteCredential, error)
	SaveWriteCredential(ctx context.Context, c *WriteCredential) (*WriteCredential, error)
	DeleteWriteCredential(ctx context.Context, id uint) error
}

// ControlExecutor 设备写通道执行器(NETCONF / SSH / Sim)及可选 NAC 联动回调。
type ControlExecutor interface {
	Execute(ctx context.Context, dev *Device, cred *WriteCredential, payload string, protocol string) (string, error)
	NotifyNAC(ctx context.Context, webhookURL string, action *ControlAction, rollback bool) (string, error)
}

// CreateControlInput 创建管控工单输入。
type CreateControlInput struct {
	Action      string
	DeviceID    uint
	IfIndex     int
	EndpointID  uint
	ParamValue  string
	Protocol    string
	Reason      string
	DryRunOnly  bool
}

// ControlUsecase 管控审批、执行与回滚编排。
type ControlUsecase struct {
	repo      ControlRepo
	devices   DeviceRepo
	endpoints EndpointRepo
	exec      ControlExecutor
}

func NewControlUsecase(repo ControlRepo, devices DeviceRepo, endpoints EndpointRepo, exec ControlExecutor) *ControlUsecase {
	return &ControlUsecase{repo: repo, devices: devices, endpoints: endpoints, exec: exec}
}

func (uc *ControlUsecase) ListActions(ctx context.Context, f ControlFilter, page, size int) ([]*ControlAction, int64, error) {
	return uc.repo.ListActions(ctx, f, page, size)
}

func (uc *ControlUsecase) ListWriteCredentials(ctx context.Context) ([]*WriteCredential, error) {
	return uc.repo.ListWriteCredentials(ctx)
}

func (uc *ControlUsecase) SaveWriteCredential(ctx context.Context, c *WriteCredential) (*WriteCredential, error) {
	c.Name = strings.TrimSpace(c.Name)
	c.Username = strings.TrimSpace(c.Username)
	if c.Name == "" || c.Username == "" {
		return nil, ErrControlInvalidState
	}
	if c.Protocol == "" {
		c.Protocol = ProtoNetconf
	}
	if c.Protocol != ProtoNetconf && c.Protocol != ProtoSSH {
		return nil, ErrControlInvalidState
	}
	if c.Port <= 0 {
		if c.Protocol == ProtoNetconf {
			c.Port = 830
		} else {
			c.Port = 22
		}
	}
	if c.ID != 0 {
		old, err := uc.repo.GetWriteCredential(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		if c.Password == "" {
			c.Password = old.Password
		}
		c.CreatedAt = old.CreatedAt
	} else if c.Password == "" {
		return nil, ErrControlInvalidState
	}
	return uc.repo.SaveWriteCredential(ctx, c)
}

func (uc *ControlUsecase) DeleteWriteCredential(ctx context.Context, id uint) error {
	return uc.repo.DeleteWriteCredential(ctx, id)
}

// CreateAction 校验保护名单、生成 dry-run 与回滚命令预览,并创建待审批工单(DryRunOnly=true 时不落库)。
func (uc *ControlUsecase) CreateAction(ctx context.Context, in CreateControlInput) (*ControlAction, error) {
	var ep *Endpoint
	if in.EndpointID != 0 {
		var err error
		ep, err = uc.endpoints.Get(ctx, in.EndpointID)
		if err != nil {
			return nil, err
		}
		if in.DeviceID == 0 {
			in.DeviceID = ep.DeviceID
		}
		if in.IfIndex == 0 {
			in.IfIndex = ep.IfIndex
		}
	}
	if in.DeviceID == 0 {
		return nil, ErrControlInvalidState
	}
	dev, err := uc.devices.Get(ctx, in.DeviceID)
	if err != nil {
		return nil, err
	}
	if !dev.Managed {
		return nil, ErrControlProtectedTarget
	}

	// 查找目标接口
	var iface *Interface
	if in.IfIndex != 0 {
		ifs, err := uc.devices.ListInterfaces(ctx, dev.ID)
		if err != nil {
			return nil, err
		}
		for _, it := range ifs {
			if it.IfIndex == in.IfIndex {
				iface = it
				break
			}
		}
		if iface == nil {
			return nil, ErrControlInvalidState
		}
	}

	// 保护名单校验: 核心/防火墙、上联口、聚合口/成员口严禁阻断/shutdown/改 VLAN
	if err := checkProtection(in.Action, dev, iface); err != nil {
		return nil, err
	}

	proto := strings.ToLower(strings.TrimSpace(in.Protocol))
	if proto == "" {
		proto = ProtoNetconf
	}
	if proto != ProtoNetconf && proto != ProtoSSH && proto != ProtoSim {
		return nil, ErrControlInvalidState
	}

	ifName := ""
	if iface != nil {
		ifName = iface.Name
	}
	epMAC := ""
	prevVal := ""
	if ep != nil {
		epMAC = ep.MAC
		prevVal = strconv.Itoa(ep.VLAN)
	}
	if in.Action == ActionSetPortDesc && iface != nil {
		prevVal = iface.Alias
	}

	preview, rollback, err := buildHuaweiCommand(in.Action, proto, ifName, epMAC, in.ParamValue, prevVal)
	if err != nil {
		return nil, err
	}

	requester := "local-admin"
	if c, ok := ClaimsFromContext(ctx); ok && c.Username != "" {
		requester = c.Username
	}
	act := &ControlAction{
		Action:          in.Action,
		DeviceID:        dev.ID,
		DeviceName:      dev.Name,
		IfIndex:         in.IfIndex,
		IfName:          ifName,
		EndpointID:      in.EndpointID,
		EndpointMAC:     epMAC,
		ParamValue:      strings.TrimSpace(in.ParamValue),
		PrevValue:       prevVal,
		Protocol:        proto,
		DryRunPreview:   preview,
		RollbackPreview: rollback,
		Status:          ControlPending,
		Reason:          strings.TrimSpace(in.Reason),
		RequestedBy:     requester,
		CreatedAt:       time.Now(),
	}
	if in.DryRunOnly {
		return act, nil
	}
	return uc.repo.CreateAction(ctx, act)
}

func (uc *ControlUsecase) Approve(ctx context.Context, id uint, autoExecute bool) (*ControlAction, error) {
	act, err := uc.repo.GetAction(ctx, id)
	if err != nil {
		return nil, err
	}
	if act.Status != ControlPending {
		return nil, ErrControlInvalidState
	}
	approver := "admin"
	if c, ok := ClaimsFromContext(ctx); ok && c.Username != "" {
		approver = c.Username
	}
	act.Status = ControlApproved
	act.ApprovedBy = approver
	if err := uc.repo.UpdateAction(ctx, act); err != nil {
		return nil, err
	}
	if autoExecute {
		return uc.Execute(ctx, id)
	}
	return act, nil
}

func (uc *ControlUsecase) Reject(ctx context.Context, id uint, reason string) (*ControlAction, error) {
	act, err := uc.repo.GetAction(ctx, id)
	if err != nil {
		return nil, err
	}
	if act.Status != ControlPending {
		return nil, ErrControlInvalidState
	}
	approver := "admin"
	if c, ok := ClaimsFromContext(ctx); ok && c.Username != "" {
		approver = c.Username
	}
	act.Status = ControlRejected
	act.ApprovedBy = approver
	if reason != "" {
		act.ResultLog = "rejected: " + reason
	}
	if err := uc.repo.UpdateAction(ctx, act); err != nil {
		return nil, err
	}
	return act, nil
}

func (uc *ControlUsecase) Execute(ctx context.Context, id uint) (*ControlAction, error) {
	act, err := uc.repo.GetAction(ctx, id)
	if err != nil {
		return nil, err
	}
	if act.Status != ControlApproved {
		return nil, ErrControlInvalidState
	}
	dev, err := uc.devices.Get(ctx, act.DeviceID)
	if err != nil {
		return nil, err
	}
	cred := uc.pickWriteCredential(ctx, act.Protocol)
	out, execErr := uc.exec.Execute(ctx, dev, cred, act.DryRunPreview, act.Protocol)
	now := time.Now()
	act.ExecutedAt = &now
	if execErr != nil {
		act.Status = ControlFailed
		act.ResultLog = execErr.Error()
		_ = uc.repo.UpdateAction(ctx, act)
		return act, ErrControlExecFailed(execErr.Error())
	}
	act.Status = ControlExecuted
	act.ResultLog = out

	// 联动终端信任状态与可选 NAC Webhook
	uc.syncEndpointAndNAC(ctx, act, cred, false)
	if err := uc.repo.UpdateAction(ctx, act); err != nil {
		return nil, err
	}
	return act, nil
}

func (uc *ControlUsecase) Rollback(ctx context.Context, id uint) (*ControlAction, error) {
	act, err := uc.repo.GetAction(ctx, id)
	if err != nil {
		return nil, err
	}
	if act.Status != ControlExecuted {
		return nil, ErrControlInvalidState
	}
	dev, err := uc.devices.Get(ctx, act.DeviceID)
	if err != nil {
		return nil, err
	}
	cred := uc.pickWriteCredential(ctx, act.Protocol)
	out, execErr := uc.exec.Execute(ctx, dev, cred, act.RollbackPreview, act.Protocol)
	if execErr != nil {
		act.ResultLog += "\n[rollback failed] " + execErr.Error()
		_ = uc.repo.UpdateAction(ctx, act)
		return act, ErrControlExecFailed(execErr.Error())
	}
	now := time.Now()
	act.Status = ControlRolledBack
	act.RolledBackAt = &now
	act.ResultLog += "\n[rollback ok] " + out

	uc.syncEndpointAndNAC(ctx, act, cred, true)
	if err := uc.repo.UpdateAction(ctx, act); err != nil {
		return nil, err
	}
	return act, nil
}

func (uc *ControlUsecase) pickWriteCredential(ctx context.Context, protocol string) *WriteCredential {
	creds, err := uc.repo.ListWriteCredentials(ctx)
	if err != nil || len(creds) == 0 {
		return nil
	}
	for _, c := range creds {
		if c.Protocol == protocol {
			return c
		}
	}
	return creds[0]
}

func (uc *ControlUsecase) syncEndpointAndNAC(ctx context.Context, act *ControlAction, cred *WriteCredential, rollback bool) {
	if act.EndpointID != 0 {
		if ep, err := uc.endpoints.Get(ctx, act.EndpointID); err == nil {
			switch act.Action {
			case ActionBlockEndpoint:
				if rollback {
					ep.Trust = TrustUnknown
				} else {
					ep.Trust = TrustBlocked
				}
				_ = uc.endpoints.Update(ctx, ep)
			case ActionUnblockEndpoint:
				if rollback {
					ep.Trust = TrustBlocked
				} else {
					ep.Trust = TrustTrusted
				}
				_ = uc.endpoints.Update(ctx, ep)
			}
		}
	}
	if cred != nil && strings.TrimSpace(cred.NACWebhookURL) != "" {
		if msg, err := uc.exec.NotifyNAC(ctx, cred.NACWebhookURL, act, rollback); err != nil {
			act.ResultLog += "\n[nac webhook warn] " + err.Error()
		} else if msg != "" {
			act.ResultLog += "\n[nac webhook] " + msg
		}
	}
}

// checkProtection 保护名单校验: 防止误操作核心/防火墙设备或上联/聚合端口。
func checkProtection(action string, dev *Device, iface *Interface) error {
	destructive := action == ActionShutdownPort || action == ActionSetPortVLAN || action == ActionBlockEndpoint
	if !destructive {
		return nil
	}
	if dev.Role == RoleCore || dev.Role == RoleFirewall || dev.Family == FamilyFirewall {
		return ErrControlProtectedTarget
	}
	if iface != nil {
		switch iface.Kind {
		case IfKindUplink, IfKindLAG, IfKindLAGMember:
			return ErrControlProtectedTarget
		}
	}
	return nil
}

// buildHuaweiCommand 生成华为 VRP CLI 或 NETCONF XML 预览及对称回滚载荷。
func buildHuaweiCommand(action, proto, ifName, epMAC, param, prev string) (preview, rollback string, err error) {
	hwMAC := formatHuaweiMAC(epMAC)
	switch action {
	case ActionShutdownPort:
		if ifName == "" {
			return "", "", ErrControlInvalidState
		}
		if proto == ProtoNetconf {
			return netconfIfAdmin(ifName, "down"), netconfIfAdmin(ifName, "up"), nil
		}
		return fmt.Sprintf("system-view\ninterface %s\n shutdown\nquit", ifName),
			fmt.Sprintf("system-view\ninterface %s\n undo shutdown\nquit", ifName), nil

	case ActionUndoShutdownPort:
		if ifName == "" {
			return "", "", ErrControlInvalidState
		}
		if proto == ProtoNetconf {
			return netconfIfAdmin(ifName, "up"), netconfIfAdmin(ifName, "down"), nil
		}
		return fmt.Sprintf("system-view\ninterface %s\n undo shutdown\nquit", ifName),
			fmt.Sprintf("system-view\ninterface %s\n shutdown\nquit", ifName), nil

	case ActionSetPortVLAN:
		vlan, e := strconv.Atoi(strings.TrimSpace(param))
		if ifName == "" || e != nil || vlan < 1 || vlan > 4094 {
			return "", "", ErrControlInvalidState
		}
		oldVLAN := prev
		if oldVLAN == "" || oldVLAN == "0" {
			oldVLAN = "1"
		}
		return fmt.Sprintf("system-view\ninterface %s\n port default vlan %d\nquit", ifName, vlan),
			fmt.Sprintf("system-view\ninterface %s\n port default vlan %s\nquit", ifName, oldVLAN), nil

	case ActionSetPortDesc:
		desc := strings.TrimSpace(param)
		if ifName == "" || desc == "" {
			return "", "", ErrControlInvalidState
		}
		rb := fmt.Sprintf("system-view\ninterface %s\n undo description\nquit", ifName)
		if prev != "" {
			rb = fmt.Sprintf("system-view\ninterface %s\n description %s\nquit", ifName, prev)
		}
		return fmt.Sprintf("system-view\ninterface %s\n description %s\nquit", ifName, desc), rb, nil

	case ActionBlockEndpoint:
		if hwMAC == "" {
			return "", "", ErrControlInvalidState
		}
		vlan := prev
		if strings.TrimSpace(param) != "" {
			vlan = strings.TrimSpace(param)
		}
		if vlan == "" || vlan == "0" {
			vlan = "1"
		}
		return fmt.Sprintf("system-view\nmac-address blackhole %s vlan %s\nquit", hwMAC, vlan),
			fmt.Sprintf("system-view\nundo mac-address blackhole %s vlan %s\nquit", hwMAC, vlan), nil

	case ActionUnblockEndpoint:
		if hwMAC == "" {
			return "", "", ErrControlInvalidState
		}
		vlan := prev
		if strings.TrimSpace(param) != "" {
			vlan = strings.TrimSpace(param)
		}
		if vlan == "" || vlan == "0" {
			vlan = "1"
		}
		return fmt.Sprintf("system-view\nundo mac-address blackhole %s vlan %s\nquit", hwMAC, vlan),
			fmt.Sprintf("system-view\nmac-address blackhole %s vlan %s\nquit", hwMAC, vlan), nil

	default:
		return "", "", ErrControlInvalidState
	}
}

func formatHuaweiMAC(mac string) string {
	m := NormalizeMAC(mac)
	if m == "" {
		return ""
	}
	h := strings.ReplaceAll(m, ":", "")
	return h[0:4] + "-" + h[4:8] + "-" + h[8:12]
}

func netconfIfAdmin(ifName, status string) string {
	return fmt.Sprintf(`<edit-config><target><running/></target><config><ifm xmlns="urn:huawei:yang:huawei-ifm"><interfaces><interface><name>%s</name><ifAdminStatus>%s</ifAdminStatus></interface></interfaces></ifm></config></edit-config>`, ifName, status)
}
