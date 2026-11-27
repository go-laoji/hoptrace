package server

import (
	"context"
	"strings"

	v1 "hoptrace/api/hoptrace/v1"
	"hoptrace/internal/biz"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
)

// AuthMiddleware 实现 JWT 鉴权与基于 Operation 的 RBAC 拦截。
func AuthMiddleware(auth *biz.AuthUsecase) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			if !auth.Enabled() {
				return handler(biz.WithClaims(ctx, &biz.AuthClaims{
					Username: "local-admin", DisplayName: "网络运维", Role: biz.UserRoleAdmin,
				}), req)
			}

			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return handler(ctx, req)
			}
			op := tr.Operation()
			if op == v1.OperationAuthServiceLogin {
				return handler(ctx, req)
			}

			rawAuth := tr.RequestHeader().Get("Authorization")
			var claims *biz.AuthClaims
			if strings.HasPrefix(rawAuth, "Bearer ") {
				if c, err := auth.ParseToken(strings.TrimPrefix(rawAuth, "Bearer ")); err == nil {
					claims = c
				}
			}

			// CurrentUser 允许未登录时探测 auth_enabled 状态
			if op == v1.OperationAuthServiceCurrentUser {
				if claims != nil {
					ctx = biz.WithClaims(ctx, claims)
				}
				return handler(ctx, req)
			}

			if claims == nil {
				return nil, biz.ErrUnauthorized
			}
			if !allowed(claims.Role, op) {
				return nil, biz.ErrForbidden
			}
			return handler(biz.WithClaims(ctx, claims), req)
		}
	}
}

func allowed(role, op string) bool {
	if role == biz.UserRoleAdmin {
		return true
	}
	// 用户管理、管控审批/执行/回滚与写凭据管理仅限 admin
	switch op {
	case v1.OperationAuthServiceListUsers,
		v1.OperationAuthServiceCreateUser,
		v1.OperationAuthServiceDeleteUser,
		v1.OperationControlServiceApproveControlAction,
		v1.OperationControlServiceRejectControlAction,
		v1.OperationControlServiceExecuteControlAction,
		v1.OperationControlServiceRollbackControlAction,
		v1.OperationControlServiceSaveWriteCredential,
		v1.OperationControlServiceDeleteWriteCredential:
		return false
	}
	if role == biz.UserRoleOperator {
		return true
	}
	// viewer 仅允许只读查询与修改自己密码
	switch op {
	case v1.OperationAuthServiceChangePassword,
		v1.OperationDashboardServiceGetDashboard,
		v1.OperationTopologyServiceGetTopology,
		v1.OperationDeviceServiceListDevices,
		v1.OperationDeviceServiceGetDevice,
		v1.OperationDeviceServiceListInterfaces,
		v1.OperationDeviceServiceListInterfaceMetrics,
		v1.OperationEndpointServiceListEndpoints,
		v1.OperationEndpointServiceGetEndpoint,
		v1.OperationEndpointServiceListEvents,
		v1.OperationDiscoveryServiceListJobs,
		v1.OperationDiscoveryServiceGetJob,
		v1.OperationCredentialServiceListCredentials,
		v1.OperationControlServiceListControlActions,
		v1.OperationControlServiceListWriteCredentials,
		v1.OperationTopologyServiceTracePath,
		v1.OperationAlertServiceListAlerts,
		v1.OperationAlertServiceListChannels,
		v1.OperationAlertServiceListSyslogs,
		v1.OperationSiteServiceListSites,
		v1.OperationSiteServiceGetSite,
		v1.OperationIpamServiceListSubnets,
		v1.OperationIpamServiceGetSubnetDetail,
		v1.OperationIpamServiceListAnomalies,
		v1.OperationBackupServiceListBackups,
		v1.OperationBackupServiceGetBackup,
		v1.OperationBackupServiceDiffBackups:
		return true
	default:
		return false
	}
}
