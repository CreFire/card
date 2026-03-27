package etcd

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	rawClient *clientv3.Client
)

// TestMain sets up the etcd client for all tests in this package.
func TestMain(m *testing.M) {
	// NOTE: This test requires an etcd server running on localhost:2379.
	endpoints := []string{"localhost:2379"}
	var err error

	// We need a logger for the watcher code.

	// Use clientv3.New directly to avoid dependency issues with xlog.
	rawClient, err = clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to create etcd client for testing: %v", err)
	}

	// Run tests
	code := m.Run()

	// Teardown
	rawClient.Close()
	os.Exit(code)
}

// cleanupKeys deletes all keys with a given prefix.
func cleanupKeys(t *testing.T, prefix string) {
	_, err := rawClient.Delete(context.Background(), prefix, clientv3.WithPrefix())
	require.NoError(t, err, "Failed to clean up keys with prefix %s", prefix)
}

func TestMultiWatcher_Lifecycle(t *testing.T) {

	// Define prefixes for two different services
	prefix1 := "/test/multiwatch/service1/"
	prefix2 := "/test/multiwatch/service2/"
	keysToWatch := []string{prefix1, prefix2}

	// Ensure a clean state before the test
	cleanupKeys(t, "/test/multiwatch/")
	defer cleanupKeys(t, "/test/multiwatch/")

	// --- 1. Setup initial data ---
	initialData := map[string]string{
		prefix1 + "node1": "value1",
		prefix1 + "node2": "value2",
		prefix2 + "nodeA": "valueA",
	}
	for k, v := range initialData {
		_, err := rawClient.Put(context.Background(), k, v)
		require.NoError(t, err)
	}

	// --- 2. Create MultiWatcher ---
	ctx, cancel := context.WithCancel(context.Background())

	multiWatcher, err := NewMultiWatcher(rawClient, keysToWatch, ctx, clientv3.WithPrefix())
	require.NoError(t, err)
	require.NotNil(t, multiWatcher)

	eventChan := multiWatcher.EventChan()

	// --- 3. Consume events in a separate goroutine ---
	go func() {

		// --- 3.1. Verify SNAPSHOT events ---
		// The first response should be the snapshot of all existing keys.
		for snapshotResp := range eventChan {
			// require.NoError(t, snapshotResp.Err)
			// assert.Len(t, snapshotResp.Events, 3, "Should receive 3 snapshot events")

			// receivedSnapshots := make(map[string]string)
			// for _, event := range snapshotResp.Events {
			// 	assert.Equal(t, EventTypeSnapshot, event.Type)
			// 	receivedSnapshots[event.Key] = string(event.Value)
			// }
			// assert.Equal(t, initialData, receivedSnapshots, "Snapshot data does not match initial data")

			log.Printf("recv event %+v", snapshotResp.Events)
			for _, event := range snapshotResp.Events {
				log.Printf("Event Type: %s, Key: %s, Value: %s", event.Type, event.Key, string(event.Value))
			}
		}

		// // --- 3.2. Verify PUT event ---
		// select {
		// case putResp := <-eventChan:
		// 	require.NoError(t, putResp.Err)
		// 	assert.Len(t, putResp.Events, 1, "Should receive 1 PUT event")
		// 	putEvent := putResp.Events[0]
		// 	assert.Equal(t, EventTypePut, putEvent.Type)
		// 	assert.Equal(t, prefix1+"node3", putEvent.Key)
		// 	assert.Equal(t, "value3", string(putEvent.Value))
		// 	log.Println("PUT event received and verified.")
		// case <-time.After(2 * time.Second):
		// 	t.Error("timed out waiting for PUT event")
		// 	return
		// }

		// // --- 3.3. Verify DELETE event ---
		// select {
		// case deleteResp := <-eventChan:
		// 	require.NoError(t, deleteResp.Err)
		// 	assert.Len(t, deleteResp.Events, 1, "Should receive 1 DELETE event")
		// 	deleteEvent := deleteResp.Events[0]
		// 	assert.Equal(t, EventTypeDelete, deleteEvent.Type)
		// 	assert.Equal(t, prefix2+"nodeA", deleteEvent.Key)
		// 	assert.Nil(t, deleteEvent.Value, "Value for DELETE event should be nil")
		// 	log.Println("DELETE event received and verified.")
		// case <-time.After(2 * time.Second):
		// 	t.Error("timed out waiting for DELETE event")
		// 	return
		// }
	}()

	// --- 4. Trigger live events ---
	time.Sleep(200 * time.Millisecond) // Give watcher time to get snapshot
	log.Println("Putting new key to trigger PUT event...")
	_, err = rawClient.Put(context.Background(), prefix1+"node3", "value3")
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
	log.Println("Deleting key to trigger DELETE event...")
	_, err = rawClient.Delete(context.Background(), prefix2+"nodeA")
	require.NoError(t, err)

	// --- 5. Close the watcher and wait for goroutine to finish ---
	time.Sleep(10000 * time.Millisecond) // Allow time for events to be processed
	log.Println("Closing watcher...")
	multiWatcher.Close()

	log.Println("Test finished.")
	cancel()
}

func TestNewMultiWatcher_EdgeCases(t *testing.T) {
	t.Run("NilClient", func(t *testing.T) {
		_, err := NewMultiWatcher(nil, []string{"/key"}, context.Background())
		require.Error(t, err)
		assert.Equal(t, "etcd client is nil", err.Error())
	})

	t.Run("NoKeys", func(t *testing.T) {
		_, err := NewMultiWatcher(rawClient, []string{}, context.Background())
		require.Error(t, err)
		assert.Equal(t, "keys cannot be empty", err.Error())
	})
}

func TestMultiWatcher_ContextCancellation(t *testing.T) {
	prefix1 := "/test/cancel/service1/"
	keysToWatch := []string{prefix1}

	cleanupKeys(t, "/test/cancel/")
	defer cleanupKeys(t, "/test/cancel/")

	// Create a cancellable context
	pCtx, cancel := context.WithCancel(context.Background())

	multiWatcher, err := NewMultiWatcher(rawClient, keysToWatch, pCtx)
	require.NoError(t, err)
	require.NotNil(t, multiWatcher)

	eventChan := multiWatcher.EventChan()
	done := make(chan struct{})

	// Start a consumer goroutine
	go func() {
		for range eventChan {
			// We might receive a snapshot before cancellation, which is fine.
		}
		// When the channel is closed, this loop will exit.
		close(done)
	}()

	// Cancel the context after a short delay
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Wait for the consumer goroutine to finish, which indicates the channel was closed.
	select {
	case <-done:
		// This is the expected outcome.
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for event channel to close after context cancellation")
	}

	// Another way to check is to try to receive from the closed watcher.
	// This is redundant if the above passes, but good for illustration.
	multiWatcher.Close() // Calling Close on an already-cancelled watcher should be safe.
}
