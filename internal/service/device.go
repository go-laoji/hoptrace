package service

import (
	"context"

	v1 "hoptrace/api/hoptrace/v1"
	"hoptrace/internal/biz"

	"google.golang.org/protobuf/types/known/emptypb"
)

type DeviceService struct {
	v1.UnimplementedDeviceServiceServer
	uc *biz.DeviceUsecase
}

func NewDeviceService(uc *biz.DeviceUsecase) *DeviceService {
	return &DeviceService{uc: uc}
}

func (s *DeviceService) ListDevices(ctx context.Context, req *v1.ListDevicesRequest) (*v1.ListDevicesReply, error) {
	items, total, err := s.uc.List(ctx, biz.DeviceFilter{
		Keyword: req.GetKeyword(),
		Status:  req.GetStatus(),
		Family:  req.GetFamily(),
		Role:    req.GetRole(),
	}, int(req.GetPage()), int(req.GetPageSize()))
	if err != nil {
		return nil, err
	}
	reply := &v1.ListDevicesReply{Total: total, Items: make([]*v1.Device, 0, len(items))}
	for _, d := range items {
		reply.Items = append(reply.Items, toProtoDevice(d))
	}
	return reply, nil
}

func (s *DeviceService) GetDevice(ctx context.Context, req *v1.GetDeviceRequest) (*v1.Device, error) {
	d, err := s.uc.Get(ctx, uint(req.GetId()))
	if err != nil {
		return nil, err
	}
	return toProtoDevice(d), nil
}

func (s *DeviceService) CreateDevice(ctx context.Context, req *v1.CreateDeviceRequest) (*v1.Device, error) {
	d, err := s.uc.Create(ctx, &biz.Device{
		Name:         req.GetName(),
		MgmtIP:       req.GetMgmtIp(),
		SNMPPort:     int(req.GetSnmpPort()),
		CredentialID: uint(req.GetCredentialId()),
		Site:         req.GetSite(),
		Location:     req.GetLocation(),
	})
	if err != nil {
		return nil, err
	}
	return toProtoDevice(d), nil
}

func (s *DeviceService) UpdateDevice(ctx context.Context, req *v1.UpdateDeviceRequest) (*v1.Device, error) {
	d, err := s.uc.Update(ctx, uint(req.GetId()), req.GetName(), uint(req.GetCredentialId()),
		req.GetRole(), req.GetSite(), req.GetLocation())
	if err != nil {
		return nil, err
	}
	return toProtoDevice(d), nil
}

func (s *DeviceService) DeleteDevice(ctx context.Context, req *v1.DeleteDeviceRequest) (*emptypb.Empty, error) {
	if err := s.uc.Delete(ctx, uint(req.GetId())); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *DeviceService) PollDevice(ctx context.Context, req *v1.PollDeviceRequest) (*v1.Device, error) {
	d, err := s.uc.Poll(ctx, uint(req.GetId()))
	if err != nil {
		return nil, err
	}
	return toProtoDevice(d), nil
}

func (s *DeviceService) ListInterfaces(ctx context.Context, req *v1.ListInterfacesRequest) (*v1.ListInterfacesReply, error) {
	items, err := s.uc.Interfaces(ctx, uint(req.GetDeviceId()))
	if err != nil {
		return nil, err
	}
	reply := &v1.ListInterfacesReply{Items: make([]*v1.Interface, 0, len(items))}
	for _, it := range items {
		reply.Items = append(reply.Items, &v1.Interface{
			Id:          int64(it.ID),
			DeviceId:    int64(it.DeviceID),
			IfIndex:     int32(it.IfIndex),
			Name:        it.Name,
			Descr:       it.Descr,
			Alias:       it.Alias,
			SpeedMbps:   it.SpeedMbps,
			AdminStatus: it.AdminStatus,
			OperStatus:  it.OperStatus,
			Mac:         it.MAC,
			Kind:        it.Kind,
			AggIfIndex:  int32(it.AggIfIndex),
			MacCount:    int32(it.MACCount),
			Neighbor:    it.Neighbor,
		})
	}
	return reply, nil
}

func toProtoDevice(d *biz.Device) *v1.Device {
	var lastPoll int64
	if d.LastPollAt != nil {
		lastPoll = d.LastPollAt.Unix()
	}
	return &v1.Device{
		Id:             int64(d.ID),
		Name:           d.Name,
		MgmtIp:         d.MgmtIP,
		SnmpPort:       int32(d.SNMPPort),
		CredentialId:   int64(d.CredentialID),
		Managed:        d.Managed,
		SysName:        d.SysName,
		SysDescr:       d.SysDescr,
		SysOid:         d.SysOID,
		Vendor:         d.Vendor,
		Family:         d.Family,
		Model:          d.Model,
		OsVersion:      d.OSVersion,
		Serial:         d.Serial,
		Role:           d.Role,
		RoleManual:     d.RoleManual,
		Status:         d.Status,
		Site:           d.Site,
		Location:       d.Location,
		Ips:            d.IPs,
		ChassisMac:     d.ChassisMAC,
		CpuUsage:       int32(d.CPUUsage),
		MemUsage:       int32(d.MemUsage),
		Temperature:    int32(d.Temperature),
		StackMembers:   int32(d.StackMembers),
		UptimeSeconds:  d.UptimeSecs,
		LastPollAt:     lastPoll,
		LastError:      d.LastError,
		CreatedAt:      d.CreatedAt.Unix(),
		InterfaceCount: int32(d.InterfaceCount),
		EndpointCount:  int32(d.EndpointCount),
	}
}
