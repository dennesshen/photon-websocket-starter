package websocket

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"reflect"

	"github.com/gofiber/contrib/websocket"
)

type EndpointAndHandlers struct {
	endpoint    SocketEndpoint
	middlewares []fiber.Handler
}

type WSConn struct {
	*websocket.Conn
	ctx context.Context
	id  string
}

func (w *WSConn) Ctx() context.Context {
	return w.ctx
}

func (w *WSConn) Id() string {
	return w.id
}

var enpointAndHandlersMap = make(map[string]EndpointAndHandlers)

type SocketEndpoint interface {
	OnOpen(conn *WSConn) error
	OnClose(closeType int, message string) error
	OnMessage(*WSConn, string) error
	OnError(*WSConn, error)
	OnPing(message string) error
	OnPong(message string) error
	GetEndpointPath() string
}

func RegisterSocketEndpoint(endpoint SocketEndpoint, middlewares ...func(*fiber.Ctx) error) {
	if reflect.ValueOf(endpoint).Kind() != reflect.Ptr {
		panic("websocket endpoint must be a pointer")
	}
	endpointPath := endpoint.GetEndpointPath()
	if _, ok := enpointAndHandlersMap[endpointPath]; ok {
		panic("endpoint path repeat: " + endpointPath)
	}
	enpointAndHandlersMap[endpointPath] = EndpointAndHandlers{
		endpoint:    endpoint,
		middlewares: middlewares,
	}
}
