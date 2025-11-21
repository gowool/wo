package wo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNegotiationFormatWithAccept(t *testing.T) {
	accepted := ParseAcceptHeader("text/html,application/xhtml+xml,application/xml;q=0.9;q=0.8")

	assert.Equal(t, MIMEApplicationXML, NegotiateFormat(accepted, MIMEApplicationJSON, MIMEApplicationXML))
	assert.Equal(t, MIMETextHTML, NegotiateFormat(accepted, MIMEApplicationXML, MIMETextHTML))
	assert.Equal(t, MIMETextHTMLCharsetUTF8, NegotiateFormat(accepted, MIMEApplicationXML, MIMETextHTMLCharsetUTF8))
	assert.Empty(t, NegotiateFormat(accepted, MIMEApplicationJSON))
}

func TestNegotiationFormatWithWildcardAccept(t *testing.T) {
	accepted := ParseAcceptHeader("*/*")

	assert.Equal(t, "*/*", NegotiateFormat(accepted, "*/*"))
	assert.Equal(t, "text/*", NegotiateFormat(accepted, "text/*"))
	assert.Equal(t, "application/*", NegotiateFormat(accepted, "application/*"))
	assert.Equal(t, MIMEApplicationJSON, NegotiateFormat(accepted, MIMEApplicationJSON)) //nolint:testifylint
	assert.Equal(t, MIMEApplicationXML, NegotiateFormat(accepted, MIMEApplicationXML))
	assert.Equal(t, MIMETextHTML, NegotiateFormat(accepted, MIMETextHTML))

	accepted = ParseAcceptHeader("text/*")

	assert.Equal(t, "*/*", NegotiateFormat(accepted, "*/*"))
	assert.Equal(t, "text/*", NegotiateFormat(accepted, "text/*"))
	assert.Empty(t, NegotiateFormat(accepted, "application/*"))
	assert.Empty(t, NegotiateFormat(accepted, MIMEApplicationJSON))
	assert.Empty(t, NegotiateFormat(accepted, MIMEApplicationXML))
	assert.Equal(t, MIMETextHTML, NegotiateFormat(accepted, MIMETextHTML))
}
