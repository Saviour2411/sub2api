package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newProxyQualityTestClient(t *testing.T, status int, headers http.Header, body string) *http.Client {
	t.Helper()

	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "application/json,text/html,*/*", req.Header.Get("Accept"))
			require.Equal(t, proxyQualityClientUserAgent, req.Header.Get("User-Agent"))

			return &http.Response{
				StatusCode: status,
				Header:     headers,
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    req,
			}, nil
		}),
	}
}

func TestFinalizeProxyQualityResult_ScoreAndGrade(t *testing.T) {
	result := &ProxyQualityCheckResult{
		PassedCount:    2,
		WarnCount:      1,
		FailedCount:    1,
		ChallengeCount: 1,
	}

	finalizeProxyQualityResult(result)

	require.Equal(t, 38, result.Score)
	require.Equal(t, "F", result.Grade)
	require.Contains(t, result.Summary, "通过 2 项")
	require.Contains(t, result.Summary, "告警 1 项")
	require.Contains(t, result.Summary, "失败 1 项")
	require.Contains(t, result.Summary, "挑战 1 项")
}

func TestRunProxyQualityTarget_CloudflareChallenge(t *testing.T) {
	target := proxyQualityTarget{
		Target: "openai",
		URL:    "https://example.test/v1/models",
		Method: http.MethodGet,
		AllowedStatuses: map[int]struct{}{
			http.StatusUnauthorized: {},
		},
	}
	client := newProxyQualityTestClient(
		t,
		http.StatusForbidden,
		http.Header{
			"Content-Type": []string{"text/html"},
			"Cf-Ray":       []string{"test-ray-123"},
		},
		"<!DOCTYPE html><title>Just a moment...</title><script>window._cf_chl_opt={};</script>",
	)

	item := runProxyQualityTarget(context.Background(), client, target)
	require.Equal(t, "challenge", item.Status)
	require.Equal(t, http.StatusForbidden, item.HTTPStatus)
	require.Equal(t, "test-ray-123", item.CFRay)
}

func TestRunProxyQualityTarget_AllowedStatusPass(t *testing.T) {
	target := proxyQualityTarget{
		Target: "gemini",
		URL:    "https://example.test/discovery",
		Method: http.MethodGet,
		AllowedStatuses: map[int]struct{}{
			http.StatusOK: {},
		},
	}
	client := newProxyQualityTestClient(t, http.StatusOK, http.Header{}, `{"models":[]}`)

	item := runProxyQualityTarget(context.Background(), client, target)
	require.Equal(t, "pass", item.Status)
	require.Equal(t, http.StatusOK, item.HTTPStatus)
}

func TestRunProxyQualityTarget_AllowedStatusPassForUnauthorized(t *testing.T) {
	target := proxyQualityTarget{
		Target: "openai",
		URL:    "https://example.test/v1/models",
		Method: http.MethodGet,
		AllowedStatuses: map[int]struct{}{
			http.StatusUnauthorized: {},
		},
	}
	client := newProxyQualityTestClient(t, http.StatusUnauthorized, http.Header{}, `{"error":"unauthorized"}`)

	item := runProxyQualityTarget(context.Background(), client, target)
	require.Equal(t, "pass", item.Status)
	require.Equal(t, http.StatusUnauthorized, item.HTTPStatus)
	require.Contains(t, item.Message, "目标可达")
}
