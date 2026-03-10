// SPDX-License-Identifier: MIT

package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	garmWs "github.com/cloudbase/garm-provider-common/util/websocket"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/gorilla/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"

	garmoperatorv1beta1 "github.com/mercedes-benz/garm-operator/api/v1beta1"
	garmClient "github.com/mercedes-benz/garm-operator/pkg/client"
)

const (
	wsPath = "/api/v1/ws/events"

	initialBackoff = 1 * time.Second
	maxBackoff     = 30 * time.Second
	backoffFactor  = 2
)

// Watcher connects to the GARM websocket events endpoint and
// converts database change events into controller-runtime GenericEvents
// that are fed into the RunnerReconciler's reconcile channel.
type Watcher struct {
	client         garmClient.GarmClient
	watchNamespace string
	reconcileChan  chan event.GenericEvent

	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	running bool
}

func NewWatcher(client garmClient.GarmClient, watchNamespace string, reconcileChan chan event.GenericEvent) *Watcher {
	return &Watcher{
		client:         client,
		watchNamespace: watchNamespace,
		reconcileChan:  reconcileChan,
	}
}

// Start begins the websocket watch loop in a goroutine.
// It automatically reconnects on disconnection with exponential backoff.
func (w *Watcher) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		return
	}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.running = true
	go w.loop()
}

// Stop terminates the watcher.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	w.cancel()
	w.running = false
}

func (w *Watcher) loop() {
	log := log.FromContext(w.ctx)
	backoff := initialBackoff

	for {
		select {
		case <-w.ctx.Done():
			log.Info("Websocket watcher shutting down")
			return
		default:
		}

		err := w.connect()
		if err != nil {
			log.Error(err, "Websocket connection failed, will retry", "backoff", backoff)
			select {
			case <-w.ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = backoff * backoffFactor
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// reset backoff on successful connection
		backoff = initialBackoff
	}
}

func (w *Watcher) connect() error {
	log := log.FromContext(w.ctx)

	// re-login to get a fresh token for the websocket
	if err := w.client.Login(); err != nil {
		return fmt.Errorf("failed to login for websocket: %w", err)
	}

	token := w.client.RawToken()
	baseURL := w.client.BaseURL()

	reader, err := garmWs.NewReader(w.ctx, baseURL, wsPath, token, w.handleMessage)
	if err != nil {
		return fmt.Errorf("failed to create websocket reader: %w", err)
	}

	if err := reader.Start(); err != nil {
		return fmt.Errorf("failed to start websocket reader: %w", err)
	}
	defer reader.Stop()

	log.Info("Connected to GARM websocket events endpoint")

	// subscribe to instance events (runner instances)
	filter := `{"filters":[{"entity-type":"instance","operations":["create","update","delete"]}]}`
	if err := reader.WriteMessage(websocket.TextMessage, []byte(filter)); err != nil {
		return fmt.Errorf("failed to send event filter: %w", err)
	}

	log.Info("Subscribed to instance events via websocket")

	// block until the reader is done (disconnected) or context is cancelled
	select {
	case <-w.ctx.Done():
		return nil
	case <-reader.Done():
		return fmt.Errorf("websocket connection closed")
	}
}

func (w *Watcher) handleMessage(_ int, msg []byte) error {
	log := log.FromContext(w.ctx)

	var payload dbCommon.ChangePayload
	if err := json.Unmarshal(msg, &payload); err != nil {
		log.Error(err, "Failed to unmarshal websocket event")
		return nil
	}

	if payload.EntityType != dbCommon.InstanceEntityType {
		return nil
	}

	// The payload is interface{}, which json.Unmarshal turns into map[string]interface{}.
	// Re-marshal and unmarshal into the concrete Instance type.
	payloadBytes, err := json.Marshal(payload.Payload)
	if err != nil {
		log.Error(err, "Failed to re-marshal instance payload")
		return nil
	}

	var instance params.Instance
	if err := json.Unmarshal(payloadBytes, &instance); err != nil {
		log.Error(err, "Failed to unmarshal instance from event payload")
		return nil
	}

	if instance.Name == "" {
		log.V(1).Info("Received instance event with empty name, skipping", "operation", payload.Operation)
		return nil
	}

	log.V(1).Info("Received instance event", "operation", payload.Operation, "instance", instance.Name)

	e := event.GenericEvent{
		Object: &garmoperatorv1beta1.Runner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      strings.ToLower(instance.Name),
				Namespace: w.watchNamespace,
			},
		},
	}

	select {
	case w.reconcileChan <- e:
	default:
		log.V(1).Info("Reconcile channel full, dropping event", "instance", instance.Name)
	}

	return nil
}
