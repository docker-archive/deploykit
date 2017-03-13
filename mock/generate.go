package mock

//go:generate mockgen -package mock -destination monorail/mock_monorail.go github.com/codedellemc/infrakit.rackhd/monorail Iface
//go:generate mockgen -package mock -destination mock/mock_nodes.go github.com/codedellemc/infrakit.rackhd/monorail NodeIface
//go:generate mockgen -package mock -destination mock/mock_skus.go github.com/codedellemc/infrakit.rackhd/monorail SkuIface
