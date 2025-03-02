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

func init() {
	bean.Autowire(&component)
}

var component struct {
	Server *fiber.App `autowired:"true"`
}

var wsConnections = make(map[string]*WSConn)

func Shutdown(ctx context.Context) error {
	log.Logger().Info(ctx, "shutting down websocket server")
	for _, conn := range wsConnections {
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "server shutdown"),
			time.Now().Add(3*time.Second))
		conn.close()
	}
	return nil
}

func Start(ctx context.Context) error {
	path := config.Websocket.BasePath
	if config.Websocket.ContextPath != "" {
		path = path + config.Websocket.ContextPath
	}

	for key, value := range endpointAndHandlersMap {
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

		instance := shallowCopy(endpointAndHandlers.endpoint)
		c.SetCloseHandler(instance.OnClose)
		c.SetPingHandler(instance.OnPing)
		c.SetPongHandler(instance.OnPong)
		conn := &WSConn{conn: c, ctx: context.Background(), id: generateId(c), instance: instance}

		wsConnections[conn.Id()] = conn
		defer delete(wsConnections, conn.Id())

		if err := conn.onOpen(); err != nil {
			return
		}

		counter := startPingPong(conn)
		defer counter.Stop()

		conn.startListen()

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

func startPingPong(wsConn *WSConn) *counter.Counter {
	duration := config.Websocket.PingIntervalSecond
	if duration == 0 {
		duration = 30
	}
	ticker := counter.NewCounter(time.Second * time.Duration(duration))
	future.RunAsync(func() error {
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
	return ticker
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
			log.Logger().Error(ctx.UserContext(), "WebsocketHandler panic", "error", r)
		}
	}()
	if !websocket.IsWebSocketUpgrade(ctx) {
		return fiber.ErrUpgradeRequired
	}
	ctx.Locals("headers", ctx.GetReqHeaders())
	return ctx.Next()
}
