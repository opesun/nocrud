package hooks_test

import(
	iface "github.com/opesun/nocrud/frame/interfaces"
	"github.com/opesun/nocrud/frame/hooks"
	"github.com/opesun/nocrud/frame/mod"
	"testing"
)

type M struct {
	name	string
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

func (m *ModA) HookA(s string) {
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
	hooks := map[string]interface{}{
		"HookA": []interface{}{"modA"},
		"HookB": []interface{}{[]interface{}{"modB", "MethodB"}},
	}
	ev := Hook.New(nil, hooks, newModule)
	if a != 0 {
		t.Fatal(a)
	}
	ev.Fire("HookA", "testA")
	if a != 1 {
		t.Fatal(a)
	}
	ev.Fire("HookA", "asdadsad")
	if a != 1 {
		t.Fatal(a)
	}
	ev.Fire("HookC")	// Nothing should happend when we call a not existing Hook.
	if a != 1 {
		t.Fatal(a)
	}
	if b != 0 {
		t.Fatal(b)
	}
	ev.Fire("HookB", "testB")
	if b != 1 {
		t.Fatal(b)
	}
}

func TestStatePreserving(t *testing.T) {
	a = 0
	b = 0
	called = 0
	hooks := map[string]interface{}{
		"HookA": []interface{}{"modA"},
	}
	ev := Hook.New(nil, hooks, newModule)
	if called != 0 {
		t.Fatal(called)
	}
	for i:=0;i<10;i++ {
		ev.Fire("HookA", "dummy data")
		if called != i+1 {
			t.Fatal(called)
		}
	}
}