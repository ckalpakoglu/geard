package introspect

import (
	"encoding/xml"
	"github.com/guelfey/go.dbus"
	"reflect"
)

// Introspectable implements org.freedesktop.Introspectable.
//
// You can create it by converting the XML-formatted introspection data from a
// string to an Introspectable or call NewIntrospectable with a Node. Then,
// export it as org.freedesktop.Introspectable on you object.
type Introspectable string

// NewIntrospectable returns an Introspectable that returns the introspection
// data that corresponds to the given Node. If n.Interfaces doesn't contain the
// data for org.freedesktop.DBus.Introspectable, it is added automatically.
func NewIntrospectable(n *Node) Introspectable {
	found := false
	for _, v := range n.Interfaces {
		if v.Name == "org.freedesktop.DBus.Introspectable" {
			found = true
			break
		}
	}
	if !found {
		n.Interfaces = append(n.Interfaces, IntrospectData)
	}
	b, err := xml.Marshal(n)
	if err != nil {
		panic(err)
	}
	return Introspectable(b)
}

// Introspect implements org.freedesktop.Introspectable.Introspect.
func (i Introspectable) Introspect() (string, *dbus.Error) {
	return string(i), nil
}

// Methods returns the description of the methods of v. This can be used to
// create a Node which can be passed to NewIntrospectable.
func Methods(v interface{}) []Method {
	t := reflect.TypeOf(v)
	ms := make([]Method, 0, t.NumMethod())
	for i := 0; i < t.NumMethod(); i++ {
		if t.Method(i).PkgPath != "" {
			continue
		}
		mt := t.Method(i).Type
		if mt.NumOut() == 0 ||
			mt.Out(mt.NumOut()-1) != reflect.TypeOf(&dbus.Error{"", nil}) {

			continue
		}
		var m Method
		m.Name = t.Method(i).Name
		m.Args = make([]Arg, mt.NumIn()+mt.NumOut()-2)
		for j := 1; j < mt.NumIn(); j++ {
			m.Args[j-1] = Arg{"", dbus.GetSignatureType(mt.In(j)).String(), "in"}
		}
		for j := 0; j < mt.NumOut()-1; j++ {
			m.Args[mt.NumIn()+j-1] = Arg{"",
				dbus.GetSignatureType(mt.Out(j)).String(), "out"}
		}
		m.Annotations = make([]Annotation, 0)
		ms = append(ms, m)
	}
	return ms
}
