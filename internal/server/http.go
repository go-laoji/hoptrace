package server

import (
	nethttp "net/http"
	"os"
	"path/filepath"
	"strings"

	v1 "hoptrace/api/hoptrace/v1"
	"hoptrace/internal/biz"
	"hoptrace/internal/conf"
	"hoptrace/internal/service"

	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/transport/http"
	"github.com/gorilla/handlers"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(
	c *conf.Server,
	authUC *biz.AuthUsecase,
	bus *biz.EventBus,
	dev *service.DeviceService,
	cred *service.CredentialService,
	disc *service.DiscoveryService,
	topo *service.TopologyService,
	ep *service.EndpointService,
	dash *service.DashboardService,
	auth *service.AuthService,
	ctrl *service.ControlService,
) *http.Server {
	opts := []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			AuthMiddleware(authUC),
		),
		http.Filter(handlers.CORS(
			handlers.AllowedOrigins([]string{"*"}),
			handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
			handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
		)),
	}
	if c.GetHttp().GetNetwork() != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.GetHttp().GetAddr() != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.GetHttp().GetTimeout() != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)
	v1.RegisterDeviceServiceHTTPServer(srv, dev)
	v1.RegisterCredentialServiceHTTPServer(srv, cred)
	v1.RegisterDiscoveryServiceHTTPServer(srv, disc)
	v1.RegisterTopologyServiceHTTPServer(srv, topo)
	v1.RegisterEndpointServiceHTTPServer(srv, ep)
	v1.RegisterDashboardServiceHTTPServer(srv, dash)
	v1.RegisterAuthServiceHTTPServer(srv, auth)
	v1.RegisterControlServiceHTTPServer(srv, ctrl)

	// SSE 实时推送(拓扑/Trap 增量变更)
	srv.Handle("/api/v1/events/stream", sseHandler(bus, authUC))

	// 若配置了前端构建目录(如 web/dist),由后端直接托管 SPA
	if dir := strings.TrimSpace(c.GetStaticDir()); dir != "" {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			srv.HandlePrefix("/", spaHandler(dir))
		}
	}
	return srv
}

func spaHandler(dir string) nethttp.Handler {
	fs := nethttp.FileServer(nethttp.Dir(dir))
	index := filepath.Join(dir, "index.html")
	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			nethttp.NotFound(w, r)
			return
		}
		target := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		nethttp.ServeFile(w, r, index)
	})
}
