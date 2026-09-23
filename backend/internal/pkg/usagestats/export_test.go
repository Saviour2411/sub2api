package usagestats

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseExportPageOptions(t *testing.T) {
	options, err := ParseExportPageOptions("", "")
	require.NoError(t, err)
	require.Equal(t, MaxExportPageSize, options.PageSize)
	options, err = ParseExportPageOptions("9223372036854775807", "10")
	require.NoError(t, err)
	require.Equal(t, int64(9223372036854775807), options.BeforeID)
	require.Equal(t, 10, options.PageSize)
	for _, cursor := range []string{"0", "-1", "no", "1 OR 1=1", "9223372036854775808"} {
		_, err := ParseExportPageOptions(cursor, "")
		require.Error(t, err)
	}
	for _, size := range []string{"0", "-1", "1001", "many"} {
		_, err := ParseExportPageOptions("", size)
		require.Error(t, err)
	}
}
