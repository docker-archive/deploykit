package instance

import (
	logutil "github.com/docker/infrakit/pkg/log"
)

var (
	log     = logutil.New("module", "provider/aws")
	debugV  = logutil.V(500)
	debugV2 = logutil.V(1000)
)
