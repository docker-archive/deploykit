package ucsclient

import (
	"fmt"
)

func tplConfigResolveDn(cookie, dn string) []byte {
	tpl := `<configResolveDn cookie="%s" dn="%s" inHierarchical="true" />
`
	return []byte(fmt.Sprintf(tpl, cookie, dn))
}
