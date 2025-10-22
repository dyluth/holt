package blackboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

// Client provides instance-scoped Redis operations for the blackboard.
// All keys and channels are automatically namespaced with the instance name.
// The client is thread-safe and can be used concurrently from multiple goroutines.
type Client struct {
	rdb          *redis.Client
	instanceName string
}

// NewClient creates a new blackboard client for the specified instance.
// The client automatically namespaces all keys and channels with the instance name.
//
// Parameters:
//   - redisOpts: Redis connection options (address, password, DB, etc.)
//   - instanceName: Holt instance identifier (must not be empty)
//
// Returns an error if instanceName is empty.
func NewClient(redisOpts *redis.Options, instanceName string) (*Client, error) {
	if instanceName == "" {
		return nil, fmt.Errorf("instance name cannot be empty")
	}

	return &Client{
		rdb:          redis.NewClient(redisOpts),
		instanceName: instanceName,
	}, nil
}

// Close closes the Redis connection. Implements io.Closer.
// After calling Close(), the client should not be used.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Ping verifies Redis connectivity. Useful for health checks.
// Returns an error if Redis is not reachable.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// RedisClient returns the underlying Redis client for advanced operations.
// This is primarily for testing purposes. Use the Client methods when possible.
func (c *Client) RedisClient() *redis.Client {
	return c.rdb
}

// CreateArtefact writes an artefact to Redis and publishes an event.
// Validates the artefact before writing. Returns error if validation fails or Redis operation fails.
// Publishes full artefact JSON to holt:{instance}:artefact_events after successful write.
//
// The artefact is stored as a Redis hash at holt:{instance}:artefact:{id}.
// This method is idempotent - writing the same artefact twice is safe.
func (c *Client) CreateArtefact(ctx context.Context, a *Artefact) error {
	// Validate artefact
	if err := a.Validate(); err != nil {
		return fmt.Errorf("invalid artefact: %w", err)
	}

	// Convert to Redis hash
	hash, err := ArtefactToHash(a)
	if err != nil {
		return fmt.Errorf("failed to serialize artefact: %w", err)
	}

	// Write to Redis
	key := ArtefactKey(c.instanceName, a.ID)
	if err := c.rdb.HSet(ctx, key, hash).Err(); err != nil {
		return fmt.Errorf("failed to write artefact to Redis: %w", err)
	}

	// Publish event
	artefactJSON, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("failed to marshal artefact for event: %w", err)
	}

	channel := ArtefactEventsChannel(c.instanceName)
	if err := c.rdb.Publish(ctx, channel, artefactJSON).Err(); err != nil {
		return fmt.Errorf("failed to publish artefact event: %w", err)
	}

	return nil
}

// GetArtefact retrieves an artefact by ID.
// Returns (nil, redis.Nil) if the artefact doesn't exist.
// Use IsNotFound() to check for not-found errors.
func (c *Client) GetArtefact(ctx context.Context, artefactID string) (*Artefact, error) {
	key := ArtefactKey(c.instanceName, artefactID)

	// Read hash from Redis
	hashData, err := c.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to read artefact from Redis: %w", err)
	}

	// Check if key exists (HGetAll returns empty map for non-existent keys)
	if len(hashData) == 0 {
		return nil, redis.Nil
	}

	// Convert to Artefact
	artefact, err := HashToArtefact(hashData)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize artefact: %w", err)
	}

	return artefact, nil
}

// ArtefactExists checks if an artefact exists without fetching it.
// More efficient than GetArtefact when you only need to check existence.
func (c *Client) ArtefactExists(ctx context.Context, artefactID string) (bool, error) {
	key := ArtefactKey(c.instanceName, artefactID)
	exists, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check artefact existence: %w", err)
	}
	return exists > 0, nil
}

