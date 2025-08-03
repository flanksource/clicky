package api

// Field type constants
const (
	FieldTypeString   = "string"
	FieldTypeInt      = "int"
	FieldTypeFloat    = "float"
	FieldTypeBoolean  = "boolean"
	FieldTypeDate     = "date"
	FieldTypeDuration = "duration"
	FieldTypeArray    = "array"
	FieldTypeMap      = "map"
	FieldTypeStruct   = "struct"
)

// Format constants
const (
	FormatTable    = "table"
	FormatCurrency = "currency"
	FormatHide     = "hide"
	FormatList     = "list"
	FormatDate     = "date"
	FormatFloat    = "float"
	FormatMarkdown = "markdown"
	FormatJSON     = "json"
	FormatYAML     = "yaml"
	FormatCSV      = "csv"
	FormatHTML     = "html"
	FormatPDF      = "pdf"
	FormatPretty   = "pretty"
)

// Common strings
const (
	EmptyValue     = "(empty)"
	DateTimeFormat = "2006-01-02 15:04:05"
	DateFormat     = "2006-01-02"
	TimeFormat     = "15:04:05"
	RFC3339Format  = "2006-01-02T15:04:05Z07:00"
	ISO8601Format  = "2006-01-02T15:04:05Z"
	AltDateFormat1 = "01/02/2006"
	AltDateFormat2 = "02/01/2006"
	AltDateFormat3 = "2006/01/02"
	UnixDateFormat = "Mon Jan _2 15:04:05 MST 2006"
	KitchenFormat  = "3:04PM"
)

// Color constants
const (
	ColorGreen = "green"
	ColorRed   = "red"
	ColorBlue  = "blue"
)

// Sort direction constants
const (
	SortAsc  = "asc"
	SortDesc = "desc"
)
