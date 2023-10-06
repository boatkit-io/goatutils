// Package subscribableevent is just to hold the Event structure documented below
package subscribableevent

import (
	"fmt"
	"reflect"
	"sync"
)

// SubscriptionId is a strongly-typed opaque token for representing a subscription, which can be passed back
// in to unsubscrbe.
type SubscriptionId uint

// trackedSub is an internal tracking struct for a single subscription
type trackedSub[F any] struct {
	subId    SubscriptionId
	callback F
	// Pre-fetch the reflection value for the callback to save some cycles on every callback
	callbackReflect reflect.Value
}

// Event is a simple structure allowing for generic structured event subscription -- create a strongly-typed
// Event, and then zero or more subscribers can listen to it and/or fire messages into it.
type Event[F any] struct {
	subMutex  sync.Mutex
	lastSubId SubscriptionId
	subs      map[SubscriptionId]*trackedSub[F]
	argKinds  []reflect.Kind
}

// NewEvent returns a new Event object
func NewEvent[F any]() Event[F] {
	// Sanity check the F type
	var zero [0]F
	tt := reflect.TypeOf(zero).Elem()
	if tt.Kind() != reflect.Func {
		panic(fmt.Sprintf("Invalid kind used with NewEvent: %+v", tt))
	}

	pc := tt.NumIn()
	kinds := make([]reflect.Kind, pc)
	for i := 0; i < pc; i++ {
		kinds[i] = tt.In(i).Kind()
	}

	return Event[F]{
		lastSubId: 0,
		subs:      map[SubscriptionId]*trackedSub[F]{},
		argKinds:  kinds,
	}
}

// Subscribe will subscribe to any events fired from the Event object, returning a SubscriptionId for later unsubscribing (if desired).
func (e *Event[F]) Subscribe(callback F) SubscriptionId {
	e.subMutex.Lock()
	defer e.subMutex.Unlock()

	e.lastSubId++
	ts := &trackedSub[F]{
		subId:           e.lastSubId,
		callback:        callback,
		callbackReflect: reflect.ValueOf(callback),
	}

	e.subs[ts.subId] = ts

	return ts.subId
}

// Unsubscribe will unsubscribe a specific SubscriptionId from the Event's subscribed callbacks.
func (e *Event[F]) Unsubscribe(subId SubscriptionId) error {
	e.subMutex.Lock()
	defer e.subMutex.Unlock()

	_, exists := e.subs[subId]
	if !exists {
		return fmt.Errorf("subscription %d not found", subId)
	}

	delete(e.subs, subId)

	return nil
}

// Fire will call all of the subscribed callbacks back with the args passed, which are type-checked against the
// type of the callbacks.
func (e *Event[F]) Fire(args ...any) {
	// Make a cloned list of what to call back inside the mutex, then call them back later outside the mutex, in case
	// someone tries to mutate the subscription list in a callback.
	e.subMutex.Lock()
	toCall := make([]reflect.Value, 0)
	for _, s := range e.subs {
		toCall = append(toCall, s.callbackReflect)
	}
	e.subMutex.Unlock()

	// Validate arg count
	na := len(args)
	if na != len(e.argKinds) {
		panic(fmt.Sprintf("Fire called with %v params when it should have been %v", na, len(e.argKinds)))
	}

	argVs := make([]reflect.Value, na)
	for i := range args {
		v := reflect.ValueOf(args[i])
		argVs[i] = v

		if e.argKinds[i] != reflect.Interface {
			// If the arg isn't an interface, validate the kind of the args, just in case a dev messed up
			if v.Kind() != e.argKinds[i] {
				panic(fmt.Sprintf("Invalid kind called into Fire(): %v should be %v", v.Kind(), e.argKinds[i]))
			}
		}
	}

	for i := range toCall {
		toCall[i].Call(argVs)
	}
}