// CreateClaim writes a claim to Redis and publishes an event.
// Validates the claim before writing.
// Publishes full claim JSON to holt:{instance}:claim_events after successful write.
// Also creates an index mapping artefact_id to claim_id for idempotency checks.
func (c *Client) CreateClaim(ctx context.Context, claim *Claim) error {
	// Validate claim
	if err := claim.Validate(); err != nil {
		return fmt.Errorf("invalid claim: %w", err)
	}

	// Convert to Redis hash
	hash, err := ClaimToHash(claim)
	if err != nil {
		return fmt.Errorf("failed to serialize claim: %w", err)
	}

	// Write to Redis
	key := ClaimKey(c.instanceName, claim.ID)
	if err := c.rdb.HSet(ctx, key, hash).Err(); err != nil {
		return fmt.Errorf("failed to write claim to Redis: %w", err)
	}

	// Create artefact -> claim index for idempotency checks
	indexKey := ClaimByArtefactKey(c.instanceName, claim.ArtefactID)
	if err := c.rdb.Set(ctx, indexKey, claim.ID, 0).Err(); err != nil {
		return fmt.Errorf("failed to create claim index: %w", err)
	}

	// Publish event
	claimJSON, err := json.Marshal(claim)
	if err != nil {
		return fmt.Errorf("failed to marshal claim for event: %w", err)
	}

	channel := ClaimEventsChannel(c.instanceName)
	if err := c.rdb.Publish(ctx, channel, claimJSON).Err(); err != nil {
		return fmt.Errorf("failed to publish claim event: %w", err)
	}

	return nil
}

// GetClaim retrieves a claim by ID.
// Returns (nil, redis.Nil) if the claim doesn't exist.
func (c *Client) GetClaim(ctx context.Context, claimID string) (*Claim, error) {
	key := ClaimKey(c.instanceName, claimID)

	// Read hash from Redis
	hashData, err := c.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to read claim from Redis: %w", err)
	}

	// Check if key exists
	if len(hashData) == 0 {
		return nil, redis.Nil
	}

	// Convert to Claim
	claim, err := HashToClaim(hashData)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize claim: %w", err)
	}

	return claim, nil
}

// UpdateClaim replaces an existing claim with new data (full HMSET replacement).
// Used by orchestrator to update status and granted agents as claim progresses through phases.
// Validates the claim before writing.
//
// Note: This performs a full replacement of all fields. The claim will be created if it doesn't exist.
func (c *Client) UpdateClaim(ctx context.Context, claim *Claim) error {
	// Validate claim
	if err := claim.Validate(); err != nil {
		return fmt.Errorf("invalid claim: %w", err)
	}

	// Convert to Redis hash
	hash, err := ClaimToHash(claim)
	if err != nil {
		return fmt.Errorf("failed to serialize claim: %w", err)
	}

	// Write to Redis (full replacement)
	key := ClaimKey(c.instanceName, claim.ID)
	if err := c.rdb.HSet(ctx, key, hash).Err(); err != nil {
		return fmt.Errorf("failed to update claim in Redis: %w", err)
	}

	return nil
}

// ClaimExists checks if a claim exists without fetching it.
func (c *Client) ClaimExists(ctx context.Context, claimID string) (bool, error) {
	key := ClaimKey(c.instanceName, claimID)
	exists, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check claim existence: %w", err)
	}
	return exists > 0, nil
}

// GetClaimByArtefactID retrieves a claim by its associated artefact ID.
// Returns (nil, redis.Nil) if no claim exists for the given artefact.
// Used for idempotency checking - ensures only one claim per artefact.
func (c *Client) GetClaimByArtefactID(ctx context.Context, artefactID string) (*Claim, error) {
	// Look up claim ID from index
	indexKey := ClaimByArtefactKey(c.instanceName, artefactID)
	claimID, err := c.rdb.Get(ctx, indexKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, redis.Nil
		}
		return nil, fmt.Errorf("failed to lookup claim by artefact: %w", err)
	}

	// Retrieve the claim
	return c.GetClaim(ctx, claimID)
}

// SetBid records an agent's bid on a claim and publishes a bid_submitted event.
// Uses HSET on holt:{instance}:claim:{claim_id}:bids with key=agentName, value=bidType.
// Validates the bid type before writing.
// Publishes bid_submitted event to workflow_events channel after successful write.
func (c *Client) SetBid(ctx context.Context, claimID string, agentName string, bidType BidType) error {
	// Validate bid type
	if err := bidType.Validate(); err != nil {
		return fmt.Errorf("invalid bid type: %w", err)
	}

	// Write bid to Redis
	key := ClaimBidsKey(c.instanceName, claimID)
	if err := c.rdb.HSet(ctx, key, agentName, string(bidType)).Err(); err != nil {
		return fmt.Errorf("failed to write bid to Redis: %w", err)
	}

	// Publish bid_submitted event
	eventData := map[string]interface{}{
		"claim_id":   claimID,
		"agent_name": agentName,
		"bid_type":   string(bidType),
	}
	if err := c.publishWorkflowEvent(ctx, "bid_submitted", eventData); err != nil {
		return fmt.Errorf("failed to publish bid_submitted event: %w", err)
	}

	return nil
}

