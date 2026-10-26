package fix

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// object is a JSON object that keeps its keys in file order, so a settings
// file can be changed in one place without reshuffling the rest of it.
type object struct {
	keys   []string
	values map[string]json.RawMessage
}

func parseObject(data []byte) (*object, error) {
	o := &object{values: map[string]json.RawMessage{}}
	if len(bytes.TrimSpace(data)) == 0 {
		return o, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("not a JSON object")
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key := tok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		if _, dup := o.values[key]; !dup {
			o.keys = append(o.keys, key)
		}
		o.values[key] = raw
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("unexpected data after the JSON object")
	}
	return o, nil
}

// child returns the object stored under key, or an empty one.
func (o *object) child(key string) (*object, error) {
	raw, ok := o.values[key]
	if !ok {
		return &object{values: map[string]json.RawMessage{}}, nil
	}
	return parseObject(raw)
}

func (o *object) set(key string, value any) error {
	raw, err := marshal(value)
	if err != nil {
		return err
	}
	if _, ok := o.values[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.values[key] = raw
	return nil
}

func (o *object) setObject(key string, child *object) error {
	return o.set(key, json.RawMessage(child.encode("")))
}

func (o *object) remove(key string) bool {
	if _, ok := o.values[key]; !ok {
		return false
	}
	delete(o.values, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
	return true
}

// encode writes the object with two-space indentation, the format Claude
// Code itself writes settings in. Values are re-indented, not re-encoded, so
// their escaping and number formatting stay as they were.
func (o *object) encode(prefix string) []byte {
	if len(o.keys) == 0 {
		return []byte("{}")
	}
	var b bytes.Buffer
	b.WriteString("{\n")
	for i, k := range o.keys {
		key, _ := marshal(k)
		var val bytes.Buffer
		if err := json.Indent(&val, o.values[k], prefix+"  ", "  "); err != nil {
			val.Write(o.values[k])
		}
		b.WriteString(prefix + "  ")
		b.Write(key)
		b.WriteString(": ")
		b.Write(val.Bytes())
		if i < len(o.keys)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString(prefix + "}")
	return b.Bytes()
}

func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}
