package types

import (
	"fmt"
)

type errMissingAttribute string

func (e errMissingAttribute) Error() string {
	return fmt.Sprintf("missing attribute: %s", string(e))
}
