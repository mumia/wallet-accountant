package websocket

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"go.uber.org/fx"
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
	notifiers                map[eventhorizon.AggregateType]modelUpdateNotifier
	upgrader                 *websocket.Upgrader
	wg                       *sync.WaitGroup
	log                      *zap.Logger
	registeredListenersMutex sync.Mutex
	registeredListeners      map[uuid.UUID]chan message
}

func NewModelUpdater(
	projectorRegistry *eventhandler.ProjectionRegistry,
	upgrader *websocket.Upgrader,
	log *zap.Logger,
	lifecycle fx.Lifecycle,
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

	modelUpdater := &ModelUpdater{
		notifiers:           notifierInstances,
		upgrader:            upgrader,
		log:                 log,
		registeredListeners: make(map[uuid.UUID]chan message),
	}

	var lifecycleCtx context.Context
	var lifecycleCtxCancel context.CancelFunc
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			lifecycleCtx, lifecycleCtxCancel = context.WithCancel(context.Background())

			for aggregate, notifier := range modelUpdater.notifiers {
				go modelUpdater.listenToUpdaters(lifecycleCtx, notifier, aggregate)
			}

			return nil
		},

		OnStop: func(ctx context.Context) error {
			lifecycleCtxCancel()

			return nil
		},
	})

	return modelUpdater
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

	go m.registerAndHandleUpdateNotification(ctx, conn)
}

func (m *ModelUpdater) registerAndHandleUpdateNotification(ctx context.Context, conn *websocket.Conn) {
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			m.log.Error("failed to close websocket connection", zap.Error(err))
		}
	}(conn)

	listenerId := uuid.New()
	messageChannel := m.registerListener(listenerId)
	defer m.unregisterListener(listenerId)

	//go m.handleIncoming(conn)

	keepRunning := true
	for keepRunning {
		select {
		case <-ctx.Done():
			keepRunning = false

		case message := <-messageChannel:
			bytes, err := json.Marshal(message)
			if err != nil {
				m.log.Error(
					"failed to marshal web socket message",
					zap.String("aggregate", message.Subject),
					zap.String("event", message.Event),
					zap.Error(err),
				)

				break
			}

			m.log.Debug(
				"notifying web socket",
				zap.String("aggregate", message.Subject),
				zap.String("event", message.Event),
			)

			err = conn.WriteMessage(websocket.TextMessage, bytes)
			if err != nil {
				m.log.Warn(
					"failed to notify web socket",
					zap.String("aggregate", message.Subject),
					zap.String("event", message.Event),
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

func (m *ModelUpdater) registerListener(listenerId uuid.UUID) chan message {
	m.registeredListenersMutex.Lock()
	defer m.registeredListenersMutex.Unlock()

	channel := make(chan message)

	m.registeredListeners[listenerId] = channel

	return channel
}

func (m *ModelUpdater) unregisterListener(listenerId uuid.UUID) {
	m.registeredListenersMutex.Lock()
	defer m.registeredListenersMutex.Unlock()

	delete(m.registeredListeners, listenerId)
}

func (m *ModelUpdater) listenToUpdaters(ctx context.Context, notifier modelUpdateNotifier, aggregate eventhorizon.AggregateType) {
	keepRunning := true
	for keepRunning {
		select {
		case <-ctx.Done():
			keepRunning = false

		case modelUpdated := <-notifier.channel:
			m.registeredListenersMutex.Lock()

			updateMessage := message{
				Subject: aggregate.String(),
				Event:   modelUpdated.Event.String(),
			}

			for _, listenerChannel := range m.registeredListeners {
				listenerChannel <- updateMessage
			}

			m.registeredListenersMutex.Unlock()
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
