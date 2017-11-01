package template

// import (
// 	"github.com/docker/infrakit/pkg/run/scope"
// 	"github.com/docker/infrakit/pkg/template"
// )

// // StdFunctions adds a set of standard functions for access in templates
// func StdFunctions(engine *template.Template, scp scope.Scope) *template.Template {

// 	engine.WithFunctions(func() []template.Function {
// 		return append(scp.TemplateFuncs,
// 			// This is an override of the existing Var function
// 			template.Function{
// 				Name: "var",
// 				Func: func(name string, optional ...interface{}) (interface{}, error) {

// 					if len(optional) > 0 {
// 						return engine.Var(name, optional...), nil
// 					}

// 					v := engine.Ref(name)
// 					if v == nil {
// 						// If not resolved, try to interpret the path as a path for metadata...
// 						m, err := scope.MetadataFunc(scp)(name, optional...)
// 						if err != nil {
// 							return nil, err
// 						}
// 						v = m
// 					}

// 					if v == nil && engine.Options().MultiPass {
// 						return engine.DeferVar(name), nil
// 					}

// 					return v, nil
// 				},
// 			},
// 		)
// 	})
// 	return engine
// }
