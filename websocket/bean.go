package websocket

import (
	"context"
	"reflect"

	"github.com/gofiber/contrib/websocket"
)

var socketEndPoints = make([]SocketEndpoint, 0)

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

type SocketEndpoint interface {
	OnOpen(conn *WSConn) error
	OnClose(closeType int, message string) error
	OnMessage(*WSConn, string) error
	OnError(*WSConn, error)
	OnPing(message string) error
	OnPong(message string) error
	GetEndpointPath() string
}

func RegisterSocketEndpoint(endpoint SocketEndpoint) {
	if reflect.ValueOf(endpoint).Kind() != reflect.Ptr {
		panic("websocket endpoint must be a pointer")
	}
	socketEndPoints = append(socketEndPoints, endpoint)
}
