package libmachete

type EventType int

const (
	CreateStarted EventType = iota
	CreateInProgress
	CreateCompleted
	CreateError
)

type CreateEvent struct {
	Type       EventType
	Error      error
	ResourceId string
}

type BaseRequest struct {
	Name       string `json:"name"`
	SSHUser    string `json:"ssh_user"`
	SSHKeyPath string `json:"ssh_key_path"`
}

type CreateRequest struct {
	BaseRequest
	// TODO- driver-specific params
}

type Provisioner interface {
	Create(CreateRequest) (<-chan CreateEvent, error)
}
