// File: game/collision_tracker_test.go
package game

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollisionTracker_BeginCollision(t *testing.T) {
	tracker := NewCollisionTracker()
	key1 := CollisionKey{Object1ID: 1, Object2ID: 10}
	key2 := CollisionKey{Object1ID: 2, Object2ID: 20}

	// First time registration should return true
	assert.True(t, tracker.BeginCollision(key1), "First BeginCollision for key1 should return true")
	assert.True(t, tracker.IsColliding(key1), "key1 should be colliding after BeginCollision")

	// Second time registration for the same key should return false
	assert.False(t, tracker.BeginCollision(key1), "Second BeginCollision for key1 should return false")
	assert.True(t, tracker.IsColliding(key1), "key1 should still be colliding")

	// Registration for a different key should return true
	assert.True(t, tracker.BeginCollision(key2), "BeginCollision for key2 should return true")
	assert.True(t, tracker.IsColliding(key2), "key2 should be colliding")
}

func TestCollisionTracker_EndCollision(t *testing.T) {
	tracker := NewCollisionTracker()
	key1 := CollisionKey{Object1ID: 1, Object2ID: 10}

	// Register collision
	tracker.BeginCollision(key1)
	assert.True(t, tracker.IsColliding(key1), "key1 should be colliding initially")

	// End collision
	tracker.EndCollision(key1)
	assert.False(t, tracker.IsColliding(key1), "key1 should not be colliding after EndCollision")

	// Ending a non-existent collision should not cause issues
	tracker.EndCollision(key1)
	assert.False(t, tracker.IsColliding(key1), "key1 should still not be colliding after ending again")

	key2 := CollisionKey{Object1ID: 2, Object2ID: 20}
	tracker.EndCollision(key2) // End collision that was never started
	assert.False(t, tracker.IsColliding(key2), "key2 should not be colliding")
}

func TestCollisionTracker_IsColliding(t *testing.T) {
	tracker := NewCollisionTracker()
	key1 := CollisionKey{Object1ID: 1, Object2ID: 10}
	key2 := CollisionKey{Object1ID: 2, Object2ID: 20}

	assert.False(t, tracker.IsColliding(key1), "key1 should not be colliding initially")
	assert.False(t, tracker.IsColliding(key2), "key2 should not be colliding initially")

	tracker.BeginCollision(key1)
	assert.True(t, tracker.IsColliding(key1), "key1 should be colliding after BeginCollision")
	assert.False(t, tracker.IsColliding(key2), "key2 should still not be colliding")

	tracker.EndCollision(key1)
	assert.False(t, tracker.IsColliding(key1), "key1 should not be colliding after EndCollision")
}

func TestCollisionTracker_GetActiveCollisionsForKey1(t *testing.T) {
	tracker := NewCollisionTracker()
	key1_10 := CollisionKey{Object1ID: 1, Object2ID: 10}
	key1_11 := CollisionKey{Object1ID: 1, Object2ID: 11}
	key2_10 := CollisionKey{Object1ID: 2, Object2ID: 10}
	key2_20 := CollisionKey{Object1ID: 2, Object2ID: 20}

	tracker.BeginCollision(key1_10)
	tracker.BeginCollision(key1_11)
	tracker.BeginCollision(key2_10)
	tracker.BeginCollision(key2_20)

	collisionsFor1 := tracker.GetActiveCollisionsForKey1(1)
	assert.Len(t, collisionsFor1, 2, "Should find 2 collisions for Object1ID=1")
	assert.Contains(t, collisionsFor1, key1_10)
	assert.Contains(t, collisionsFor1, key1_11)

	collisionsFor2 := tracker.GetActiveCollisionsForKey1(2)
	assert.Len(t, collisionsFor2, 2, "Should find 2 collisions for Object1ID=2")
	assert.Contains(t, collisionsFor2, key2_10)
	assert.Contains(t, collisionsFor2, key2_20)

	collisionsFor3 := tracker.GetActiveCollisionsForKey1(3)
	assert.Len(t, collisionsFor3, 0, "Should find 0 collisions for Object1ID=3")

	// End one collision and check again
	tracker.EndCollision(key1_10)
	collisionsFor1 = tracker.GetActiveCollisionsForKey1(1)
	assert.Len(t, collisionsFor1, 1, "Should find 1 collision for Object1ID=1 after ending one")
	assert.Contains(t, collisionsFor1, key1_11)
}

func TestCollisionTracker_ClearAll(t *testing.T) {
	tracker := NewCollisionTracker()
	key1 := CollisionKey{Object1ID: 1, Object2ID: 10}
	key2 := CollisionKey{Object1ID: 2, Object2ID: 20}

	tracker.BeginCollision(key1)
	tracker.BeginCollision(key2)
	assert.True(t, tracker.IsColliding(key1))
	assert.True(t, tracker.IsColliding(key2))

	tracker.ClearAll()
	assert.False(t, tracker.IsColliding(key1))
	assert.False(t, tracker.IsColliding(key2))
	assert.Empty(t, tracker.activeCollisions)
}

// Test with example brick IDs (using the helper from game_actor_physics.go)
func TestCollisionTracker_WithBrickIDs(t *testing.T) {
	tracker := NewCollisionTracker()
	gridSize := 10
	ballID := 100

	brickID1 := makeBrickID(3, 4, gridSize) // Uses makeBrickID from game_actor_physics.go
	brickID2 := makeBrickID(5, 6, gridSize) // Uses makeBrickID from game_actor_physics.go

	key1 := CollisionKey{Object1ID: ballID, Object2ID: brickID1}
	key2 := CollisionKey{Object1ID: ballID, Object2ID: brickID2}

	assert.True(t, tracker.BeginCollision(key1))
	assert.False(t, tracker.BeginCollision(key1)) // Already active
	assert.True(t, tracker.BeginCollision(key2))

	assert.True(t, tracker.IsColliding(key1))
	assert.True(t, tracker.IsColliding(key2))

	active := tracker.GetActiveCollisionsForKey1(ballID)
	assert.Len(t, active, 2)
	assert.Contains(t, active, key1)
	assert.Contains(t, active, key2)

	tracker.EndCollision(key1)
	assert.False(t, tracker.IsColliding(key1))
	assert.True(t, tracker.IsColliding(key2))

	active = tracker.GetActiveCollisionsForKey1(ballID)
	assert.Len(t, active, 1)
	assert.Contains(t, active, key2)
}