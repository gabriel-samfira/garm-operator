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
	"github.com/go-logr/logr"
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
// that are fed into the various reconcilers' reconcile channels.
type Watcher struct {
	client         garmClient.GarmClient
	watchNamespace string

	// reconcile channels for different entity types
	runnerChan       chan event.GenericEvent
	repositoryChan   chan event.GenericEvent
	organizationChan chan event.GenericEvent
	enterpriseChan   chan event.GenericEvent

	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	running bool
}

type WatcherChannels struct {
	RunnerChan       chan event.GenericEvent
	RepositoryChan   chan event.GenericEvent
	OrganizationChan chan event.GenericEvent
	EnterpriseChan   chan event.GenericEvent
}

func NewWatcher(client garmClient.GarmClient, watchNamespace string, channels WatcherChannels) *Watcher {
	return &Watcher{
		client:           client,
		watchNamespace:   watchNamespace,
		runnerChan:       channels.RunnerChan,
		repositoryChan:   channels.RepositoryChan,
		organizationChan: channels.OrganizationChan,
		enterpriseChan:   channels.EnterpriseChan,
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

	// subscribe to instance and entity events
	filter := `{"filters":[` +
		`{"entity-type":"instance","operations":["create","update","delete"]},` +
		`{"entity-type":"repository","operations":["create","update","delete"]},` +
		`{"entity-type":"organization","operations":["create","update","delete"]},` +
		`{"entity-type":"enterprise","operations":["create","update","delete"]}` +
		`]}`
	if err := reader.WriteMessage(websocket.TextMessage, []byte(filter)); err != nil {
		return fmt.Errorf("failed to send event filter: %w", err)
	}

	log.Info("Subscribed to instance and entity events via websocket")

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

	switch payload.EntityType {
	case dbCommon.InstanceEntityType:
		return w.handleInstanceEvent(log, payload)
	case dbCommon.RepositoryEntityType:
		return w.handleRepositoryEvent(log, payload)
	case dbCommon.OrganizationEntityType:
		return w.handleOrganizationEvent(log, payload)
	case dbCommon.EnterpriseEntityType:
		return w.handleEnterpriseEvent(log, payload)
	default:
		// ignore other entity types
		return nil
	}
}

func (w *Watcher) handleInstanceEvent(log logr.Logger, payload dbCommon.ChangePayload) error {
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

	if w.runnerChan == nil {
		return nil
	}

	e := event.GenericEvent{
		Object: &garmoperatorv1beta1.Runner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      strings.ToLower(instance.Name),
				Namespace: w.watchNamespace,
			},
		},
	}

	select {
	case w.runnerChan <- e:
	default:
		log.V(1).Info("Runner channel full, dropping event", "instance", instance.Name)
	}

	return nil
}

func (w *Watcher) handleRepositoryEvent(log logr.Logger, payload dbCommon.ChangePayload) error {
	payloadBytes, err := json.Marshal(payload.Payload)
	if err != nil {
		log.Error(err, "Failed to re-marshal repository payload")
		return nil
	}

	var repo params.Repository
	if err := json.Unmarshal(payloadBytes, &repo); err != nil {
		log.Error(err, "Failed to unmarshal repository from event payload")
		return nil
	}

	if repo.Name == "" {
		log.V(1).Info("Received repository event with empty name, skipping", "operation", payload.Operation)
		return nil
	}

	log.V(1).Info("Received repository event", "operation", payload.Operation, "repository", repo.Name)

	if w.repositoryChan == nil {
		return nil
	}

	e := event.GenericEvent{
		Object: &garmoperatorv1beta1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Name:      strings.ToLower(repo.Name),
				Namespace: w.watchNamespace,
			},
		},
	}

	select {
	case w.repositoryChan <- e:
	default:
		log.V(1).Info("Repository channel full, dropping event", "repository", repo.Name)
	}

	return nil
}

func (w *Watcher) handleOrganizationEvent(log logr.Logger, payload dbCommon.ChangePayload) error {
	payloadBytes, err := json.Marshal(payload.Payload)
	if err != nil {
		log.Error(err, "Failed to re-marshal organization payload")
		return nil
	}

	var org params.Organization
	if err := json.Unmarshal(payloadBytes, &org); err != nil {
		log.Error(err, "Failed to unmarshal organization from event payload")
		return nil
	}

	if org.Name == "" {
		log.V(1).Info("Received organization event with empty name, skipping", "operation", payload.Operation)
		return nil
	}

	log.V(1).Info("Received organization event", "operation", payload.Operation, "organization", org.Name)

	if w.organizationChan == nil {
		return nil
	}

	e := event.GenericEvent{
		Object: &garmoperatorv1beta1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      strings.ToLower(org.Name),
				Namespace: w.watchNamespace,
			},
		},
	}

	select {
	case w.organizationChan <- e:
	default:
		log.V(1).Info("Organization channel full, dropping event", "organization", org.Name)
	}

	return nil
}

func (w *Watcher) handleEnterpriseEvent(log logr.Logger, payload dbCommon.ChangePayload) error {
	payloadBytes, err := json.Marshal(payload.Payload)
	if err != nil {
		log.Error(err, "Failed to re-marshal enterprise payload")
		return nil
	}

	var ent params.Enterprise
	if err := json.Unmarshal(payloadBytes, &ent); err != nil {
		log.Error(err, "Failed to unmarshal enterprise from event payload")
		return nil
	}

	if ent.Name == "" {
		log.V(1).Info("Received enterprise event with empty name, skipping", "operation", payload.Operation)
		return nil
	}

	log.V(1).Info("Received enterprise event", "operation", payload.Operation, "enterprise", ent.Name)

	if w.enterpriseChan == nil {
		return nil
	}

	e := event.GenericEvent{
		Object: &garmoperatorv1beta1.Enterprise{
			ObjectMeta: metav1.ObjectMeta{
				Name:      strings.ToLower(ent.Name),
				Namespace: w.watchNamespace,
			},
		},
	}

	select {
	case w.enterpriseChan <- e:
	default:
		log.V(1).Info("Enterprise channel full, dropping event", "enterprise", ent.Name)
	}

	return nil
}
