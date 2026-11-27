// Package threeway merges two versions of a list keyed by id against
// their common base: the merge git cannot do on the text of a state file
// that every change rewrites.
package threeway

import "fmt"

// Conflict is an entry the two sides disagree about: a real conflict,
// which a person settles.
type Conflict struct {
	Key    string
	Reason string
}

func (c *Conflict) Error() string {
	return fmt.Sprintf("%s %s", c.Key, c.Reason)
}

// Merge returns ours with theirs' changes since base applied. An entry
// changed, added or removed on one side takes that side; one both sides
// changed to the same content is kept once. The same entry changed on
// both sides, added on both with different content, or changed on one
// and removed on the other is a *Conflict.
//
// The result keeps ours' order. An entry only theirs added is appended,
// or with anchored set placed after the entry it followed in theirs.
func Merge[T any](base, ours, theirs []T, key func(*T) string, equal func(a, b *T) bool, anchored bool) ([]T, error) {
	b, o, t := index(base, key), index(ours, key), index(theirs, key)
	out := make([]T, 0, len(ours)+len(theirs))
	for i := range ours {
		k := key(&ours[i])
		bv, inBase := b[k]
		tv, inTheirs := t[k]
		switch {
		case inTheirs && equal(&ours[i], tv):
			out = append(out, ours[i])
		case inTheirs && inBase && equal(bv, &ours[i]):
			out = append(out, *tv)
		case inTheirs && inBase && equal(bv, tv):
			out = append(out, ours[i])
		case inTheirs && inBase:
			return nil, &Conflict{k, "changed on both sides"}
		case inTheirs:
			return nil, &Conflict{k, "added on both sides with different content"}
		case !inBase:
			out = append(out, ours[i])
		case equal(bv, &ours[i]):
			// Removed by theirs.
		default:
			return nil, &Conflict{k, "changed on one side and removed on the other"}
		}
	}
	prev := ""
	for i := range theirs {
		k := key(&theirs[i])
		if _, inOurs := o[k]; inOurs {
			prev = k
			continue
		}
		if bv, inBase := b[k]; inBase {
			// Removed by ours.
			if !equal(bv, &theirs[i]) {
				return nil, &Conflict{k, "changed on one side and removed on the other"}
			}
			continue
		}
		at := len(out)
		if anchored {
			at = 0
			for j := range out {
				if key(&out[j]) == prev {
					at = j + 1
				}
			}
		}
		out = append(out[:at], append([]T{theirs[i]}, out[at:]...)...)
		prev = k
	}
	return out, nil
}

func index[T any](list []T, key func(*T) string) map[string]*T {
	m := make(map[string]*T, len(list))
	for i := range list {
		m[key(&list[i])] = &list[i]
	}
	return m
}
