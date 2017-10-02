package oracle

// Response creates a struct for the Stringer type
type Response string

func (r Response) String() string {
	return string(r)
}
