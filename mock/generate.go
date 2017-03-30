package mock

//go:generate mockgen -package mock -destination mock_monorail.go github.com/codedellemc/infrakit.rackhd/monorail Iface
//go:generate mockgen -package mock -destination mock_nodes.go github.com/codedellemc/infrakit.rackhd/monorail NodeIface
//go:generate mockgen -package mock -destination mock_skus.go github.com/codedellemc/infrakit.rackhd/monorail SkuIface
//go:generate mockgen -package mock -destination mock_tags.go github.com/codedellemc/infrakit.rackhd/monorail TagIface
