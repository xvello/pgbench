package db

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryParser_Read(t *testing.T) {
	csvInput := `hostname,start_time,end_time
host_000008,2017-01-01 08:59:22,2017-01-01 09:59:22
invalid line
host_000002,2017-01-02 00:25:56,2017-01-02 01:25:56
`
	reader, err := NewQueryParser(strings.NewReader(csvInput))
	require.NoError(t, err)

	query, err := reader.Read()
	assert.NoError(t, err)
	assert.Equal(t, &Query{
		Hostname:  "host_000008",
		StartTime: "2017-01-01 08:59:22",
		EndTime:   "2017-01-01 09:59:22",
	}, query)

	query, err = reader.Read()
	assert.EqualError(t, err, "record on line 3: wrong number of fields")
	assert.Nil(t, query)

	query, err = reader.Read()
	assert.NoError(t, err)
	assert.Equal(t, &Query{
		Hostname:  "host_000002",
		StartTime: "2017-01-02 00:25:56",
		EndTime:   "2017-01-02 01:25:56",
	}, query)

	query, err = reader.Read()
	assert.Equal(t, io.EOF, err)
	assert.Nil(t, query)
}