// GetAllBids retrieves all bids for a claim as a map of agent name to bid type.
// Returns empty map if no bids exist (not an error).
func (c *Client) GetAllBids(ctx context.Context, claimID string) (map[string]BidType, error) {
	key := ClaimBidsKey(c.instanceName, claimID)

	// Read all bids from Redis
	rawBids, err := c.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to read bids from Redis: %w", err)
	}

	// Convert string values to BidType
	bids := make(map[string]BidType, len(rawBids))
	for agentName, bidTypeStr := range rawBids {
		bids[agentName] = BidType(bidTypeStr)
	}

	return bids, nil
}

// AddVersionToThread adds an artefact to a version thread.
// Uses ZADD with score=version to maintain sorted order.
// Threads are stored as ZSETs at holt:{instance}:thread:{logical_id}.
func (c *Client) AddVersionToThread(ctx context.Context, logicalID string, artefactID string, version int) error {
	key := ThreadKey(c.instanceName, logicalID)
	score := ThreadScore(version)

	z := redis.Z{
		Score:  score,
		Member: artefactID,
	}

	if err := c.rdb.ZAdd(ctx, key, z).Err(); err != nil {
		return fmt.Errorf("failed to add version to thread: %w", err)
	}

	return nil
}

// GetLatestVersion retrieves the artefact ID of the highest version in a thread.
// Returns ("", 0, redis.Nil) if the thread doesn't exist or is empty.
func (c *Client) GetLatestVersion(ctx context.Context, logicalID string) (artefactID string, version int, err error) {
	key := ThreadKey(c.instanceName, logicalID)

	// Get the member with the highest score (ZREVRANGE with limit 1)
	results, err := c.rdb.ZRevRangeWithScores(ctx, key, 0, 0).Result()
	if err != nil {
		return "", 0, fmt.Errorf("failed to get latest version from thread: %w", err)
	}

	// Check if thread is empty
	if len(results) == 0 {
		return "", 0, redis.Nil
	}

	// Extract artefact ID and version
	artefactID = results[0].Member.(string)
	version = VersionFromScore(results[0].Score)

	return artefactID, version, nil
}

// Subscription represents an active Pub/Sub subscription to artefact events.
// Caller must call Close() when done to clean up resources.
// Subscriptions deliver full artefact objects via the Events() channel.
type Subscription struct {
	events <-chan *Artefact
	errors <-chan error
	cancel func()
	once   sync.Once
}

// Events returns the channel of artefact events.
// The channel will be closed when the subscription is closed or the context is cancelled.
func (s *Subscription) Events() <-chan *Artefact {
	return s.events
}

// Errors returns the channel of subscription errors.
// Errors include JSON unmarshaling failures and other non-fatal issues.
// The subscription continues after errors - messages are skipped.
func (s *Subscription) Errors() <-chan error {
	return s.errors
}

// Close stops the subscription and cleans up resources. Implements io.Closer.
// Safe to call multiple times - subsequent calls are no-ops.
func (s *Subscription) Close() error {
	s.once.Do(s.cancel)
	return nil
}

// ClaimSubscription represents an active Pub/Sub subscription to claim events.
// Caller must call Close() when done to clean up resources.
type ClaimSubscription struct {
	events <-chan *Claim
	errors <-chan error
	cancel func()
	once   sync.Once
}

// WorkflowEvent represents a workflow event (bid submission or claim grant).
// These events are published for real-time monitoring via the watch command.
type WorkflowEvent struct {
	Event string                 `json:"event"` // "bid_submitted" or "claim_granted"
	Data  map[string]interface{} `json:"data"`  // Event-specific data
}

