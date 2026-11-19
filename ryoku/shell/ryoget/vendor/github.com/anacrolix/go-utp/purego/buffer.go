package purego

// A circular buffer indexed directly by (wrapping) sequence number. Slots are pointers so that an
// absent element is distinguishable from a zero one, and the buffer grows by powers of two as the
// distance between the oldest and newest live index demands.
type circularBuffer[T any] struct {
	mask  uint32
	elems []*T
}

const initialCircularBufferSize = 16

func (b *circularBuffer[T]) init() {
	b.mask = initialCircularBufferSize - 1
	b.elems = make([]*T, initialCircularBufferSize)
}

func (b *circularBuffer[T]) get(i uint32) *T {
	if b.elems == nil {
		return nil
	}
	return b.elems[i&b.mask]
}

func (b *circularBuffer[T]) put(i uint32, v *T) {
	b.elems[i&b.mask] = v
}

func (b *circularBuffer[T]) size() uint32 {
	return b.mask + 1
}

// Makes room for item, which sits index slots from the oldest live element.
func (b *circularBuffer[T]) ensureSize(item, index uint32) {
	if index > b.mask {
		b.grow(item, index)
	}
}

func (b *circularBuffer[T]) grow(item, index uint32) {
	size := b.mask + 1
	for {
		size *= 2
		if index < size {
			break
		}
	}
	buf := make([]*T, size)
	newMask := size - 1
	// Copy across by sequence number rather than by slot, so everything keeps landing on the
	// index it will be looked up by.
	for i := uint32(0); i <= b.mask; i++ {
		src := item - index + i
		buf[src&newMask] = b.elems[src&b.mask]
	}
	b.elems = buf
	b.mask = newMask
}
