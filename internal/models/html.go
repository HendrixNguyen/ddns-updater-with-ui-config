package models

// HTMLData is a list of HTML fields to be rendered.
// It is exported so that the HTML template engine can render it.
type HTMLData struct {
	Rows []HTMLRow
	// BasePath is the root URL the server is mounted under, without a
	// trailing slash, as a JSON string literal escaped so that it is safe to
	// embed in the text content of a script element. It is empty (the two
	// quote characters) when the server is mounted at the root.
	// The template publishes it as window.__DDNS_BASE__ so the page
	// JavaScript can build absolute API URLs under any root URL.
	BasePath string `json:"basePath"`
}

// HTMLRow contains HTML fields to be rendered
// It is exported so that the HTML template engine can render it.
type HTMLRow struct {
	Domain      string
	Owner       string
	Provider    string
	IPVersion   string
	Status      string
	CurrentIP   string
	PreviousIPs string
}
