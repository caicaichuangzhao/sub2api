package repository

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMarshalTimePricingNilProducesValidJSONNull(t *testing.T) {
	raw, err := marshalTimePricing(nil)
	require.NoError(t, err)
	require.Equal(t, []byte("null"), raw)

	var value any
	require.NoError(t, json.Unmarshal(raw, &value))
	require.Nil(t, value)
}

func TestMarshalTimePricingConfigProducesValidJSON(t *testing.T) {
	price := 0.000001
	raw, err := marshalTimePricing(&service.TimePricingConfig{
		Timezone: "Asia/Shanghai",
		Periods: []service.TimePricingPeriod{{
			Name:        "peak",
			StartTime:   "09:00",
			EndTime:     "18:00",
			InputPrice:  &price,
			Weekdays:    []int{1, 2, 3, 4, 5},
		}},
	})
	require.NoError(t, err)

	var value map[string]any
	require.NoError(t, json.Unmarshal(raw, &value))
	require.Equal(t, "Asia/Shanghai", value["timezone"])
}
