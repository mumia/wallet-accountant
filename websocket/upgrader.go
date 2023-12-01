package websocket

import (
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"net/http"
)

//const frontendUrlName = "FRONTEND_URL"

func NewWebsocketUpgrader(log *zap.Logger) *websocket.Upgrader {
	log = log.With(zap.String("struct", "NewWebsocketUpgrader"))

	return &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     checkOrigin(log),
	}
}

func checkOrigin(log *zap.Logger) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		//if r.RemoteAddr == os.Getenv(frontendUrlName) {
		return true
		//}
		//
		//log.Error(
		//	"different origin on websocket",
		//	zap.String("request_host", r.Host),
		//	zap.String("allowed_host", os.Getenv(frontendUrlName)),
		//)
		//
		//return false
	}
}
