// Package page defines the data handed from the Go backend to the frontend.
package page

// Kind says how the frontend should display a Page.
type Kind string

const (
	KindMarkdown Kind = "markdown"
	KindFallback Kind = "fallback"
	KindError    Kind = "error"
	// KindExternal is the answer to a clicked link in Bonbon mode that turned out to be a web page: it belongs
	// in the reader's own browser, and the tab stays where it was.
	KindExternal Kind = "external"
	// KindAnchor is the answer to a move inside the page already shown (a #link, or Back or Forward between two
	// places on it): nothing is loaded again, only the address, the buttons and the scroll position change.
	KindAnchor Kind = "anchor"
)

// Link is a hyperlink found on a page.
type Link struct {
	Text string `json:"text"`
	Href string `json:"href"`
}

// Page is one loaded document.
type Page struct {
	URL           string `json:"url"`
	Kind          Kind   `json:"kind"`
	Status        int    `json:"status"`
	ContentType   string `json:"contentType"`
	Title         string `json:"title"`
	HTML          string `json:"html"`       // sanitized, links resolved (markdown only)
	Source        string `json:"source"`     // raw markdown (markdown only)
	Links         []Link `json:"links"`      // fallback only
	Favicon       string `json:"favicon"`    // the icon the page names in its front matter (favicon:), as a full address
	MarkdownOf    string `json:"markdownOf"` // set when this is the .md version of an HTML page: that page's address
	Negotiated    bool   `json:"negotiated"` // this address answered with markdown because it was asked to; it has a web page too
	OwnAddress    string `json:"ownAddress"` // the address of this page under the name the site asks for (bonbon.json, cname)
	EditURL       string `json:"editUrl"`    // where the markdown can be saved with PUT, when the server allows it
	ETag          string `json:"etag"`       // the version loaded, sent back as If-Match
	Error         string `json:"error"`
	Submitted     bool   `json:"submitted"`     // a save the server accepted but has not applied (a review queue); the page is still the old text
	SubmittedNote string `json:"submittedNote"` // what the server said about it

	Origin     string `json:"origin"`     // set on 401/403 so the UI can offer credentials
	AuthScheme string `json:"authScheme"` // "basic", "bearer", ... from WWW-Authenticate

	CanBack    bool `json:"canBack"`
	CanForward bool `json:"canForward"`
}
