package hooks_test

import (
	"github.com/opesun/nocrud/frame/impl/hooks"
	iface "github.com/opesun/nocrud/frame/interfaces"
	"github.com/opesun/nocrud/frame/mod"
	"testing"
)

type M struct {
	name string
}

func (m *M) Instance() iface.Instance {
	if m.name == "modA" {
		return mod.ToInstance(&ModA{})
	} else if m.name == "modB" {
		return mod.ToInstance(&ModB{})
	}
	return nil
}

func (m *M) Exists() bool {
	if m.name == "modA" || m.name == "modB" {
		return true
	}
	return false
}

type ModA struct {
	called int
}

var a = 0
var called = 0

func (m *ModA) hooksA(s string) {
	m.called++
	called = m.called
	if s == "testA" {
		a++
	}
}

type ModB struct{}

var b = 0

func (m *ModB) MethodB(s string) {
	if s == "testB" {
		b++
	}
}

func newModule(s string) iface.Module {
	return &M{s}
}

func TestMethodDispatch(t *testing.T) {
	a = 0
	b = 0
	called = 0
	hookZ := map[string]interface{}{
		"hooksA": []interface{}{"modA"},
		"hooksB": []interface{}{[]interface{}{"modB", "MethodB"}},
	}
	ev := hooks.New(nil, hookZ, newModule)
	if a != 0 {
		t.Fatal(a)
	}
	ev.Fire("hooksA", "testA")
	if a != 1 {
		t.Fatal(a)
	}
	ev.Fire("hooksA", "asdadsad")
	if a != 1 {
		t.Fatal(a)
	}
	ev.Fire("hooksC") // Nothing should happend when we call a not existing hooks.
	if a != 1 {
		t.Fatal(a)
	}
	if b != 0 {
		t.Fatal(b)
	}
	ev.Fire("hooksB", "testB")
	if b != 1 {
		t.Fatal(b)
	}
}

func TestStatePreserving(t *testing.T) {
	a = 0
	b = 0
	called = 0
	hookZ := map[string]interface{}{
		"hooksA": []interface{}{"modA"},
	}
	ev := hooks.New(nil, hookZ, newModule)
	if called != 0 {
		t.Fatal(called)
	}
	for i := 0; i < 10; i++ {
		ev.Fire("hooksA", "dummy data")
		if called != i+1 {
			t.Fatal(called)
		}
	}
}
