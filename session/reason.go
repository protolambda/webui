package session

import "strconv"

// EndReason says why a session ended, for audit logs and for telling the
// user (signed out, expired, signed in elsewhere). The zero value means
// that the session has not ended.
type EndReason uint8

// The reasons a session ends.
const (
	// EndLogout: the application ended the session itself, with
	// [Store.End] or [Session.End].
	EndLogout EndReason = iota + 1
	// EndRevoked: [Store.EndAll] or [Store.EndFunc] ended the session.
	EndRevoked
	// EndIdle: the session went unused for [Options.IdleTTL].
	EndIdle
	// EndAbsolute: [Options.AbsoluteTTL] passed since its creation.
	EndAbsolute
	// EndRotated: [Store.New] replaced the session with a new one for the
	// same browser, or [Store.Rotate] moved its value to a new session.
	EndRotated
	// EndEvicted: the store was full and the session was the least
	// recently used when [Store.New] needed room. Many evictions in a
	// short time mean that the store is too small for its load, or that
	// someone logs in again and again to push other sessions out.
	EndEvicted
)

// String returns the lowercase name of the reason ("logout", "revoked",
// "idle", "absolute", "rotated", "evicted"), or "EndReason(n)" for other
// values.
func (r EndReason) String() string {
	switch r {
	case EndLogout:
		return "logout"
	case EndRevoked:
		return "revoked"
	case EndIdle:
		return "idle"
	case EndAbsolute:
		return "absolute"
	case EndRotated:
		return "rotated"
	case EndEvicted:
		return "evicted"
	}
	return "EndReason(" + strconv.Itoa(int(r)) + ")"
}
