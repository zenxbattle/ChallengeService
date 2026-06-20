package wss

import (
	"errors"
	"log"

	wsstypes "github.com/lijuuu/ChallengeWssManagerService/internal/wss/types"
)

// WsHandlerType defines the signature for a WebSocket event handler
type WsHandlerType func(*wsstypes.WsContext) error

// WsMiddleware defines the signature for middleware functions
type WsMiddleware func(*wsstypes.WsContext) error

// HandlerRegistration stores a handler with its associated middleware chain
type HandlerRegistration struct {
	Handler     WsHandlerType
	Middlewares []WsMiddleware
}

type Dispatcher struct {
	handlers map[string]*HandlerRegistration
}

func NewDispatcher() *Dispatcher {
	log.Println("dispatcher initialized")
	return &Dispatcher{
		handlers: make(map[string]*HandlerRegistration),
	}
}

func (d *Dispatcher) Register(event string, handler WsHandlerType) {
	log.Printf("registering handler for event: %s", event)
	d.handlers[event] = &HandlerRegistration{
		Handler:     handler,
		Middlewares: nil,
	}
}

func (d *Dispatcher) RegisterWithMiddleware(event string, handler WsHandlerType, middlewares ...WsMiddleware) {
	log.Printf("registering handler with middleware for event: %s", event)
	d.handlers[event] = &HandlerRegistration{
		Handler:     handler,
		Middlewares: middlewares,
	}
}

func (d *Dispatcher) Dispatch(event string, ctx *wsstypes.WsContext) error {
	log.Printf("dispatching event: %s", event)

	registration, ok := d.handlers[event]
	if !ok {
		log.Printf("no handler found for event: %s", event)
		return errors.New("unknown event type: " + event)
	}

	// Execute middleware chain before handler
	if len(registration.Middlewares) > 0 {
		err := d.executeMiddlewareChain(registration.Middlewares, ctx)
		if err != nil {
			log.Printf("middleware error for event %s: %v", event, err)
			return err
		}
	}

	// Execute the main handler
	err := registration.Handler(ctx)
	if err != nil {
		log.Printf("handler error for event %s: %v", event, err)
	}
	return err
}

// executeMiddlewareChain executes middleware functions in order, stopping on first error
func (d *Dispatcher) executeMiddlewareChain(middlewares []WsMiddleware, ctx *wsstypes.WsContext) error {
	for i, middleware := range middlewares {
		log.Printf("executing middleware %d of %d", i+1, len(middlewares))
		err := middleware(ctx)
		if err != nil {
			log.Printf("middleware %d failed: %v", i+1, err)
			return err
		}
	}
	log.Printf("middleware chain completed successfully")
	return nil
}
