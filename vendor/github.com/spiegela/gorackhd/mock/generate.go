package mock

//go:generate mockgen -package mock -destination mock_monorail.go github.com/spiegela/gorackhd/monorail Iface
//go:generate mockgen -package mock -destination mock_nodes.go github.com/spiegela/gorackhd/monorail NodeIface
//go:generate mockgen -package mock -destination mock_skus.go github.com/spiegela/gorackhd/monorail SkuIface
//go:generate mockgen -package mock -destination mock_tags.go github.com/spiegela/gorackhd/monorail TagIface
