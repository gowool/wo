package middleware

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestItem_MessagePackOperations(t *testing.T) {
	t.Parallel()

	t.Run("marshal and unmarshal empty item", func(t *testing.T) {
		original := &item{}

		// Marshal
		data, err := original.MarshalMsg(nil)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		// Unmarshal
		unmarshaled := &item{}
		remaining, err := unmarshaled.UnmarshalMsg(data)
		require.NoError(t, err)
		require.Empty(t, remaining)

		// Verify values
		require.Equal(t, original.currHits, unmarshaled.currHits)
		require.Equal(t, original.prevHits, unmarshaled.prevHits)
		require.Equal(t, original.exp, unmarshaled.exp)
	})

	t.Run("marshal and unmarshal item with data", func(t *testing.T) {
		original := &item{
			currHits: 5,
			prevHits: 3,
			exp:      1234567890,
		}

		// Marshal
		data, err := original.MarshalMsg(nil)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		// Unmarshal
		unmarshaled := &item{}
		remaining, err := unmarshaled.UnmarshalMsg(data)
		require.NoError(t, err)
		require.Empty(t, remaining)

		// Verify values
		require.Equal(t, original.currHits, unmarshaled.currHits)
		require.Equal(t, original.prevHits, unmarshaled.prevHits)
		require.Equal(t, original.exp, unmarshaled.exp)
	})

	t.Run("marshal into existing buffer", func(t *testing.T) {
		original := &item{
			currHits: 10,
			prevHits: 7,
			exp:      9876543210,
		}

		// Marshal into existing buffer
		existingBuffer := make([]byte, 0, 100)
		data, err := original.MarshalMsg(existingBuffer)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		// Unmarshal
		unmarshaled := &item{}
		remaining, err := unmarshaled.UnmarshalMsg(data)
		require.NoError(t, err)
		require.Empty(t, remaining)

		// Verify values
		require.Equal(t, original.currHits, unmarshaled.currHits)
		require.Equal(t, original.prevHits, unmarshaled.prevHits)
		require.Equal(t, original.exp, unmarshaled.exp)
	})

	t.Run("msgsize returns reasonable size", func(t *testing.T) {
		testItem := &item{
			currHits: 100,
			prevHits: 50,
			exp:      1234567890,
		}

		size := testItem.Msgsize()
		require.True(t, size > 0)
		require.True(t, size < 100) // Should be reasonably small
	})
}
