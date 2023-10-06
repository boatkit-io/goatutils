package subscribableevent

import (
	"fmt"
	"reflect"
	"sync"
)

type SubscriptionId uint

type trackedSub[F any] struct {
	subId    SubscriptionId
	callback F
	// Pre-fetch the reflection value for the callback to save some cycles on every callback
	callbackReflect reflect.Value
}

type Event[F any] struct {
	subMutex  sync.Mutex
	lastSubId SubscriptionId
	subs      map[SubscriptionId]*trackedSub[F]
	argKinds  []reflect.Kind
}

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
