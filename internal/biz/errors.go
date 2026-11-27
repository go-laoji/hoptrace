package biz

import (
	v1 "hoptrace/api/hoptrace/v1"

	"github.com/go-kratos/kratos/v3/errors"
)

var (
	ErrDeviceNotFound         = errors.NotFound(v1.ErrorReason_DEVICE_NOT_FOUND.String(), "device not found")
	ErrDeviceInvalidArgument  = errors.BadRequest(v1.ErrorReason_DEVICE_INVALID_ARGUMENT.String(), "invalid device argument")
	ErrDeviceAlreadyExists    = errors.Conflict(v1.ErrorReason_DEVICE_ALREADY_EXISTS.String(), "device already exists")
	ErrCredentialNotFound     = errors.NotFound(v1.ErrorReason_CREDENTIAL_NOT_FOUND.String(), "credential not found")
	ErrCredentialInvalid      = errors.BadRequest(v1.ErrorReason_CREDENTIAL_INVALID_ARGUMENT.String(), "invalid credential argument")
	ErrCredentialInUse        = errors.Conflict(v1.ErrorReason_CREDENTIAL_IN_USE.String(), "credential is in use by devices")
	ErrEndpointNotFound       = errors.NotFound(v1.ErrorReason_ENDPOINT_NOT_FOUND.String(), "endpoint not found")
	ErrEndpointInvalid        = errors.BadRequest(v1.ErrorReason_ENDPOINT_INVALID_ARGUMENT.String(), "invalid endpoint argument")
	ErrDiscoveryNotFound      = errors.NotFound(v1.ErrorReason_DISCOVERY_NOT_FOUND.String(), "discovery job not found")
	ErrDiscoveryInvalid       = errors.BadRequest(v1.ErrorReason_DISCOVERY_INVALID_ARGUMENT.String(), "invalid discovery argument")
)

// ErrCollectFailed 采集失败(设备不可达、凭据错误等)。
func ErrCollectFailed(msg string) error {
	return errors.New(502, v1.ErrorReason_COLLECT_FAILED.String(), msg)
}
