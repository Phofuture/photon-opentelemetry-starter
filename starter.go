package opentelStarter

import (
	"github.com/Phofuture/photon-core-starter/core"
	"github.com/Phofuture/photon-core-starter/log"
	"github.com/Phofuture/photon-opentelemetry-starter/opentelCore"
	"github.com/Phofuture/photon-opentelemetry-starter/opentelLog"
)

func init() {
	log.RegisterInitAction(opentelLog.Start)
	core.RegisterCoreDependency(opentelCore.Start)
	core.RegisterShutdownCoreDependency(opentelCore.Shutdown)
}
