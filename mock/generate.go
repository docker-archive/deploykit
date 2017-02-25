package mock

//go:generate mockgen -package mock -destination monorail/mock_monorail.go github.com/codedellemc/infrakit.rackhd/monorail Iface
//go:generage mockgen -package mock -destination mock/mock_nodes.go github.com/codedellemc/infrakit.rackhd/monorail NodeIface
