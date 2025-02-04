package websocket

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/dennesshen/photon-core-starter/bean"
	"github.com/dennesshen/photon-core-starter/log"
	"github.com/dennesshen/photon-core-starter/utils/counter"
	"github.com/dennesshen/photon-core-starter/utils/future"
	"reflect"
	"time"

	"github.com/google/uuid"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

type pair struct {
	wsConn   *WSConn
	endpoint *SocketEndpoint
}

func init() {
	bean.Autowire(&component)
}

var component struct {
	Server *fiber.App `autowired:"true"`
}

var wsConnections = make(map[string]pair)

func Shutdown(ctx context.Context) error {
	log.Logger().Info(ctx, "shutting down websocket server")
	for _, pair := range wsConnections {
		(*pair.endpoint).OnClose(websocket.CloseAbnormalClosure, "server shutdown")
		_ = pair.wsConn.Close()
	}
	return nil
}

func Start(ctx context.Context) error {
	path := config.Websocket.BasePath
	if config.Websocket.ContextPath != "" {
		path = path + config.Websocket.ContextPath
	}

	for key, value := range enpointAndHandlersMap {
		endpointPath := path + key
		log.Logger().Info(ctx, "Registering websocket endpoint: "+endpointPath)

		handlers := make([]fiber.Handler, len(value.middlewares)+2)
		handlers[0] = SocketUpgradeHandler
		copy(handlers[1:], value.middlewares)
		handlers[len(handlers)-1] = getEndpointHandler(value)
		component.Server.Get(endpointPath, handlers...)
	}
	return nil
}

func getEndpointHandler(endpointAndHandlers EndpointAndHandlers) fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		userCtx := getContext(c)
		instance := shallowCopy(endpointAndHandlers.endpoint)
		conn := &WSConn{Conn: c, ctx: userCtx, id: generateId(c)}
		wsConnections[conn.Id()] = pair{wsConn: conn, endpoint: &instance}
		conn.SetCloseHandler(instance.OnClose)
		conn.SetPingHandler(instance.OnPing)
		conn.SetPongHandler(instance.OnPong)

		if err := instance.OnOpen(conn); err != nil {
			closeProcessWithErr(conn, &instance, err)
			return
		}
		startPingPong(conn)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					closeProcess(conn, &instance)
					return
				}
				closeProcessWithErr(conn, &instance, err)
				return
			}
			if err = instance.OnMessage(conn, string(msg)); err != nil {
				closeProcessWithErr(conn, &instance, err)
				return
			}
		}
	})
}
func generateId(c *websocket.Conn) string {
	uuid := uuid.New()
	headers, _ := c.Locals("headers").(map[string][]string)
	ip := c.RemoteAddr().String()
	timestamp := time.Now().UnixNano()
	key := fmt.Sprintf("%s%s%d%v", uuid.String(), ip, timestamp, headers)
	sum256 := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum256[:])
}

func getContext(c *websocket.Conn) context.Context {
	var userCtx context.Context
	if fiberCtx, ok := c.Locals("user_context").(context.Context); ok {
		userCtx = fiberCtx
	} else {
		panic("user_context not found")
	}
	return userCtx
}

func closeProcessWithErr(conn *WSConn, s *SocketEndpoint, err error) {
	_ = conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseUnsupportedData, err.Error()),
		time.Now().Add(3*time.Second))
	(*s).OnError(conn, err)
	if err := (*s).OnClose(websocket.CloseUnsupportedData, err.Error()); err != nil {
		log.Logger().Error(conn.Ctx(), "Close websocket error: "+err.Error())
	}
	conn.Close()
	delete(wsConnections, conn.Id())
}

func closeProcess(conn *WSConn, s *SocketEndpoint) {
	if err := (*s).OnClose(websocket.CloseNormalClosure, ""); err != nil {
		log.Logger().Error(conn.Ctx(), "Close websocket error: "+err.Error())
	}
	conn.Close()
	delete(wsConnections, conn.Id())
}

func startPingPong(wsConn *WSConn) {
	future.RunAsync(func() error {
		duration := config.Websocket.PingIntervalSecond
		if duration == 0 {
			duration = 30
		}
		ticker := counter.NewCounter(time.Second * time.Duration(duration))
		for {
			select {
			case <-ticker.Touch():
				err := wsConn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(3*time.Second))
				if err != nil {
					return err
				}
			case <-ticker.IsStop():
				return nil
			}
		}
	})
}

func shallowCopy[T any](t T) T {
	tt := reflect.TypeOf(t)
	if tt.Kind() != reflect.Ptr {
		return t
	}
	tt = tt.Elem()
	nt := reflect.New(tt)
	for i := 0; i < tt.NumField(); i++ {
		if !nt.Elem().Field(i).CanSet() {
			continue
		}
		nt.Elem().Field(i).Set(reflect.ValueOf(t).Elem().Field(i))
	}
	return nt.Interface().(T)
}

func SocketUpgradeHandler(ctx *fiber.Ctx) error {
	defer func() {
		if r := recover(); r != nil {
			log.Logger().Error(ctx.UserContext(), "SocketUpgradeHandler panic", "error", r)
		}
	}()
	if !websocket.IsWebSocketUpgrade(ctx) {
		return fiber.ErrUpgradeRequired
	}
	ctx.Locals("headers", ctx.GetReqHeaders())
	ctx.Locals("user_context", ctx.UserContext())
	return ctx.Next()
}
