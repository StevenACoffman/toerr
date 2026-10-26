// Package errclass attaches a coarse severity classification (Transient,
// Persistent, Panic) to an error for retry decisions inside a component. The
// classification survives wrapping, and GetClass folds over errors.Join by returning
// the highest class among the joined errors.
//
// GetClass finds a class anywhere in the chain, so a class set deep inside a
// dependency still answers at the top. Where an error crosses into another domain,
// judge the class there: reclassify with WrapAs, or seal the cause with errors.Seal,
// which GetClass cannot see past. Across a service or package boundary, prefer the
// errcode status, whose codes say whether a retry can help.
package errclass
