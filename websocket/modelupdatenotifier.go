package websocket

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/looplab/eventhorizon"
	"go.uber.org/zap"
	"net/http"
	"sync"
	"walletaccountant/definitions"
	"walletaccountant/eventhandler"
)

var _ definitions.Route = &ModelUpdater{}

type ModelUpdated struct {
	Event eventhorizon.EventType
}

type ModelUpdateNotifier interface {
	UpdatedAggregate() eventhorizon.AggregateType
	UpdateChannel() chan ModelUpdated
}

type message struct {
	Subject string `json:"subject"`
	Event   string `json:"event"`
}

type modelUpdateNotifier struct {
	channel chan ModelUpdated
}

type ModelUpdater struct {
	notifiers map[eventhorizon.AggregateType]modelUpdateNotifier
	upgrader  *websocket.Upgrader
	wg        *sync.WaitGroup
	log       *zap.Logger
}

func NewModelUpdater(
	projectorRegistry *eventhandler.ProjectionRegistry,
	upgrader *websocket.Upgrader,
	log *zap.Logger,
) *ModelUpdater {
	log = log.With(zap.String("struct", "ModelUpdater"))

	log.Debug("registering notifier")

	notifierInstances := make(map[eventhorizon.AggregateType]modelUpdateNotifier)
	for _, handler := range projectorRegistry.GetHandlers() {
		notifier, ok := handler.EventHandler.(ModelUpdateNotifier)
		if !ok {
			continue
		}

		notifierInstances[notifier.UpdatedAggregate()] = modelUpdateNotifier{
			channel: notifier.UpdateChannel(),
		}

		log.Debug("notifier registered", zap.String("aggregate", notifier.UpdatedAggregate().String()))
	}

	return &ModelUpdater{
		notifiers: notifierInstances,
		upgrader:  upgrader,
		log:       log,
	}
}

func (m *ModelUpdater) Configuration() (string, string) {
	return http.MethodGet, "/ws"
}

func (m *ModelUpdater) Handle(ctx *gin.Context) {
	conn, err := m.upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		m.log.Error("failed to open websocket connection", zap.Error(err))

		return
	}

	m.log.Debug("new web socket connection")

	//conn.SetCloseHandler(func(code int, text string) error {
	//	m.log.Debug("close handler called", zap.Int("code", code), zap.String("text", text))
	//
	//	return conn.CloseHandler()(code, text)
	//})
	//
	//conn.SetPingHandler(func(appData string) error {
	//	m.log.Debug("ping handler called", zap.String("appData", appData))
	//
	//	return conn.PingHandler()(appData)
	//})
	//
	//conn.SetPongHandler(func(appData string) error {
	//	m.log.Debug("pong handler called", zap.String("appData", appData))
	//
	//	return conn.PongHandler()(appData)
	//})

	for aggregate, notifier := range m.notifiers {
		go m.handleUpdateNotification(ctx, conn, aggregate, notifier)
	}
}

func (m *ModelUpdater) handleUpdateNotification(
	ctx context.Context,
	conn *websocket.Conn,
	aggregate eventhorizon.AggregateType,
	notifier modelUpdateNotifier,
) {
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			m.log.Error("failed to close websocket connection", zap.Error(err))
		}
	}(conn)

	//go m.handleIncoming(conn)

	keepRunning := true
	for keepRunning {
		select {
		case <-ctx.Done():
			keepRunning = false

		case modelUpdated := <-notifier.channel:
			bytes, err := json.Marshal(
				message{
					Subject: aggregate.String(),
					Event:   modelUpdated.Event.String(),
				},
			)
			if err != nil {
				m.log.Error(
					"failed to marshal web socket message",
					zap.String("aggregate", aggregate.String()),
					zap.String("event", modelUpdated.Event.String()),
					zap.Error(err),
				)

				break
			}

			m.log.Debug(
				"notifying web socket",
				zap.String("aggregate", aggregate.String()),
				zap.String("event", modelUpdated.Event.String()),
			)

			err = conn.WriteMessage(websocket.TextMessage, bytes)
			if err != nil {
				m.log.Warn(
					"failed to notify web socket",
					zap.String("aggregate", aggregate.String()),
					zap.String("event", modelUpdated.Event.String()),
					zap.Error(err),
				)

				keepRunning = false
			}
			//
			//case <-time.After(5 * time.Second):
			//	if aggregate.String() != "tagCategory" {
			//		continue
			//	}
			//
			//	bytes, _ := json.Marshal(
			//		message{
			//			Subject: aggregate.String(),
			//			Event:   "dome",
			//		},
			//	)
			//
			//	m.log.Debug(
			//		"notifying web socket",
			//		zap.String("aggregate", aggregate.String()),
			//	)
			//	_ = conn.WriteMessage(websocket.TextMessage, bytes)
		}
	}
}

//func (m *ModelUpdater) handleIncoming(conn *websocket.Conn) {
//	for {
//		msgType, messageData, err := conn.ReadMessage()
//		if err != nil {
//			m.log.Error("incoming message error", zap.Error(err))
//
//			break
//		}
//
//		m.log.Debug(
//			"incoming message",
//			zap.Int("type", msgType),
//			zap.String("message", string(messageData)),
//		)
//
//		switch msgType {
//		case websocket.PingMessage:
//			err := conn.PingHandler()("pong")
//			if err != nil {
//				m.log.Error("ping handler error", zap.Error(err))
//			}
//		}
//	}
//}
