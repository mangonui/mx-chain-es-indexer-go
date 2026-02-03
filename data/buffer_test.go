package data

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBufferSlice_PutDataShouldWork(t *testing.T) {
	buffSlice := NewBufferSlice(DefaultMaxBulkSize)

	meta := generateRandomBytes(100)
	serializedData := generateRandomBytes(100)

	err := buffSlice.PutData(meta, serializedData)
	require.Nil(t, err)

	serializedData = generateRandomBytes(DefaultMaxBulkSize)
	err = buffSlice.PutData(meta, serializedData)
	require.Nil(t, err)

	returnedBuffSlice := buffSlice.Buffers()
	require.Equal(t, 2, len(returnedBuffSlice))
}

func TestBufferSlice_PutDataShouldWorkNilSerializedData(t *testing.T) {
	buffSlice := NewBufferSlice(DefaultMaxBulkSize)

	meta := []byte("my data")

	err := buffSlice.PutData(meta, nil)
	require.Nil(t, err)

	returnedBuffSlice := buffSlice.Buffers()
	require.Equal(t, 1, len(returnedBuffSlice))
}

func TestBufferSlice_PutDataShouldWorkNilSerializedDataSize1(t *testing.T) {
	buffSlice := NewBufferSlice(1)

	meta := []byte("my data")

	err := buffSlice.PutData(meta, []byte("serialized"))
	require.Nil(t, err)

	returnedBuffSlice := buffSlice.Buffers()
	require.Equal(t, 1, len(returnedBuffSlice))
	require.Equal(t, "my dataserialized\n", returnedBuffSlice[0].String())
}

func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)

	return b
}

func TestBufferSlice_Merge(t *testing.T) {
	t.Run("Merge nil or empty buffer should do nothing", func(t *testing.T) {
		bs := NewBufferSlice(100)
		require.NoError(t, bs.PutData([]byte("meta"), []byte("data")))

		bs.Merge(nil)
		require.Len(t, bs.Buffers(), 1)
		require.Equal(t, "metadata\n", bs.Buffers()[0].String())

		other := NewBufferSlice(100)
		bs.Merge(other)
		require.Len(t, bs.Buffers(), 1)
		require.Equal(t, "metadata\n", bs.Buffers()[0].String())
	})

	t.Run("Merge fit in current buffer", func(t *testing.T) {
		bs := NewBufferSlice(100)
		require.NoError(t, bs.PutData([]byte("m1"), []byte("d1"))) // 6 bytes

		other := NewBufferSlice(100)
		require.NoError(t, other.PutData([]byte("m2"), []byte("d2"))) // 6 bytes

		bs.Merge(other)
		require.Len(t, bs.Buffers(), 1)
		expected := "m1d1\nm2d2\n"
		require.Equal(t, expected, bs.Buffers()[0].String())
	})

	t.Run("Merge exceeds threshold, new buffer created", func(t *testing.T) {
		bs := NewBufferSlice(9)                                    // Small threshold
		require.NoError(t, bs.PutData([]byte("m1"), []byte("d1"))) // 5 bytes

		other := NewBufferSlice(9)
		require.NoError(t, other.PutData([]byte("m2"), []byte("d2"))) // 5 bytes

		// Merging 5 bytes into existing 5 bytes with threshold 9 -> should create new buffer
		bs.Merge(other)

		require.Len(t, bs.Buffers(), 2)
		require.Equal(t, "m1d1\n", bs.Buffers()[0].String())
		require.Equal(t, "m2d2\n", bs.Buffers()[1].String())
	})

	t.Run("Merge ignores empty buffers in source", func(t *testing.T) {
		bs := NewBufferSlice(100)
		require.NoError(t, bs.PutData([]byte("m1"), []byte("d1")))

		other := NewBufferSlice(100)
		// Manually append empty buffer to simulate edge case if possible via internal manipulation or just empty initialized
		other.buffSlice = append(other.buffSlice, &bytes.Buffer{})
		require.NoError(t, other.PutData([]byte("m2"), []byte("d2")))

		bs.Merge(other)
		require.Len(t, bs.Buffers(), 1)
		require.Contains(t, bs.Buffers()[0].String(), "m1d1")
		require.Contains(t, bs.Buffers()[0].String(), "m2d2")
	})

	t.Run("Complex Merge", func(t *testing.T) {
		bs := NewBufferSlice(10)
		_ = bs.PutData([]byte("12"), []byte("34")) // "1234\n" -> 5 bytes

		other := NewBufferSlice(10)
		_ = other.PutData([]byte("ab"), []byte("cd")) // "abcd\n" -> 5 bytes
		_ = other.PutData([]byte("fg"), []byte("hi")) // "fghi\n" -> 5 bytes.

		bs.Merge(other)

		require.Len(t, bs.Buffers(), 2)
		require.Equal(t, "1234\n", bs.Buffers()[0].String())
		require.Equal(t, "abcd\nfghi\n", bs.Buffers()[1].String())
	})
}
