package encoding

const (
	ContentTypeDefault  ContentType = ContentType("")
	ContentTypeAny      ContentType = ContentType("*/*")
	ContentTypeJSON     ContentType = ContentType("application/json")
	ContentTypeYAML     ContentType = ContentType("application/yaml")
	ContentTypeProtobuf ContentType = ContentType("application/protobuf")
	ContentTypeHTML     ContentType = ContentType("text/html")
	ContentTypePlain    ContentType = ContentType("text/plain")
)

type ContentType string

func (this ContentType) String() string {
	return string(this)
}
