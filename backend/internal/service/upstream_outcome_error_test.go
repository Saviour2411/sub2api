package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewUpstreamOutcomeErrorPreservesOutcomeWithoutFailover(t *testing.T) {
	cause := errors.New("upstream rejected request")
	err := NewUpstreamOutcomeError(http.StatusServiceUnavailable, []byte(`{"error":{"message":"busy"}}`), cause)

	require.Equal(t, http.StatusServiceUnavailable, err.StatusCode)
	require.JSONEq(t, `{"error":{"message":"busy"}}`, string(err.ResponseBody))
	require.ErrorIs(t, err, cause)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
}

func TestNewUpstreamOutcomeErrorNormalizesInvalidStatus(t *testing.T) {
	err := NewUpstreamOutcomeError(0, nil, nil)
	require.Equal(t, http.StatusBadGateway, err.StatusCode)
}

func TestOpenAIStreamFailedEventSemanticStatusTreatsPolicyRejectionAsBadRequest(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{
			name:    "cyber policy",
			payload: `{"type":"response.failed","response":{"error":{"code":"cyber_policy","message":"blocked"}}}`,
		},
		{
			name:    "content policy",
			payload: `{"type":"response.failed","response":{"error":{"code":"content_policy_violation","message":"blocked"}}}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, http.StatusBadRequest, openAIStreamFailedEventSemanticStatus([]byte(test.payload), "blocked"))
		})
	}
}

func TestOpenAIWSTurnOutcomeErrorClassifiesRawErrorEvent(t *testing.T) {
	serverError := openAIWSTurnOutcomeError(
		"error",
		[]byte(`{"type":"error","error":{"type":"server_error","code":"server_error","message":"busy"}}`),
		false,
	)
	require.NotNil(t, serverError)
	require.Equal(t, http.StatusBadGateway, serverError.StatusCode)
	require.False(t, serverError.ClientDisconnect)

	invalidRequest := openAIWSTurnOutcomeError(
		"error",
		[]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"bad input"}}`),
		true,
	)
	require.NotNil(t, invalidRequest)
	require.Equal(t, http.StatusBadRequest, invalidRequest.StatusCode)
	require.True(t, invalidRequest.ClientDisconnect)

	for _, code := range []string{
		"cyber_policy",
		"content_policy_violation",
		"safety_policy",
		"moderation_blocked",
		"policy_violation",
	} {
		t.Run(code, func(t *testing.T) {
			policyError := openAIWSTurnOutcomeError(
				"error",
				[]byte(`{"type":"error","error":{"type":"server_error","code":"`+code+`","message":"blocked"}}`),
				false,
			)
			require.NotNil(t, policyError)
			require.Equal(t, http.StatusBadRequest, policyError.StatusCode)
		})
	}
}
