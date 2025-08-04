package clickhouse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTTLToSeconds(t *testing.T) {
	// INTERVAL syntax tests
	assert.Equal(t, uint64(30*86400), parseTTLToSeconds("INTERVAL 30 DAY"))
	assert.Equal(t, uint64(2*604800), parseTTLToSeconds("INTERVAL 2 WEEK"))
	assert.Equal(t, uint64(24*3600), parseTTLToSeconds("INTERVAL 24 HOUR"))
	assert.Equal(t, uint64(120*60), parseTTLToSeconds("INTERVAL 120 MINUTE"))
	assert.Equal(t, uint64(300), parseTTLToSeconds("INTERVAL 300 SECOND"))
	assert.Equal(t, uint64(3*2592000), parseTTLToSeconds("INTERVAL 3 MONTH"))
	assert.Equal(t, uint64(7776000), parseTTLToSeconds("INTERVAL 1 QUARTER"))
	assert.Equal(t, uint64(31536000), parseTTLToSeconds("INTERVAL 1 YEAR"))
	assert.Equal(t, uint64(7*86400), parseTTLToSeconds("  INTERVAL  7   DAY  "))
	assert.Equal(t, uint64(5*86400), parseTTLToSeconds("INTERVAL 5 DAYS"))

	// toInterval function tests
	assert.Equal(t, uint64(30*86400), parseTTLToSeconds("toIntervalDay(30)"))
	assert.Equal(t, uint64(2*604800), parseTTLToSeconds("toIntervalWeek(2)"))
	assert.Equal(t, uint64(24*3600), parseTTLToSeconds("toIntervalHour(24)"))
	assert.Equal(t, uint64(120*60), parseTTLToSeconds("toIntervalMinute(120)"))
	assert.Equal(t, uint64(300), parseTTLToSeconds("toIntervalSecond(300)"))
	assert.Equal(t, uint64(3*2592000), parseTTLToSeconds("toIntervalMonth(3)"))
	assert.Equal(t, uint64(7776000), parseTTLToSeconds("toIntervalQuarter(1)"))
	assert.Equal(t, uint64(31536000), parseTTLToSeconds("toIntervalYear(1)"))

	// Edge cases and invalid inputs
	assert.Equal(t, uint64(0), parseTTLToSeconds(""))
	assert.Equal(t, uint64(0), parseTTLToSeconds("invalid ttl expression"))
	assert.Equal(t, uint64(0), parseTTLToSeconds("INTERVAL 30 INVALID"))
	assert.Equal(t, uint64(0), parseTTLToSeconds("toIntervalInvalid(30)"))
	assert.Equal(t, uint64(0), parseTTLToSeconds("INTERVAL abc DAY"))
	assert.Equal(t, uint64(0), parseTTLToSeconds("toIntervalDay(abc)"))
}

func TestConvertIntervalToSeconds(t *testing.T) {
	// Seconds
	assert.Equal(t, uint64(30), convertIntervalToSeconds(30, "SECOND"))
	assert.Equal(t, uint64(30), convertIntervalToSeconds(30, "SECONDS"))

	// Minutes
	assert.Equal(t, uint64(300), convertIntervalToSeconds(5, "MINUTE"))
	assert.Equal(t, uint64(300), convertIntervalToSeconds(5, "MINUTES"))

	// Hours
	assert.Equal(t, uint64(7200), convertIntervalToSeconds(2, "HOUR"))
	assert.Equal(t, uint64(7200), convertIntervalToSeconds(2, "HOURS"))

	// Days
	assert.Equal(t, uint64(259200), convertIntervalToSeconds(3, "DAY"))
	assert.Equal(t, uint64(259200), convertIntervalToSeconds(3, "DAYS"))

	// Weeks
	assert.Equal(t, uint64(604800), convertIntervalToSeconds(1, "WEEK"))
	assert.Equal(t, uint64(604800), convertIntervalToSeconds(1, "WEEKS"))

	// Months
	assert.Equal(t, uint64(5184000), convertIntervalToSeconds(2, "MONTH"))
	assert.Equal(t, uint64(5184000), convertIntervalToSeconds(2, "MONTHS"))

	// Quarters
	assert.Equal(t, uint64(7776000), convertIntervalToSeconds(1, "QUARTER"))
	assert.Equal(t, uint64(7776000), convertIntervalToSeconds(1, "QUARTERS"))

	// Years
	assert.Equal(t, uint64(31536000), convertIntervalToSeconds(1, "YEAR"))
	assert.Equal(t, uint64(31536000), convertIntervalToSeconds(1, "YEARS"))

	// Edge cases
	assert.Equal(t, uint64(0), convertIntervalToSeconds(10, "INVALID"))
	assert.Equal(t, uint64(0), convertIntervalToSeconds(0, "DAY"))
}
