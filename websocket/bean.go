package websocket

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"reflect"
	"time"

	"github.com/dennesshen/photon-core-starter/log"
	"github.com/gofiber/contrib/websocket"
)

type WSConn struct {
	instance SocketEndpoint
	conn     *websocket.Conn
	ctx      context.Context
	id       string
	isClosed bool
}

func (w *WSConn) onOpen() error {
	if err := w.instance.OnOpen(w); err != nil {
		w.instance.OnError(w, err)
		_ = w.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseUnsupportedData, err.Error()),
			time.Now().Add(3*time.Second))
		_ = w.instance.OnClose(websocket.CloseUnsupportedData, err.Error())
		w.close()
		return err
	}
	return nil
}

func (w *WSConn) startListen() {
	defer w.close()
	for {
		_, msg, err := w.conn.ReadMessage()
		if err != nil {
			return
		}
		if err = w.instance.OnMessage(w, string(msg)); err != nil {
			w.instance.OnError(w, err)
			_ = w.conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseUnsupportedData, err.Error()),
				time.Now().Add(3*time.Second))
			_ = w.instance.OnClose(websocket.CloseUnsupportedData, err.Error())
			return
		}
	}
}

func (w *WSConn) close() {
	w.isClosed = true
	if err := w.conn.Close(); err != nil {
		log.Logger().Error(w.ctx, "close websocket connection error", "error", err)
	}
}

func (w *WSConn) Interrupt(err error) {
	if w.isClosed {
		return
	}
	defer w.close()
	if err != nil {
		w.instance.OnError(w, err)
		_ = w.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, err.Error()),
			time.Now().Add(3*time.Second))
		return
	}
	_ = w.conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(3*time.Second))
}

func (w *WSConn) WriteControl(messageType int, message []byte, deadline time.Time) error {
	if w.isClosed {
		return nil
	}
	return w.conn.WriteControl(messageType, message, deadline)
}

func (w *WSConn) WriteMessage(messageType int, data []byte) error {
	if w.isClosed {
		return nil
	}
	return w.conn.WriteMessage(messageType, data)
}

func (w *WSConn) WriteJSON(v interface{}) error {
	if w.isClosed {
		return nil
	}
	return w.conn.WriteJSON(v)
}

func (w *WSConn) Ctx() context.Context {
	return w.ctx
}

func (w *WSConn) Id() string {
	return w.id
}

type EndpointAndHandlers struct {
	endpoint    SocketEndpoint
	middlewares []fiber.Handler
}

var endpointAndHandlersMap = make(map[string]EndpointAndHandlers)

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
	if _, ok := endpointAndHandlersMap[endpointPath]; ok {
		panic("endpoint path repeat: " + endpointPath)
	}
	endpointAndHandlersMap[endpointPath] = EndpointAndHandlers{
		endpoint:    endpoint,
		middlewares: middlewares,
	}
}
