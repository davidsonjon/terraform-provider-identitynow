package util

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

var (
	// Default retry configuration
	defaultRetryWaitMin = 1 * time.Second
	defaultRetryWaitMax = 30 * time.Second
	defaultRetryMax     = 4

	// defaultLogger is the logger provided with defaultClient
	defaultLogger = log.New(os.Stderr, "", log.LstdFlags)

	// defaultClient is used for performing requests without explicitly making
	// a new client. It is purposely private to avoid modifications.
	defaultClient = retryablehttp.NewClient()

	// We need to consume response bodies to maintain http connections, but
	// limit the size we consume to respReadLimit.
	respReadLimit = int64(4096)

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

	// Check if the response is in sailpoint error format
	sailpointError, isSailpointError := SailpointErrorFromHTTPBody(resp)
	if isSailpointError {
		fmt.Fprintf(os.Stderr, "sailpointError: %v\n", sailpointError)
		if sailpointError.SlptErrorCode == "SLPT-1211" {
			fmt.Fprintf(os.Stderr, "Sync going!: %v\n", sailpointError.MessageTemplate)
			return true, nil
		}
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

type SailpointError struct {
	Response         string
	MessageTemplate  string `json:"msg_template,omitempty"`
	Message          string `json:"message,omitempty"`
	FormattedMessage string `json:"formatted_msg,omitempty"`

	Code           int    `json:"code,omitempty"`
	ErrorCode      int    `json:"error_code,omitempty"`
	SlptErrorCode  string `json:"slpt_error_code,omitempty"`
	ExceptionClass string `json:"exception_class,omitempty"`
}

func (s *SailpointError) Error() string {
	// return s.Response
	return s.Response
}

func SailpointErrorFromHTTPBody(resp *http.Response) (*SailpointError, bool) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "err: %v\n", err)
	}
	// response := string(body)
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	s := &SailpointError{}
	err = json.Unmarshal(body, s)
	if err != nil {
		return nil, false
	}
	if s.ErrorCode == 0 && s.Code == 0 {
		return nil, false
	}
	s.Response = string(body)
	return s, true
}
