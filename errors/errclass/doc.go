// Package errclass attaches a coarse severity classification (Transient,
// Persistent, Panic) to an error for retry decisions. The classification survives
// wrapping, and GetClass folds over errors.Join by returning the highest class
// among the joined errors.
package errclass
