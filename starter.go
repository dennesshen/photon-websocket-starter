package websocketStarter

import (
	"github.com/dennesshen/photon-core-starter/core"
	"github.com/dennesshen/photon-websocket-starter/websocket"
)

func init() {
	core.RegisterCoreDependency(websocket.Start)
	core.RegisterShutdownCoreDependency(websocket.Shutdown)
}