// WorkflowSubscription represents an active Pub/Sub subscription to workflow events.
// Caller must call Close() when done to clean up resources.
type WorkflowSubscription struct {
	events <-chan *WorkflowEvent
	errors <-chan error
	cancel func()
	once   sync.Once
}

// Events returns the channel of claim events.
func (s *ClaimSubscription) Events() <-chan *Claim {
	return s.events
}

// Errors returns the channel of subscription errors.
func (s *ClaimSubscription) Errors() <-chan error {
	return s.errors
}

// Close stops the subscription and cleans up resources. Implements io.Closer.
func (s *ClaimSubscription) Close() error {
	s.once.Do(s.cancel)
	return nil
}

// Events returns the channel of workflow events.
func (s *WorkflowSubscription) Events() <-chan *WorkflowEvent {
	return s.events
}

// Errors returns the channel of subscription errors.
func (s *WorkflowSubscription) Errors() <-chan error {
	return s.errors
}

// Close stops the subscription and cleans up resources. Implements io.Closer.
func (s *WorkflowSubscription) Close() error {
	s.once.Do(s.cancel)
	return nil
}

// SubscribeArtefactEvents subscribes to artefact creation events for this instance.
// Returns a Subscription that delivers full artefact objects.
// Caller must call subscription.Close() when done.
// Context cancellation also stops the subscription.
//
// Events are delivered on a buffered channel (size 10) to prevent blocking.
// If the subscriber is too slow, events may be dropped by Redis Pub/Sub (at-most-once delivery).
func (c *Client) SubscribeArtefactEvents(ctx context.Context) (*Subscription, error) {
	channel := ArtefactEventsChannel(c.instanceName)
	pubsub := c.rdb.Subscribe(ctx, channel)

	// Create buffered channels for events and errors
	eventsChan := make(chan *Artefact, 10)
	errorsChan := make(chan error, 10)

	// Create cancellation context
	subCtx, cancelFunc := context.WithCancel(ctx)

	// Start goroutine to process messages
	go func() {
		defer close(eventsChan)
		defer close(errorsChan)
		defer pubsub.Close()

		// Receive channel from pubsub
		ch := pubsub.Channel()

		for {
			select {
			case <-subCtx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}

				// Unmarshal artefact from JSON
				var artefact Artefact
				if err := json.Unmarshal([]byte(msg.Payload), &artefact); err != nil {
					// Send error on error channel, skip message
					select {
					case errorsChan <- fmt.Errorf("failed to unmarshal artefact event: %w", err):
					case <-subCtx.Done():
						return
					}
					continue
				}

				// Send artefact on events channel
				select {
				case eventsChan <- &artefact:
				case <-subCtx.Done():
					return
				}
			}
		}
	}()

	return &Subscription{
		events: eventsChan,
		errors: errorsChan,
		cancel: cancelFunc,
	}, nil
}

// SubscribeClaimEvents subscribes to claim creation events for this instance.
// Returns a ClaimSubscription that delivers full claim objects.
// Caller must call subscription.Close() when done.
func (c *Client) SubscribeClaimEvents(ctx context.Context) (*ClaimSubscription, error) {
	channel := ClaimEventsChannel(c.instanceName)
	pubsub := c.rdb.Subscribe(ctx, channel)

	// Create buffered channels for events and errors
	eventsChan := make(chan *Claim, 10)
	errorsChan := make(chan error, 10)

	// Create cancellation context
	subCtx, cancelFunc := context.WithCancel(ctx)

	// Start goroutine to process messages
	go func() {
		defer close(eventsChan)
		defer close(errorsChan)
		defer pubsub.Close()

		// Receive channel from pubsub
		ch := pubsub.Channel()

		for {
			select {
			case <-subCtx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}

				// Unmarshal claim from JSON
				var claim Claim
				if err := json.Unmarshal([]byte(msg.Payload), &claim); err != nil {
					// Send error on error channel, skip message
					select {
					case errorsChan <- fmt.Errorf("failed to unmarshal claim event: %w", err):
					case <-subCtx.Done():
						return
					}
					continue
				}

				// Send claim on events channel
				select {
				case eventsChan <- &claim:
				case <-subCtx.Done():
					return
				}
			}
		}
	}()

	return &ClaimSubscription{
		events: eventsChan,
		errors: errorsChan,
		cancel: cancelFunc,
	}, nil
}

