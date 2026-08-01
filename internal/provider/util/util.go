package util

import (
	"bytes"
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"

	// golang-sdk v3 has no shared/common package; ErrorResponseDto is
	// generated identically into every per-service package. We import it from
	// the sources package as an arbitrary but stable choice for this shared
	// error-parsing helper (the JSON shape is the same across all packages).
	sperr "github.com/sailpoint-oss/golang-sdk/v3/sources"
)

var (
	// A regular expression to match the error returned by net/http when the
	// configured number of redirects is exhausted. This error isn't typed
	// specifically so we resort to matching on the error string.
	redirectsErrorRe = regexp.MustCompile(`stopped after \d+ redirects\z`)

	// A regular expression to match the error returned by net/http when the
	// scheme specified in the URL is invalid. This error isn't typed
	// specifically so we resort to matching on the error string.
	schemeErrorRe = regexp.MustCompile(`unsupported protocol scheme`)

	// A regular expression to match the error returned by net/http when the
	// TLS certificate is not trusted. This error isn't typed
	// specifically so we resort to matching on the error string.
	notTrustedErrorRe = regexp.MustCompile(`certificate is not trusted`)
)

func Retry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	// do not retry on context.Canceled or context.DeadlineExceeded
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	// don't propagate other errors
	shouldRetry, _ := baseRetryPolicy(resp, err)
	return shouldRetry, nil
}

func baseRetryPolicy(resp *http.Response, err error) (bool, error) {
	if err != nil {
		if v, ok := err.(*url.Error); ok {
			// Don't retry if the error was due to too many redirects.
			if redirectsErrorRe.MatchString(v.Error()) {
				return false, v
			}

			// Don't retry if the error was due to an invalid protocol scheme.
			if schemeErrorRe.MatchString(v.Error()) {
				return false, v
			}

			// Don't retry if the error was due to TLS cert verification failure.
			if notTrustedErrorRe.MatchString(v.Error()) {
				return false, v
			}
			if _, ok := v.Err.(x509.UnknownAuthorityError); ok {
				return false, v
			}
		}

		// The error is likely recoverable so retry.
		return true, nil
	}

	// 429 Too Many Requests is recoverable. Sometimes the server puts
	// a Retry-After response header to indicate when the server is
	// available to start processing request from client.
	if resp.StatusCode == http.StatusTooManyRequests {
		return true, nil
	}

	// Check the response code. We retry on 500-range responses to allow
	// the server time to recover, as 500's are typically not permanent
	// errors and may relate to outages on the server side. This will catch
	// invalid response codes as well, like 0 and 999.
	if resp.StatusCode == 0 || (resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented) {
		return true, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}

	return false, nil
}

func SailpointErrorFromHTTPBody(resp *http.Response) (*sperr.ErrorResponseDto, bool) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false
	}
	// response := string(body)
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	spError := sperr.NewErrorResponseDto()

	err = spError.UnmarshalJSON(body)
	if err != nil {
		return nil, false
	}
	return spError, true
}

// SailpointErrorDetail builds a single-line diagnostic detail string from a
// golang-sdk error and its associated *http.Response, suitable for
// resp.Diagnostics.AddError. When the response body parses as a SailPoint
// ErrorResponseDto, the HTTP status, detailCode, trackingId (useful when
// opening a SailPoint support case), and any human-readable messages are
// included so operators get actionable detail directly in `terraform
// plan`/`apply` output without needing to re-run with TF_LOG. Falls back to
// the HTTP status plus the raw Go error, or just the raw Go error if there is
// no response at all (e.g. network/transport failure).
func SailpointErrorDetail(err error, httpResp *http.Response) string {
	if httpResp == nil {
		return err.Error()
	}

	spErr, ok := SailpointErrorFromHTTPBody(httpResp)
	if !ok {
		return fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode, err.Error())
	}

	detail := fmt.Sprintf("HTTP %d", httpResp.StatusCode)
	if spErr.HasDetailCode() {
		detail += fmt.Sprintf(", detailCode=%s", spErr.GetDetailCode())
	}
	if spErr.HasTrackingId() {
		detail += fmt.Sprintf(", trackingId=%s", spErr.GetTrackingId())
	}
	for _, m := range spErr.Messages {
		if m.Text != nil && *m.Text != "" {
			detail += fmt.Sprintf(", message=%q", *m.Text)
		}
	}
	return detail
}
