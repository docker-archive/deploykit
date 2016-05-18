package encoding

import (
	"encoding/json"
	"fmt"
	"time"
)

type Duration struct {
	time.Duration
}

func (d *Duration) String() string {
	return d.Duration.String()
}

func (d *Duration) UnmarshalJSON(b []byte) (err error) {
	if b[0] == '"' {
		sd := string(b[1 : len(b)-1])
		d.Duration, err = time.ParseDuration(sd)
		return
	}

	var id int64
	id, err = json.Number(string(b)).Int64()
	d.Duration = time.Duration(id)

	return
}

func (d Duration) MarshalJSON() (b []byte, err error) {
	return []byte(fmt.Sprintf(`"%s"`, d.String())), nil
}

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) (err error) {
	stringBuff := ""
	err = unmarshal(&stringBuff)
	if err != nil {
		return
	}
	return d.UnmarshalJSON([]byte(fmt.Sprintf("\"%s\"", stringBuff))) // need to quote
}

func (d Duration) MarshalYAML() (v interface{}, err error) {
	return d.String(), nil
}