// SubscribeWorkflowEvents subscribes to workflow events (bid submissions and grants) for this instance.
// Returns a WorkflowSubscription that delivers workflow event objects.
// Caller must call subscription.Close() when done.
func (c *Client) SubscribeWorkflowEvents(ctx context.Context) (*WorkflowSubscription, error) {
	channel := WorkflowEventsChannel(c.instanceName)
	pubsub := c.rdb.Subscribe(ctx, channel)

	// Create buffered channels for events and errors
	eventsChan := make(chan *WorkflowEvent, 10)
	errorsChan := make(chan error, 10)

	// Create cancellation context
	subCtx, cancelFunc := context.WithCancel(ctx)

	// Start goroutine to process messages
	go func() {
		defer close(eventsChan)
		defer close(errorsChan)
		defer pubsub.Close()

		// Receive channel from pubsub
		ch := pubsub.Channel()

		for {
			select {
			case <-subCtx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}

				// Unmarshal workflow event from JSON
				var event WorkflowEvent
				if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
					// Send error on error channel, skip message
					select {
					case errorsChan <- fmt.Errorf("failed to unmarshal workflow event: %w", err):
					case <-subCtx.Done():
						return
					}
					continue
				}

				// Send event on events channel
				select {
				case eventsChan <- &event:
				case <-subCtx.Done():
					return
				}
			}
		}
	}()

	return &WorkflowSubscription{
		events: eventsChan,
		errors: errorsChan,
		cancel: cancelFunc,
	}, nil
}

// RawSubscription represents an active Pub/Sub subscription to a raw channel.
// Used for subscribing to custom channels like agent-specific event channels.
// Caller must call Close() when done to clean up resources.
type RawSubscription struct {
	messages <-chan string
	cancel   func()
	once     sync.Once
}

// Messages returns the channel of raw message payloads.
func (s *RawSubscription) Messages() <-chan string {
	return s.messages
}

// Close stops the subscription and cleans up resources. Implements io.Closer.
func (s *RawSubscription) Close() error {
	s.once.Do(s.cancel)
	return nil
}

// SubscribeRawChannel subscribes to a raw Pub/Sub channel for this instance.
// Returns a RawSubscription that delivers message payloads as strings.
// Caller must call subscription.Close() when done.
//
// This is used for subscribing to custom channels like agent-specific event channels
// where the message format is known but not typed (e.g., grant notifications).
func (c *Client) SubscribeRawChannel(ctx context.Context, channel string) (*RawSubscription, error) {
	pubsub := c.rdb.Subscribe(ctx, channel)

	// Create buffered channel for messages
	messagesChan := make(chan string, 10)

	// Create cancellation context
	subCtx, cancelFunc := context.WithCancel(ctx)

	// Start goroutine to process messages
	go func() {
		defer close(messagesChan)
		defer pubsub.Close()

		// Receive channel from pubsub
		ch := pubsub.Channel()

		for {
			select {
			case <-subCtx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}

				// Send raw payload on messages channel
				select {
				case messagesChan <- msg.Payload:
				case <-subCtx.Done():
					return
				}
			}
		}
	}()

	return &RawSubscription{
		messages: messagesChan,
		cancel:   cancelFunc,
	}, nil
}

// PublishRaw publishes a raw message to a specified Redis Pub/Sub channel.
// This is used for publishing custom messages like grant notifications to agent-specific channels.
// The channel name should be a full channel name (not auto-prefixed with instance).
func (c *Client) PublishRaw(ctx context.Context, channel string, message string) error {
	if err := c.rdb.Publish(ctx, channel, message).Err(); err != nil {
		return fmt.Errorf("failed to publish to channel %s: %w", channel, err)
	}
	return nil
}

// publishWorkflowEvent publishes a workflow event to the workflow_events channel.
// This is an internal helper used by SetBid and orchestrator for real-time monitoring.
// Event types: "bid_submitted", "claim_granted"
func (c *Client) publishWorkflowEvent(ctx context.Context, eventType string, data map[string]interface{}) error {
	event := WorkflowEvent{
		Event: eventType,
		Data:  data,
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow event: %w", err)
	}

	channel := WorkflowEventsChannel(c.instanceName)
	if err := c.rdb.Publish(ctx, channel, eventJSON).Err(); err != nil {
		return fmt.Errorf("failed to publish workflow event: %w", err)
	}

	return nil
}

