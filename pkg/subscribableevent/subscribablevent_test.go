package subscribableevent_test

import (
	"testing"

	"github.com/boatkit-io/goatutils/pkg/subscribableevent"
	"github.com/stretchr/testify/assert"
)

func TestBasics(t *testing.T) {
	lastval := int(0)
	calls := 0
	cb := func(v int) {
		lastval = v
		calls++
	}

	assert.Equal(t, 0, lastval)
	assert.Equal(t, 0, calls)
	e := subscribableevent.NewEvent[func(int)]()
	assert.Equal(t, 0, lastval)
	assert.Equal(t, 0, calls)
	e.Fire(3)
	assert.Equal(t, 0, lastval)
	assert.Equal(t, 0, calls)
	si := e.Subscribe(cb)
	assert.Equal(t, 0, lastval)
	assert.Equal(t, 0, calls)
	e.Fire(4)
	assert.Equal(t, 4, lastval)
	assert.Equal(t, 1, calls)
	e.Fire(5)
	assert.Equal(t, 5, lastval)
	assert.Equal(t, 2, calls)
	err := e.Unsubscribe(si)
	assert.NoError(t, err)
	e.Fire(6)
	assert.Equal(t, 5, lastval)
	assert.Equal(t, 2, calls)

	err = e.Unsubscribe(si)
	assert.Error(t, err)

	assert.Panics(t, func() {
		e.Fire("hi")
	})
	assert.Panics(t, func() {
		e.Fire(4, 4)
	})

	assert.Panics(t, func() {
		subscribableevent.NewEvent[int]()
	})
}
