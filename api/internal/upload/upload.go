// Package upload handles multipart/form-data parsing and file-field metadata binding for file uploads.
package upload

import (
	"errors"
	"mime/multipart"
	"strings"

	"github.com/labstack/echo/v4"
)

// MaxMultipartMemory is the parser buffer; must exceed parser max file size (50MiB).
const MaxMultipartMemory = 52 << 20

var (
	errNotMultipart    = errors.New("Content-Type must be multipart/form-data")
	errParseMultipart  = errors.New("could not parse multipart form")
	errReadMultipart   = errors.New("could not read multipart form")
	errInvalidFormData = errors.New("invalid form data")
)

// FormData holds optional Work / Expression / publication fields from multipart (same as POST /files).
type FormData struct {
	Subtype string `form:"subtype"`
	Number  string `form:"number"`
	Author  string `form:"author"`
	Date    string `form:"date"`
	Country string `form:"country"`
	Name    string `form:"name"`

	VersionDate   string `form:"version_date"`
	VersionNumber string `form:"version_number"`
	VersionLabel  string `form:"version_label"`
	Language      string `form:"language"`

	PubName   string `form:"pub_name"`
	PubDate   string `form:"pub_date"`
	PubNumber string `form:"pub_number"`

	// Chunk matching (diff) parameters. Nil = use env defaults.
	MatchThresholdLow  *float64 `form:"match_threshold_low"`
	MatchThresholdHigh *float64 `form:"match_threshold_high"`
}

// ParseMultipart parses multipart/form-data once per request. Call before Bind / FormFile / MultipartForm.
func ParseMultipart(c echo.Context) error {
	ct := c.Request().Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "multipart/form-data") {
		return errNotMultipart
	}
	if err := c.Request().ParseMultipartForm(MaxMultipartMemory); err != nil {
		return errParseMultipart
	}
	return nil
}

// MultipartForm returns the parsed form after ParseMultipart.
func MultipartForm(c echo.Context) (*multipart.Form, error) {
	mf, err := c.MultipartForm()
	if err != nil {
		return nil, errReadMultipart
	}
	return mf, nil
}

// HasFile reports whether the form has at least one file part for name.
func HasFile(mf *multipart.Form, name string) bool {
	fs := mf.File[name]
	return len(fs) > 0
}

// OpenMultipart parses multipart and returns the form (e.g. to probe file fields before saving).
func OpenMultipart(c echo.Context) (*multipart.Form, error) {
	if err := ParseMultipart(c); err != nil {
		return nil, err
	}
	return MultipartForm(c)
}

// BindFormData binds FormData from the request (multipart must already be parsed).
func BindFormData(c echo.Context) (FormData, error) {
	var req FormData
	if err := c.Bind(&req); err != nil {
		return req, errInvalidFormData
	}
	return req, nil
}

// PrepareForm parses multipart and binds FormData — same prelude as POST /files.
func PrepareForm(c echo.Context) (FormData, error) {
	if err := ParseMultipart(c); err != nil {
		return FormData{}, err
	}
	return BindFormData(c)
}
