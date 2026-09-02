package pipeline

import "sync"

// Progress provides lossless publishing with mutex+chan(1) semantics.
// Publish never drops when used with a concurrent consumer; Complete closes
// exactly once; NextMessage drains events in order.
type Progress struct {
	mu     sync.Mutex
	ch     ProgressChan
	closed bool
}

// NewProgress creates a Progress with the given buffer. If buffer <1, 1 is used.
// Caller typically uses cap 32 for burst tolerance (>=16 per spec).
func NewProgress(buffer int) *Progress {
	if buffer < 1 {
		buffer = 1
	}
	return &Progress{ch: make(ProgressChan, buffer)}
}

// Chan returns the underlying channel for Apply and consumers.
func (p *Progress) Chan() ProgressChan { return p.ch }

// Publish sends ev without dropping. Returns false if already completed.
// It is safe for concurrent callers; send is recover-protected against
// close race and never panics.
func (p *Progress) Publish(ev ProgressEvent) bool {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return false
	}
	ch := p.ch
	p.mu.Unlock()
	defer func() { _ = recover() }()
	select {
	case ch <- ev:
		return true
	default:
		// buffer full: blocking send preserves lossless guarantee when
		// consumer is concurrent; fallback to blocking if needed.
		ch <- ev
		return true
	}
}

// Complete closes the channel exactly once.
func (p *Progress) Complete() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.closed {
		close(p.ch)
		p.closed = true
	}
}

// NextMessage receives the next event. Second result false means channel
// closed and drained.
func (p *Progress) NextMessage() (ProgressEvent, bool) {
	ev, ok := <-p.ch
	return ev, ok
}

// SafeClose closes ch exactly once, recovering from double-close panic.
// Useful when both StagePlan.Apply and Orchestrator defer close.
func SafeClose(ch ProgressChan) {
	defer func() { _ = recover() }()
	close(ch)
}
