// File: game/collision_tracker.go
package game

import "sync"

// CollisionKey represents a unique collision pair.
// Object1ID is typically the "active" object (e.g., Ball).
// Object2ID is the "passive" object (e.g., Brick ID, Paddle Index).
type CollisionKey struct {
	Object1ID int
	Object2ID int
}

// CollisionTracker manages active collision states using a map.
// It ensures that an action associated with a collision start
// is triggered only once until the collision ends and restarts.
type CollisionTracker struct {
	mu               sync.RWMutex
	activeCollisions map[CollisionKey]bool // Stores keys of currently active collisions
}

// NewCollisionTracker creates a new, empty collision tracker.
func NewCollisionTracker() *CollisionTracker {
	return &CollisionTracker{
		activeCollisions: make(map[CollisionKey]bool),
	}
}

// BeginCollision attempts to register the start of a collision for the given key.
// It returns true if this is a *new* collision (the key was not previously active),
// indicating that the associated "on collision begin" action should occur.
// It returns false if the collision was already active (ongoing).
// This operation is thread-safe.
func (ct *CollisionTracker) BeginCollision(key CollisionKey) bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	if _, exists := ct.activeCollisions[key]; exists {
		// Collision is already active, do not signal a new beginning.
		return false
	}

	// This is a new collision. Register it and signal true.
	ct.activeCollisions[key] = true
	return true
}

// EndCollision removes a collision registration for the given key.
// This should be called when the two objects associated with the key
// are confirmed to be no longer colliding.
// This operation is thread-safe.
func (ct *CollisionTracker) EndCollision(key CollisionKey) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	delete(ct.activeCollisions, key)
}

// IsColliding checks if a specific collision key is currently registered as active.
// This operation is thread-safe.
func (ct *CollisionTracker) IsColliding(key CollisionKey) bool {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	_, exists := ct.activeCollisions[key]
	return exists
}

// GetActiveCollisionsForKey1 returns a slice of all active collision keys
// where the Object1ID matches the provided id.
// Useful for checking which collisions involving a specific object (like a ball)
// need to be re-evaluated for separation.
// This operation is thread-safe.
func (ct *CollisionTracker) GetActiveCollisionsForKey1(object1ID int) []CollisionKey {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	keys := make([]CollisionKey, 0)
	for key := range ct.activeCollisions {
		if key.Object1ID == object1ID {
			keys = append(keys, key)
		}
	}
	return keys
}

// ClearAll removes all currently tracked collisions.
// Useful for resetting state, e.g., at the start of a test or potentially on game reset.
// This operation is thread-safe.
func (ct *CollisionTracker) ClearAll() {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.activeCollisions = make(map[CollisionKey]bool)
}