// PublishWorkflowEvent publishes a workflow event to the workflow_events channel.
// This is exposed for orchestrator use when publishing claim_granted events.
// Event types: "bid_submitted", "claim_granted"
func (c *Client) PublishWorkflowEvent(ctx context.Context, eventType string, data map[string]interface{}) error {
	return c.publishWorkflowEvent(ctx, eventType, data)
}

// GetClaimsByStatus retrieves all claims with the specified statuses (M3.5).
// Used for orchestrator startup recovery to scan Redis for active claims.
// Returns empty slice if no claims match the specified statuses.
//
// Implementation: Uses Redis SCAN to iterate over claim keys, then filters by status.
// This is efficient for moderate claim counts (<10000) but may need optimization for larger datasets.
func (c *Client) GetClaimsByStatus(ctx context.Context, statuses []string) ([]*Claim, error) {
	if len(statuses) == 0 {
		return []*Claim{}, nil
	}

	// Build status set for O(1) lookup
	statusSet := make(map[ClaimStatus]bool)
	for _, status := range statuses {
		statusSet[ClaimStatus(status)] = true
	}

	// Scan for all claim keys using pattern matching
	pattern := ClaimKey(c.instanceName, "*")
	var claims []*Claim

	// Use SCAN to iterate over keys matching the pattern
	iter := c.rdb.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()

		// Fetch claim from Redis
		hashData, err := c.rdb.HGetAll(ctx, key).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to read claim from Redis: %w", err)
		}

		// Skip if key no longer exists (race condition)
		if len(hashData) == 0 {
			continue
		}

		// Deserialize claim
		claim, err := HashToClaim(hashData)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize claim: %w", err)
		}

		// Filter by status
		if statusSet[claim.Status] {
			claims = append(claims, claim)
		}
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan claim keys: %w", err)
	}

	return claims, nil
}

// ZAdd adds a member to a sorted set with a score (M3.5 - for grant queue FIFO).
// Used to add claims to the persistent grant queue when max_concurrent limit is reached.
func (c *Client) ZAdd(ctx context.Context, key string, score float64, member string) error {
	z := redis.Z{
		Score:  score,
		Member: member,
	}

	if err := c.rdb.ZAdd(ctx, key, z).Err(); err != nil {
		return fmt.Errorf("failed to add member to sorted set: %w", err)
	}

	return nil
}

// ZRange retrieves members from a sorted set by rank range (M3.5 - for grant queue dequeue).
// Returns members in order from lowest to highest score (FIFO for timestamp-based scores).
// start and stop are inclusive (0-based indexing, -1 for last element).
func (c *Client) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	members, err := c.rdb.ZRange(ctx, key, start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to read sorted set range: %w", err)
	}

	return members, nil
}

// ZRangeWithScores retrieves members with scores from a sorted set (M3.5 - for grant queue recovery).
// Used during startup to recover grant queue state with timestamps.
func (c *Client) ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	results, err := c.rdb.ZRangeWithScores(ctx, key, start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to read sorted set with scores: %w", err)
	}

	return results, nil
}

// ZRem removes members from a sorted set (M3.5 - for grant queue dequeue).
// Used to remove claims from grant queue after they are resumed.
func (c *Client) ZRem(ctx context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}

	// Convert string slice to interface slice for variadic function
	memberInterfaces := make([]interface{}, len(members))
	for i, member := range members {
		memberInterfaces[i] = member
	}

	if err := c.rdb.ZRem(ctx, key, memberInterfaces...).Err(); err != nil {
		return fmt.Errorf("failed to remove members from sorted set: %w", err)
	}

	return nil
}

// IsNotFound returns true if the error is a Redis "key not found" error (redis.Nil).
// Use this to check if GetArtefact, GetClaim, or GetLatestVersion returned "not found".
func IsNotFound(err error) bool {
	return errors.Is(err, redis.Nil)
}
