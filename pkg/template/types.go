package template

// Function contains the description of an exported template function
type Function struct {

	// Name is the function name to bind in the template
	Name string

	// Description provides help for the function
	Description string

	// Func is the reference to the actual function
	Func interface{}
}

// AddFuncs exports the functions in the input slice
func (t *Template) AddFuncs(exported []Function) *Template {
	for _, tf := range exported {
		t.AddFunc(tf.Name, tf.Func)
	}
	return t
}
