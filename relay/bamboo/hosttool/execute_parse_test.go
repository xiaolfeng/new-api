package hosttool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedToolInput(t *testing.T) {
	assert.Nil(t, SeedToolInput(nil))
	assert.Nil(t, SeedToolInput([]byte("")))
	assert.Nil(t, SeedToolInput([]byte("   ")))
	assert.Nil(t, SeedToolInput([]byte("null")))
	assert.Nil(t, SeedToolInput([]byte("{}")))
	assert.Nil(t, SeedToolInput([]byte(" {} ")))
	assert.Equal(t, []byte(`{"query":"x"}`), SeedToolInput([]byte(`{"query":"x"}`)))
}

func TestParseSearchQuery(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "query object", input: `{"query":"筱锋"}`, want: "筱锋"},
		{name: "concatenated placeholder", input: `{}{"query":"筱锋"}`, want: "筱锋"},
		{name: "json string", input: `"筱锋"`, want: "筱锋"},
		{name: "search alias", input: `{"search":"筱锋"}`, want: "筱锋"},
		{name: "queries array", input: `{"queries":["筱锋"]}`, want: "筱锋"},
		{name: "empty object", input: `{}`, wantErr: true},
		{name: "empty", input: ``, wantErr: true},
		{name: "objective", input: `{"objective":"XiaoLFeng"}`, want: "XiaoLFeng"},
		{name: "keyword", input: `{"keyword":"筱锋"}`, want: "筱锋"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw, scalar := parseToolInput([]byte(tc.input))
			got := extractSearchQuery(raw, scalar)
			if tc.wantErr {
				assert.Empty(t, got)
				return
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func TestClientIPFromHeaders(t *testing.T) {
	assert.Equal(t, "1.2.3.4", clientIPFromHeaders(map[string]string{
		"X-Forwarded-For": "1.2.3.4, 10.0.0.1",
	}))
	assert.Equal(t, "8.8.8.8", clientIPFromHeaders(map[string]string{
		"X-Real-Ip": "8.8.8.8",
	}))
	assert.Empty(t, clientIPFromHeaders(nil))
}

func TestParseFetchURLConcatenated(t *testing.T) {
	t.Parallel()
	raw, scalar := parseToolInput([]byte(`{}{"url":"https://example.com"}`))
	assert.Equal(t, "https://example.com", extractFetchURL(raw, scalar))

	raw, scalar = parseToolInput([]byte(`"https://example.com/a"`))
	assert.Equal(t, "https://example.com/a", extractFetchURL(raw, scalar))

	raw, scalar = parseToolInput([]byte(`"筱锋"`))
	assert.Empty(t, extractFetchURL(raw, scalar))

	raw, scalar = parseToolInput([]byte(`{}`))
	assert.Empty(t, extractFetchURL(raw, scalar))
}
