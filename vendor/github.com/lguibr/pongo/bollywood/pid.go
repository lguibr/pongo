
package bollywood

// PID (Process ID) represents a unique reference to an actor instance.
type PID struct {
	ID string
	// We could add address/node info here for distributed actors later
}

// String returns the string representation of the PID.
func (pid *PID) String() string {
	return pid.ID
}